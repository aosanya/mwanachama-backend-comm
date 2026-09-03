package mwanachamacomm

import (
	"context"
	"database/sql"
	"errors"
)

// DMPostgresStore is the Postgres implementation of DMRepository. It is
// split across two files: this one covers thread + roster ops; message +
// device-key + reaction ops live in store_postgres_dm_messages.go.
//
// Tables: comm_dm_thread, comm_dm_participant, comm_dm_message,
// comm_dm_device_key, comm_dm_message_reaction.
type DMPostgresStore struct {
	db *sql.DB
}

// NewDMPostgresStore constructs a store over the given pool.
func NewDMPostgresStore(db *sql.DB) *DMPostgresStore { return &DMPostgresStore{db: db} }

// CreateThread inserts a thread and seeds the initial roster (creator = active
// admin; other initial members = invited). Runs in one transaction so a
// participant-insert failure rolls back the empty thread.
func (s *DMPostgresStore) CreateThread(ctx context.Context, t DMThread, initial []string) (DMThread, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DMThread{}, err
	}
	defer func() { _ = tx.Rollback() }()

	const q = `INSERT INTO comm_dm_thread (id, title, created_by, created_at, opened_via_address_hash, opened_via_address_owner, opened_via_address_index,
	                       sent_from_address_owner, sent_from_address_index, sent_from_address_sealed,
	                       message_ttl_seconds)
	           VALUES (COALESCE(NULLIF($1,''), 'dm-' || nextval('comm_dm_thread_seq')),
	                   NULLIF($2,''), $3, COALESCE($4, now()), $5, NULLIF($6,''), $7, NULLIF($8,''), $9, $10, $11)
	           RETURNING id, COALESCE(title,''), created_by, created_at,
	                     opened_via_address_hash, COALESCE(opened_via_address_owner,''),
	                     opened_via_address_index,
	                     COALESCE(sent_from_address_owner,''), sent_from_address_index,
	                     sent_from_address_sealed, message_ttl_seconds`
	var out DMThread
	var ttl sql.NullInt64
	err = tx.QueryRowContext(ctx, q, t.ID, t.Title, t.CreatedBy, nullTime(t.CreatedAt), t.OpenedViaAddressHash, t.OpenedViaAddressOwner, t.OpenedViaAddressIndex,
		t.SentFromAddressOwner, t.SentFromAddressIndex, nullJSON(t.SentFromAddressSealed),
		nullInt(t.MessageTTLSeconds)).
		Scan(&out.ID, &out.Title, &out.CreatedBy, &out.CreatedAt, &out.OpenedViaAddressHash, &out.OpenedViaAddressOwner,
			&out.OpenedViaAddressIndex, &out.SentFromAddressOwner,
			&out.SentFromAddressIndex, nullableJSON{&out.SentFromAddressSealed}, &ttl)
	if err != nil {
		return DMThread{}, classify(err)
	}
	out.MessageTTLSeconds = intPtr(ttl)

	const pIns = `INSERT INTO comm_dm_participant (thread_id, member_id, state, is_admin, updated_at)
	              VALUES ($1, $2, $3, $4, now())`
	if out.CreatedBy != "" {
		if _, err := tx.ExecContext(ctx, pIns, out.ID, out.CreatedBy, string(DMStateActive), true); err != nil {
			return DMThread{}, classify(err)
		}
	}
	for _, m := range initial {
		if m == out.CreatedBy {
			continue
		}
		if _, err := tx.ExecContext(ctx, pIns, out.ID, m, string(DMStateInvited), false); err != nil {
			return DMThread{}, classify(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return DMThread{}, err
	}
	return out, nil
}

// GetThread returns a thread by id.
func (s *DMPostgresStore) GetThread(ctx context.Context, id string) (DMThread, error) {
	const q = `SELECT id, COALESCE(title,''), created_by, created_at,
	                  opened_via_address_hash, COALESCE(opened_via_address_owner,''),
	                  opened_via_address_index,
	                  COALESCE(sent_from_address_owner,''), sent_from_address_index,
	                  sent_from_address_sealed, message_ttl_seconds
	             FROM comm_dm_thread WHERE id = $1`
	var t DMThread
	var ttl sql.NullInt64
	err := s.db.QueryRowContext(ctx, q, id).
		Scan(&t.ID, &t.Title, &t.CreatedBy, &t.CreatedAt, &t.OpenedViaAddressHash,
			&t.OpenedViaAddressOwner, &t.OpenedViaAddressIndex,
			&t.SentFromAddressOwner, &t.SentFromAddressIndex,
			nullableJSON{&t.SentFromAddressSealed}, &ttl)
	if errors.Is(err, sql.ErrNoRows) {
		return DMThread{}, ErrDMNotFound
	}
	if err != nil {
		return DMThread{}, err
	}
	t.MessageTTLSeconds = intPtr(ttl)
	return t, nil
}

// ListThreadsFor returns every thread the member is currently invited to or
// active in.
func (s *DMPostgresStore) ListThreadsFor(ctx context.Context, memberID string) ([]DMThread, error) {
	const q = `SELECT t.id, COALESCE(t.title,''), t.created_by, t.created_at,
	                  t.opened_via_address_hash, COALESCE(t.opened_via_address_owner,''),
	                  t.opened_via_address_index,
	                  COALESCE(t.sent_from_address_owner,''), t.sent_from_address_index,
	                  t.sent_from_address_sealed, t.message_ttl_seconds
	           FROM comm_dm_thread t
	           JOIN comm_dm_participant p ON p.thread_id = t.id
	           WHERE p.member_id = $1 AND p.state IN ($2, $3)
	           ORDER BY t.created_at, t.id`
	rows, err := s.db.QueryContext(ctx, q, memberID, string(DMStateActive), string(DMStateInvited))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DMThread{}
	for rows.Next() {
		var t DMThread
		var ttl sql.NullInt64
		if err := rows.Scan(&t.ID, &t.Title, &t.CreatedBy, &t.CreatedAt,
			&t.OpenedViaAddressHash, &t.OpenedViaAddressOwner,
			&t.OpenedViaAddressIndex, &t.SentFromAddressOwner,
			&t.SentFromAddressIndex, nullableJSON{&t.SentFromAddressSealed}, &ttl); err != nil {
			return nil, err
		}
		t.MessageTTLSeconds = intPtr(ttl)
		out = append(out, t)
	}
	return out, rows.Err()
}

var errNotAdmin = errors.New("directmessage: not admin")

func (s *DMPostgresStore) isAdmin(ctx context.Context, threadID, memberID string) (bool, error) {
	const q = `SELECT is_admin, state FROM comm_dm_participant WHERE thread_id = $1 AND member_id = $2`
	var (
		isAdmin bool
		state   string
	)
	err := s.db.QueryRowContext(ctx, q, threadID, memberID).Scan(&isAdmin, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return isAdmin && state == string(DMStateActive), nil
}

// Invite adds an invited member (admin-only). Re-inviting somebody still
// pending flips state back to invited and stamps updated_at; re-inviting
// somebody who has already accepted is refused (ErrDMAlreadyActive) rather
// than silently taking their acceptance away.
func (s *DMPostgresStore) Invite(ctx context.Context, threadID, memberID, by string) (DMParticipant, error) {
	if ok, err := s.isAdmin(ctx, threadID, by); err != nil || !ok {
		if err != nil {
			return DMParticipant{}, err
		}
		return DMParticipant{}, errNotAdmin
	}
	state, err := s.participantState(ctx, threadID, memberID)
	if err != nil && !errors.Is(err, ErrDMNotFound) {
		return DMParticipant{}, err
	}
	if err == nil && state == DMStateActive {
		return DMParticipant{}, ErrDMAlreadyActive
	}
	return s.upsertParticipant(ctx, threadID, memberID, string(DMStateInvited), nil)
}

// Accept turns an invite into active membership.
func (s *DMPostgresStore) Accept(ctx context.Context, threadID, memberID string) (DMParticipant, error) {
	return s.setState(ctx, threadID, memberID, string(DMStateActive))
}

// Leave marks the member as having left.
func (s *DMPostgresStore) Leave(ctx context.Context, threadID, memberID string) error {
	_, err := s.setState(ctx, threadID, memberID, string(DMStateLeft))
	return err
}

// Kick marks the member as removed by an admin.
func (s *DMPostgresStore) Kick(ctx context.Context, threadID, memberID, by string) error {
	if ok, err := s.isAdmin(ctx, threadID, by); err != nil || !ok {
		if err != nil {
			return err
		}
		return errNotAdmin
	}
	_, err := s.setState(ctx, threadID, memberID, string(DMStateKicked))
	return err
}

// Promote makes the target an admin (admin-only).
func (s *DMPostgresStore) Promote(ctx context.Context, threadID, memberID, by string) (DMParticipant, error) {
	if ok, err := s.isAdmin(ctx, threadID, by); err != nil || !ok {
		if err != nil {
			return DMParticipant{}, err
		}
		return DMParticipant{}, errNotAdmin
	}
	const q = `UPDATE comm_dm_participant SET is_admin = true, updated_at = now()
	           WHERE thread_id = $1 AND member_id = $2
	           RETURNING thread_id, member_id, state, is_admin, updated_at`
	return scanParticipant(s.db.QueryRowContext(ctx, q, threadID, memberID))
}

// ReEnable is the same two acts the memory store documents: the member who was
// last to leave bringing the thread back **active** (memberID == by, nobody
// else active, their own row `left`), and an admin restoring somebody else to
// `invited`.
func (s *DMPostgresStore) ReEnable(ctx context.Context, threadID, memberID, by string) (DMParticipant, error) {
	state, err := s.participantState(ctx, threadID, memberID)
	if err != nil {
		return DMParticipant{}, err
	}
	if memberID == by {
		active, err := s.hasActive(ctx, threadID)
		if err != nil {
			return DMParticipant{}, err
		}
		if state != DMStateLeft || active {
			return DMParticipant{}, ErrDMNotLastToLeave
		}
		return s.setState(ctx, threadID, memberID, string(DMStateActive))
	}
	if ok, err := s.isAdmin(ctx, threadID, by); err != nil || !ok {
		if err != nil {
			return DMParticipant{}, err
		}
		return DMParticipant{}, errNotAdmin
	}
	return s.setState(ctx, threadID, memberID, string(DMStateInvited))
}

// participantState reads one roster row's state, or ErrDMNotFound when the
// member is not on the thread at all.
func (s *DMPostgresStore) participantState(ctx context.Context, threadID, memberID string) (DMParticipantState, error) {
	const q = `SELECT state FROM comm_dm_participant WHERE thread_id = $1 AND member_id = $2`
	var state string
	err := s.db.QueryRowContext(ctx, q, threadID, memberID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrDMNotFound
	}
	if err != nil {
		return "", err
	}
	return DMParticipantState(state), nil
}

// hasActive reports whether anybody is currently active on the thread.
func (s *DMPostgresStore) hasActive(ctx context.Context, threadID string) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM comm_dm_participant WHERE thread_id = $1 AND state = $2)`
	var out bool
	if err := s.db.QueryRowContext(ctx, q, threadID, string(DMStateActive)).Scan(&out); err != nil {
		return false, err
	}
	return out, nil
}

// ListParticipants returns every participant row for a thread.
func (s *DMPostgresStore) ListParticipants(ctx context.Context, threadID string) ([]DMParticipant, error) {
	const q = `SELECT thread_id, member_id, state, is_admin, updated_at
	           FROM comm_dm_participant WHERE thread_id = $1
	           ORDER BY updated_at, member_id`
	rows, err := s.db.QueryContext(ctx, q, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DMParticipant{}
	for rows.Next() {
		p, err := scanParticipant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// setState updates a participant's state, returning the fresh row (or
// ErrDMNotFound if there was nothing to update).
func (s *DMPostgresStore) setState(ctx context.Context, threadID, memberID, state string) (DMParticipant, error) {
	const q = `UPDATE comm_dm_participant SET state = $3, updated_at = now()
	           WHERE thread_id = $1 AND member_id = $2
	           RETURNING thread_id, member_id, state, is_admin, updated_at`
	p, err := scanParticipant(s.db.QueryRowContext(ctx, q, threadID, memberID, state))
	if errors.Is(err, sql.ErrNoRows) {
		return DMParticipant{}, ErrDMNotFound
	}
	return p, err
}

// upsertParticipant inserts (or re-invites) a participant. isAdmin is optional
// and only touched when non-nil, so a re-invite preserves the prior flag.
func (s *DMPostgresStore) upsertParticipant(ctx context.Context, threadID, memberID, state string, isAdmin *bool) (DMParticipant, error) {
	const q = `INSERT INTO comm_dm_participant (thread_id, member_id, state, is_admin, updated_at)
	           VALUES ($1, $2, $3, COALESCE($4, false), now())
	           ON CONFLICT (thread_id, member_id) DO UPDATE
	              SET state = EXCLUDED.state,
	                  is_admin = COALESCE($4, comm_dm_participant.is_admin),
	                  updated_at = now()
	           RETURNING thread_id, member_id, state, is_admin, updated_at`
	var admin sql.NullBool
	if isAdmin != nil {
		admin = sql.NullBool{Bool: *isAdmin, Valid: true}
	}
	out, err := scanParticipant(s.db.QueryRowContext(ctx, q, threadID, memberID, state, admin))
	if err != nil {
		return DMParticipant{}, classify(err)
	}
	return out, nil
}

func scanParticipant(r rowScanner) (DMParticipant, error) {
	var (
		p     DMParticipant
		state string
	)
	if err := r.Scan(&p.ThreadID, &p.MemberID, &state, &p.IsAdmin, &p.UpdatedAt); err != nil {
		return DMParticipant{}, err
	}
	p.State = DMParticipantState(state)
	return p, nil
}

var _ DMRepository = (*DMPostgresStore)(nil)

// SetReaction records one member's emoji on one message, replacing any earlier
// one from the same member.
func (s *DMPostgresStore) SetReaction(ctx context.Context, r DMReaction) error {
	const q = `INSERT INTO comm_dm_message_reaction (message_id, member_id, emoji, created_at)
	           VALUES ($1, $2, $3, COALESCE($4, now()))
	           ON CONFLICT (message_id, member_id)
	           DO UPDATE SET emoji = EXCLUDED.emoji, created_at = EXCLUDED.created_at`
	if _, err := s.db.ExecContext(ctx, q, r.MessageID, r.MemberID, r.Emoji, nullTime(r.CreatedAt)); err != nil {
		return classify(err)
	}
	return nil
}

// ClearReaction removes a member's reaction. Removing one that is not there is
// not an error — the caller asked for a state and it already holds.
func (s *DMPostgresStore) ClearReaction(ctx context.Context, messageID, memberID string) error {
	const q = `DELETE FROM comm_dm_message_reaction WHERE message_id = $1 AND member_id = $2`
	if _, err := s.db.ExecContext(ctx, q, messageID, memberID); err != nil {
		return classify(err)
	}
	return nil
}

// ListReactions returns every reaction on every message of a thread.
func (s *DMPostgresStore) ListReactions(ctx context.Context, threadID string) ([]DMReaction, error) {
	const q = `SELECT r.message_id, r.member_id, r.emoji, r.created_at
	             FROM comm_dm_message_reaction r
	             JOIN comm_dm_message m ON m.id = r.message_id
	            WHERE m.thread_id = $1
	            ORDER BY r.message_id, r.member_id`
	rows, err := s.db.QueryContext(ctx, q, threadID)
	if err != nil {
		return nil, classify(err)
	}
	defer func() { _ = rows.Close() }()
	out := []DMReaction{}
	for rows.Next() {
		var r DMReaction
		if err := rows.Scan(&r.MessageID, &r.MemberID, &r.Emoji, &r.CreatedAt); err != nil {
			return nil, classify(err)
		}
		out = append(out, r)
	}
	return out, classify(rows.Err())
}
