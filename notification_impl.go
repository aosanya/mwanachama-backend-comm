package mwanachamacomm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type NotificationStore struct {
	store *store
	clock Clock
}

func NewNotificationStore(db *gorm.DB, s *spec.Spec, clock Clock) (*NotificationStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewNotificationStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	st, err := newStore(db, s, map[string]any{
		roleNotification:           models.Notification{},
		roleNotificationPreference: models.NotificationPreference{},
	})
	if err != nil {
		return nil, err
	}
	return &NotificationStore{store: st, clock: clock}, nil
}

func (s *NotificationStore) List(ctx context.Context, actorID string, limit int) ([]models.Notification, error) {
	if limit <= 0 {
		limit = models.NotificationDefaultPage
	}
	q := s.store.Query(ctx, roleNotification).
		Where("actor_id = ?", actorID).
		Order("created_at DESC, id DESC").
		Limit(limit)
	out, err := specstore.List[models.Notification](s.store, q, roleNotification)
	if err != nil {
		return nil, classify(err)
	}
	return out, nil
}

func (s *NotificationStore) UnreadCount(ctx context.Context, actorID string) (int, error) {
	var n int64
	err := s.store.Query(ctx, roleNotification).
		Where("actor_id = ? AND read_at IS NULL", actorID).
		Count(&n).Error
	if err != nil {
		return 0, classify(err)
	}
	return int(n), nil
}

func (s *NotificationStore) MarkRead(ctx context.Context, actorID string, ids []string, readAt time.Time) (int, error) {
	if len(ids) > models.NotificationMaxMarkRead {
		return 0, fmt.Errorf("%w: at most %d notifications in one call", models.ErrNotificationInvalid, models.NotificationMaxMarkRead)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	res := s.store.Query(ctx, roleNotification).
		Where("actor_id = ? AND id IN ? AND read_at IS NULL", actorID, ids).
		UpdateColumn("read_at", stamp(readAt))
	if res.Error != nil {
		return 0, classify(res.Error)
	}
	return int(res.RowsAffected), nil
}

func (s *NotificationStore) Raise(ctx context.Context, n models.Notification) (models.Notification, error) {
	if err := Check(s.store.Spec(), roleNotification, n); err != nil {
		return models.Notification{}, err
	}
	if err := n.Validate(); err != nil {
		return models.Notification{}, err
	}
	muted, err := s.IsMuted(ctx, n.ActorID, n.Category)
	if err != nil {
		return models.Notification{}, err
	}
	if muted {
		return models.Notification{}, models.ErrNotificationMuted
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = s.clock()
	}
	if n.ID == "" {
		n.ID = newID()
	}
	if err := s.store.Insert(ctx, roleNotification, n); err != nil {
		return models.Notification{}, classifyNotificationWrite(err)
	}
	return n, nil
}

func classifyNotificationWrite(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateUniqueViolation {
		return models.ErrNotificationCapSpent
	}
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return models.ErrNotificationCapSpent
	}
	return classify(err)
}

func (s *NotificationStore) ListPreferences(ctx context.Context, actorID string) ([]models.NotificationPreference, error) {
	q := s.store.Query(ctx, roleNotificationPreference).
		Where("actor_id = ?", actorID).
		Order("category")
	out, err := specstore.List[models.NotificationPreference](s.store, q, roleNotificationPreference)
	if err != nil {
		return nil, classify(err)
	}
	return out, nil
}

func (s *NotificationStore) SetPreference(ctx context.Context, actorID string, category models.NotificationCategory, muted bool) (models.NotificationPreference, error) {
	p := models.NotificationPreference{
		ID: newID(), ActorID: actorID, Category: category, Muted: muted, ChangedAt: s.clock(),
	}
	if err := Check(s.store.Spec(), roleNotificationPreference, p); err != nil {
		return models.NotificationPreference{}, err
	}
	if err := p.Validate(); err != nil {
		return models.NotificationPreference{}, err
	}
	row, err := encode(s.store.Object(roleNotificationPreference), p)
	if err != nil {
		return models.NotificationPreference{}, err
	}
	err = s.store.Query(ctx, roleNotificationPreference).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "actor_id"}, {Name: "category"}},
			DoUpdates: clause.AssignmentColumns([]string{"muted", "changed_at"}),
		}).Create(row).Error
	if err != nil {
		return models.NotificationPreference{}, classify(err)
	}

	var out models.NotificationPreference
	q := s.store.Query(ctx, roleNotificationPreference).
		Where("actor_id = ? AND category = ?", actorID, string(category))
	if err := s.store.Take(q, roleNotificationPreference, &out, models.ErrNotificationInvalid); err != nil {
		return models.NotificationPreference{}, classify(err)
	}
	return out, nil
}

func (s *NotificationStore) IsMuted(ctx context.Context, actorID string, category models.NotificationCategory) (bool, error) {
	var n int64
	err := s.store.Query(ctx, roleNotificationPreference).
		Where("actor_id = ? AND category = ? AND muted", actorID, string(category)).
		Count(&n).Error
	if err != nil {
		return false, classify(err)
	}
	return n > 0, nil
}

var _ models.NotificationRepository = (*NotificationStore)(nil)
