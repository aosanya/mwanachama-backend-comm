// dm_message_impl.go — GORM-backed DMRepository implementation, message +
// device-key + reaction half. See dm_impl.go's package doc for the
// three-way file split this port needs (dm_impl.go: thread reads;
// dm_roster_impl.go: roster writes; this file: messages).
package mwanachamacomm

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

// Post stores a ciphertext DM message. created_at is always the store's own
// clock, matching ChatStore.Post — mirrors the memory store's original
// unconditional stamp (the old Postgres store's COALESCE let a caller
// supply one; this port standardises on the memory store's contract, the
// one every business-rule test already assumes).
func (s *DMStore) Post(ctx context.Context, m models.DMMessage) (models.DMMessage, error) {
	m.CreatedAt = s.clock()
	row, err := gormstore.DMMessageToRow(m)
	if err != nil {
		return models.DMMessage{}, err
	}
	if err := s.db.WithContext(ctx).Table(s.tables.DMMessages).Create(&row).Error; err != nil {
		return models.DMMessage{}, classify(err)
	}
	return gormstore.DMMessageFromRow(row), nil
}

// ListMessages returns every message in a thread that is still within its
// deadline, oldest first.
//
// **The filter is what makes a disappearing message exact** (DEV-1541). The
// TTL is read off the thread, where it was frozen when the thread was
// opened. Applied in Go against s.clock() rather than in SQL (the original
// Postgres store used `make_interval`, which sqlite has no equivalent for
// — this store also runs against sqlite in tests) — same filter, same
// result, portable across both dialects.
func (s *DMStore) ListMessages(ctx context.Context, threadID string) ([]models.DMMessage, error) {
	var thread gormstore.DMThreadRow
	err := s.db.WithContext(ctx).Table(s.tables.DMThreads).Where("id = ?", threadID).First(&thread).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var rows []gormstore.DMMessageRow
	if err := s.db.WithContext(ctx).Table(s.tables.DMMessages).
		Where("thread_id = ?", threadID).Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	now := s.clock()
	out := make([]models.DMMessage, 0, len(rows))
	for _, r := range rows {
		if thread.MessageTTLSeconds != nil {
			deadline := r.CreatedAt.Add(time.Duration(*thread.MessageTTLSeconds) * time.Second)
			if !deadline.After(now) {
				continue
			}
		}
		out = append(out, gormstore.DMMessageFromRow(r))
	}
	return out, nil
}

// GetMessage returns one message by id, so a caller acting on a message can
// find the thread whose participant fence governs it.
func (s *DMStore) GetMessage(ctx context.Context, id string) (models.DMMessage, error) {
	var row gormstore.DMMessageRow
	err := s.db.WithContext(ctx).Table(s.tables.DMMessages).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.DMMessage{}, models.ErrDMNotFound
	}
	if err != nil {
		return models.DMMessage{}, err
	}
	return gormstore.DMMessageFromRow(row), nil
}

func (s *DMStore) PublishDeviceKey(ctx context.Context, k models.DMDeviceKey) (models.DMDeviceKey, error) {
	if k.CreatedAt.IsZero() {
		k.CreatedAt = s.clock()
	}
	// Re-publishing the same key id un-retires it, matching the original
	// upsert's `retired_at = NULL`.
	k.RetiredAt = nil
	row := gormstore.DMDeviceKeyToRow(k)
	err := s.db.WithContext(ctx).Table(s.tables.DMDeviceKeys).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "member_id"}, {Name: "key_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"public_key", "published_by", "device_id", "retired_at"}),
		}).Create(&row).Error
	if err != nil {
		return models.DMDeviceKey{}, classify(err)
	}
	// Re-fetched rather than trusting `row`: on the conflict path GORM does
	// not repopulate created_at from the pre-existing row, and that field
	// must survive a re-publish untouched.
	var out gormstore.DMDeviceKeyRow
	if err := s.db.WithContext(ctx).Table(s.tables.DMDeviceKeys).
		Where("member_id = ? AND key_id = ?", row.MemberID, row.KeyID).First(&out).Error; err != nil {
		return models.DMDeviceKey{}, err
	}
	published := gormstore.DMDeviceKeyFromRow(out)

	if published.DeviceID != "" {
		err := s.db.WithContext(ctx).Table(s.tables.DMDeviceKeys).
			Where("member_id = ? AND device_id = ? AND key_id <> ? AND retired_at IS NULL",
				published.ActorID, published.DeviceID, published.KeyID).
			Updates(map[string]any{"retired_at": s.clock()}).Error
		if err != nil {
			return models.DMDeviceKey{}, classify(err)
		}
	}
	return published, nil
}

func (s *DMStore) LookupDeviceKeys(ctx context.Context, memberIDs []string) ([]models.DMDeviceKey, error) {
	if len(memberIDs) == 0 {
		return []models.DMDeviceKey{}, nil
	}
	var rows []gormstore.DMDeviceKeyRow
	err := s.db.WithContext(ctx).Table(s.tables.DMDeviceKeys).
		Where("member_id IN ? AND retired_at IS NULL", memberIDs).
		Order("member_id, key_id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]models.DMDeviceKey, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.DMDeviceKeyFromRow(r))
	}
	return out, nil
}

// RetireDeviceKeysForDevice retires every live key one handset published
// (DEV-1272) and reports how many it retired. An empty deviceID retires
// nothing and is not an error — a session minted by the phone flow carries
// no device.
func (s *DMStore) RetireDeviceKeysForDevice(ctx context.Context, deviceID string, at time.Time) (int, error) {
	if deviceID == "" {
		return 0, nil
	}
	if at.IsZero() {
		at = s.clock()
	}
	res := s.db.WithContext(ctx).Table(s.tables.DMDeviceKeys).
		Where("device_id = ? AND retired_at IS NULL", deviceID).
		Updates(map[string]any{"retired_at": at})
	if res.Error != nil {
		return 0, classify(res.Error)
	}
	return int(res.RowsAffected), nil
}

func (s *DMStore) SetReaction(ctx context.Context, r models.DMReaction) error {
	if _, err := s.GetMessage(ctx, r.MessageID); err != nil {
		return err
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = s.clock()
	}
	row := gormstore.DMReactionToRow(r)
	err := s.db.WithContext(ctx).Table(s.tables.DMReactions).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "message_id"}, {Name: "member_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"emoji", "created_at"}),
		}).Create(&row).Error
	return classify(err)
}

func (s *DMStore) ClearReaction(ctx context.Context, messageID, memberID string) error {
	err := s.db.WithContext(ctx).Table(s.tables.DMReactions).
		Where("message_id = ? AND member_id = ?", messageID, memberID).
		Delete(&gormstore.DMReactionRow{}).Error
	return classify(err)
}

// ListReactions returns every reaction on every message of a thread.
func (s *DMStore) ListReactions(ctx context.Context, threadID string) ([]models.DMReaction, error) {
	var rows []gormstore.DMReactionRow
	err := s.db.WithContext(ctx).Table(s.tables.DMReactions+" AS r").
		Select("r.*").
		Joins("JOIN "+s.tables.DMMessages+" AS m ON m.id = r.message_id").
		Where("m.thread_id = ?", threadID).
		Order("r.message_id, r.member_id").
		Find(&rows).Error
	if err != nil {
		return nil, classify(err)
	}
	out := make([]models.DMReaction, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.DMReactionFromRow(r))
	}
	return out, nil
}
