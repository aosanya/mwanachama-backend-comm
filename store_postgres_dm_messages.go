package mwanachamacomm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Post stores a ciphertext DM message. PerRecipientKeys round-trips through
// jsonb (map[recipientMemberID]ciphertextKey), which the gateway never
// decodes — it is opaque to the server, just handed back on ListMessages.
func (s *DMPostgresStore) Post(ctx context.Context, m DMMessage) (DMMessage, error) {
	perRecipient := m.PerRecipientKeys
	if perRecipient == nil {
		perRecipient = map[string]string{}
	}
	pr, err := json.Marshal(perRecipient)
	if err != nil {
		return DMMessage{}, err
	}
	const q = `INSERT INTO comm_dm_message (id, thread_id, sender_id, sender_device_key_id,
	                                    payload_ciphertext, per_recipient_keys, created_at)
	           VALUES (COALESCE(NULLIF($1,''), 'dmmsg-' || nextval('comm_dm_message_seq')),
	                   $2, $3, $4, $5, $6::jsonb, COALESCE($7, now()))
	           RETURNING id, thread_id, sender_id, sender_device_key_id, payload_ciphertext,
	                     per_recipient_keys, created_at`
	var (
		out DMMessage
		raw []byte
	)
	err = s.db.QueryRowContext(ctx, q,
		m.ID, m.ThreadID, m.SenderID, m.SenderDeviceKeyID, m.PayloadCiphertext, pr, nullTime(m.CreatedAt),
	).Scan(&out.ID, &out.ThreadID, &out.SenderID, &out.SenderDeviceKeyID,
		&out.PayloadCiphertext, &raw, &out.CreatedAt)
	if err != nil {
		return DMMessage{}, classify(err)
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.PerRecipientKeys)
	}
	if out.PerRecipientKeys == nil {
		out.PerRecipientKeys = map[string]string{}
	}
	return out, nil
}

