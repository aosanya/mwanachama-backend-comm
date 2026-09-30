package mwanachamacomm

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (s *DMStore) Post(ctx context.Context, m models.DMMessage) (models.DMMessage, error) {
	m.CreatedAt = s.clock()
	if m.ID == "" {
		m.ID = mintID(prefixDMMessage)
	}
	if err := s.store.Insert(ctx, roleDMMessage, m); err != nil {
		return models.DMMessage{}, classify(err)
	}
	return m, nil
}

func (s *DMStore) ListMessages(ctx context.Context, threadID string) ([]models.DMMessage, error) {
	thread, err := s.GetThread(ctx, threadID)
	if err != nil && !errors.Is(err, models.ErrDMNotFound) {
		return nil, err
	}

	q := s.store.Query(ctx, roleDMMessage).Where("thread_id = ?", threadID).Order("created_at, id")
	rows, err := specstore.List[models.DMMessage](s.store, q, roleDMMessage)
	if err != nil {
		return nil, err
	}

	now := s.clock()
	out := make([]models.DMMessage, 0, len(rows))
	for _, m := range rows {
		if thread.MessageTTLSeconds != nil {
			deadline := m.CreatedAt.Add(time.Duration(*thread.MessageTTLSeconds) * time.Second)
			if !deadline.After(now) {
				continue
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *DMStore) GetMessage(ctx context.Context, id string) (models.DMMessage, error) {
	var m models.DMMessage
	q := s.store.Query(ctx, roleDMMessage).Where("id = ?", id)
	if err := s.store.Take(q, roleDMMessage, &m, models.ErrDMNotFound); err != nil {
		return models.DMMessage{}, err
	}
	return m, nil
}

func (s *DMStore) PublishDeviceKey(ctx context.Context, k models.DMDeviceKey) (models.DMDeviceKey, error) {
	if k.CreatedAt.IsZero() {
		k.CreatedAt = s.clock()
	}
	if k.KeyID == "" {
		k.KeyID = mintID(prefixDMDeviceKey)
	}
	k.RetiredAt = nil

	row, err := encode(s.store.Object(roleDMDeviceKey), k)
	if err != nil {
		return models.DMDeviceKey{}, err
	}
	err = s.store.Query(ctx, roleDMDeviceKey).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "actor_id"}, {Name: "key_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"public_key", "published_by", "device_id", "retired_at"}),
		}).Create(row).Error
	if err != nil {
		return models.DMDeviceKey{}, classify(err)
	}

	var published models.DMDeviceKey
	q := s.store.Query(ctx, roleDMDeviceKey).
		Where("actor_id = ? AND key_id = ?", k.ActorID, k.KeyID)
	if err := s.store.Take(q, roleDMDeviceKey, &published, models.ErrDMNotFound); err != nil {
		return models.DMDeviceKey{}, err
	}

	if published.DeviceID != "" {
		err := s.store.Query(ctx, roleDMDeviceKey).
			Where("actor_id = ? AND device_id = ? AND key_id <> ? AND retired_at IS NULL",
				published.ActorID, published.DeviceID, published.KeyID).
			Updates(map[string]any{"retired_at": stamp(s.clock())}).Error
		if err != nil {
			return models.DMDeviceKey{}, classify(err)
		}
	}
	return published, nil
}

func (s *DMStore) LookupDeviceKeys(ctx context.Context, actorIDs []string) ([]models.DMDeviceKey, error) {
	if len(actorIDs) == 0 {
		return []models.DMDeviceKey{}, nil
	}
	q := s.store.Query(ctx, roleDMDeviceKey).
		Where("actor_id IN ? AND retired_at IS NULL", actorIDs).
		Order("actor_id, key_id")
	return specstore.List[models.DMDeviceKey](s.store, q, roleDMDeviceKey)
}

func (s *DMStore) RetireDeviceKeysForDevice(ctx context.Context, deviceID string, at time.Time) (int, error) {
	if deviceID == "" {
		return 0, nil
	}
	if at.IsZero() {
		at = s.clock()
	}
	res := s.store.Query(ctx, roleDMDeviceKey).
		Where("device_id = ? AND retired_at IS NULL", deviceID).
		Updates(map[string]any{"retired_at": stamp(at)})
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
	row, err := encode(s.store.Object(roleDMReaction), r)
	if err != nil {
		return err
	}
	return classify(s.store.Query(ctx, roleDMReaction).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "message_id"}, {Name: "actor_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"emoji", "created_at"}),
		}).Create(row).Error)
}

func (s *DMStore) ClearReaction(ctx context.Context, messageID, actorID string) error {
	return classify(deleteWhere(ctx, s.store, roleDMReaction,
		"message_id = ? and actor_id = ?", messageID, actorID))
}

func (s *DMStore) ListReactions(ctx context.Context, threadID string) ([]models.DMReaction, error) {
	reactions, messages := s.store.Table(roleDMReaction), s.store.Table(roleDMMessage)
	q := s.store.Query(ctx, roleDMReaction).
		Table(reactions+" AS r").
		Select("r.*").
		Joins("JOIN "+messages+" AS m ON m.id = r.message_id").
		Where("m.thread_id = ?", threadID).
		Order("r.message_id, r.actor_id")

	out, err := specstore.List[models.DMReaction](s.store, q, roleDMReaction)
	if err != nil {
		return nil, classify(err)
	}
	return out, nil
}