// ListMessages returns every message in a thread that is still within its
// deadline, oldest first.
//
// **The filter is what makes a disappearing message exact** (DEV-1541). The
// TTL is joined off comm_dm_thread, where it was frozen when the thread was
// opened — never off the address, which the owner may have changed since.
func (s *DMPostgresStore) ListMessages(ctx context.Context, threadID string) ([]DMMessage, error) {
	const q = `SELECT m.id, m.thread_id, m.sender_id, m.sender_device_key_id, m.payload_ciphertext,
	                  m.per_recipient_keys, m.created_at
	           FROM comm_dm_message m
	           JOIN comm_dm_thread t ON t.id = m.thread_id
	           WHERE m.thread_id = $1
	             AND (t.message_ttl_seconds IS NULL
	                  OR m.created_at + make_interval(secs => t.message_ttl_seconds) > now())
	           ORDER BY m.created_at, m.id`
	rows, err := s.db.QueryContext(ctx, q, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DMMessage{}
	for rows.Next() {
		var (
			m   DMMessage
			raw []byte
		)
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.SenderID, &m.SenderDeviceKeyID,
			&m.PayloadCiphertext, &raw, &m.CreatedAt); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &m.PerRecipientKeys)
		}
		if m.PerRecipientKeys == nil {
			m.PerRecipientKeys = map[string]string{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetMessage returns one message by id, so a caller acting on a message can
// find the thread whose participant fence governs it.
func (s *DMPostgresStore) GetMessage(ctx context.Context, id string) (DMMessage, error) {
	const q = `SELECT id, thread_id, sender_id, sender_device_key_id, payload_ciphertext,
	                  per_recipient_keys, created_at
	           FROM comm_dm_message WHERE id = $1`
	var (
		m   DMMessage
		raw []byte
	)
	err := s.db.QueryRowContext(ctx, q, id).Scan(&m.ID, &m.ThreadID, &m.SenderID,
		&m.SenderDeviceKeyID, &m.PayloadCiphertext, &raw, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DMMessage{}, ErrDMNotFound
	}
	if err != nil {
		return DMMessage{}, classify(err)
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m.PerRecipientKeys)
	}
	if m.PerRecipientKeys == nil {
		m.PerRecipientKeys = map[string]string{}
	}
	return m, nil
}

// PublishDeviceKey upserts a member's active public device key.
func (s *DMPostgresStore) PublishDeviceKey(ctx context.Context, k DMDeviceKey) (DMDeviceKey, error) {
	// Mirror the memory store: mint a key id when the caller left it empty.
	if k.KeyID == "" {
		// Fetched from the sequence directly so we can honour the (member, key)
		// primary key without a race on ON CONFLICT (member_id, key_id).
		row := s.db.QueryRowContext(ctx, `SELECT 'dkey-' || nextval('comm_dm_device_key_seq')`)
		if err := row.Scan(&k.KeyID); err != nil {
			return DMDeviceKey{}, err
		}
	}
	const q = `INSERT INTO comm_dm_device_key (member_id, key_id, public_key, created_at, published_by, device_id)
	           VALUES ($1, $2, $3, COALESCE($4, now()), $5, $6)
	           ON CONFLICT (member_id, key_id) DO UPDATE
	              SET public_key   = EXCLUDED.public_key,
	                  published_by = EXCLUDED.published_by,
	                  device_id    = EXCLUDED.device_id,
	                  retired_at   = NULL
	           RETURNING member_id, key_id, public_key, created_at, published_by, device_id, retired_at`
	var (
		out         DMDeviceKey
		publishedBy sql.NullString
		deviceID    sql.NullString
		retiredAt   sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, q,
		k.MemberID, k.KeyID, k.PublicKey, nullTime(k.CreatedAt),
		nullString(k.PublishedBy), nullString(k.DeviceID)).
		Scan(&out.MemberID, &out.KeyID, &out.PublicKey, &out.CreatedAt, &publishedBy, &deviceID, &retiredAt)
	if err != nil {
		return DMDeviceKey{}, classify(err)
	}
	out.PublishedBy = publishedBy.String
	out.DeviceID = deviceID.String
	if retiredAt.Valid {
		t := retiredAt.Time
		out.RetiredAt = &t
	}

	// DEV-1265 · a handset publishing a new key retires that handset's previous
	// one. Scoped to device_id, NOT to the member — a member with two handsets
	// holds two live keys at once and a sender must wrap to both.
	if out.DeviceID != "" {
		const retire = `UPDATE comm_dm_device_key SET retired_at = now()
		                WHERE member_id = $1 AND device_id = $2 AND key_id <> $3
		                  AND retired_at IS NULL`
		if _, err := s.db.ExecContext(ctx, retire, out.MemberID, out.DeviceID, out.KeyID); err != nil {
			return DMDeviceKey{}, classify(err)
		}
	}
	return out, nil
}

// LookupDeviceKeys returns every device key held for the requested members.
func (s *DMPostgresStore) LookupDeviceKeys(ctx context.Context, memberIDs []string) ([]DMDeviceKey, error) {
	if len(memberIDs) == 0 {
		return []DMDeviceKey{}, nil
	}
	// Build the IN (...) list dynamically; keeps us off driver-specific array
	// scanning while staying safe against injection (args are $1, $2, ...).
	args := make([]any, len(memberIDs))
	inClause := ""
	for i, m := range memberIDs {
		if i > 0 {
			inClause += ","
		}
		inClause += "$" + itoa(i+1)
		args[i] = m
	}
	// DEV-1265 · retired keys are never returned.
	q := `SELECT member_id, key_id, public_key, created_at, published_by, device_id
	      FROM comm_dm_device_key
	      WHERE member_id IN (` + inClause + `) AND retired_at IS NULL
	      ORDER BY member_id, key_id`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DMDeviceKey{}
	for rows.Next() {
		var (
			k                     DMDeviceKey
			publishedBy, deviceID sql.NullString
		)
		if err := rows.Scan(&k.MemberID, &k.KeyID, &k.PublicKey, &k.CreatedAt, &publishedBy, &deviceID); err != nil {
			return nil, err
		}
		k.PublishedBy = publishedBy.String
		k.DeviceID = deviceID.String
		out = append(out, k)
	}
	return out, rows.Err()
}

// RetireDeviceKeysForDevice retires every live key one handset published
// (DEV-1272) and reports how many it retired.
func (s *DMPostgresStore) RetireDeviceKeysForDevice(ctx context.Context, deviceID string, at time.Time) (int, error) {
	if deviceID == "" {
		return 0, nil
	}
	const q = `UPDATE comm_dm_device_key SET retired_at = COALESCE($2, now())
	           WHERE device_id = $1 AND retired_at IS NULL`
	res, err := s.db.ExecContext(ctx, q, deviceID, nullTime(at))
	if err != nil {
		return 0, classify(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
