package mwanachamacomm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type PushStore struct {
	store *store
	clock Clock
}

func NewPushStore(db *gorm.DB, s *spec.Spec, clock Clock) (*PushStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewPushStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	st, err := newStore(db, s, map[string]any{
		rolePushSubscription: models.PushSubscription{},
		rolePushDelivery:     models.PushDelivery{},
	})
	if err != nil {
		return nil, err
	}
	return &PushStore{store: st, clock: clock}, nil
}

func (s *PushStore) SubscribePush(ctx context.Context, sub models.PushSubscription) (models.PushSubscription, error) {
	sub.Endpoint = strings.TrimSpace(sub.Endpoint)
	sub.ID = newID()
	sub.CreatedAt = s.clock()
	if err := Check(s.store.Spec(), rolePushSubscription, sub); err != nil {
		return models.PushSubscription{}, err
	}
	if err := sub.Validate(); err != nil {
		return models.PushSubscription{}, err
	}
	row, err := encode(s.store.Object(rolePushSubscription), sub)
	if err != nil {
		return models.PushSubscription{}, err
	}
	err = s.store.Query(ctx, rolePushSubscription).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "endpoint"}},
			DoUpdates: clause.AssignmentColumns([]string{"actor_id", "device_id", "public_key", "auth_secret", "created_at"}),
		}).Create(row).Error
	if err != nil {
		return models.PushSubscription{}, classify(err)
	}

	var out models.PushSubscription
	q := s.store.Query(ctx, rolePushSubscription).Where("endpoint = ?", sub.Endpoint)
	if err := s.store.Take(q, rolePushSubscription, &out, models.ErrPushSubscriptionInvalid); err != nil {
		return models.PushSubscription{}, classify(err)
	}
	return out, nil
}

func (s *PushStore) UnsubscribePush(ctx context.Context, actorID, endpoint string) error {
	if strings.TrimSpace(actorID) == "" {
		return fmt.Errorf("%w: a subscription belongs to somebody", models.ErrPushSubscriptionInvalid)
	}
	return classify(deleteWhere(ctx, s.store, rolePushSubscription,
		"actor_id = ? and endpoint = ?", actorID, strings.TrimSpace(endpoint)))
}

func (s *PushStore) ListPushSubscriptions(ctx context.Context, actorID string) ([]models.PushSubscription, error) {
	q := s.store.Query(ctx, rolePushSubscription).
		Where("actor_id = ?", actorID).
		Order("created_at, id")
	out, err := specstore.List[models.PushSubscription](s.store, q, rolePushSubscription)
	if err != nil {
		return nil, classify(err)
	}
	return out, nil
}

func (s *PushStore) DropPushSubscription(ctx context.Context, id string) error {
	return classify(deleteWhere(ctx, s.store, rolePushSubscription, "id = ?", id))
}

func (s *PushStore) RecordPushDelivery(ctx context.Context, d models.PushDelivery) (models.PushDelivery, error) {
	d.ID = newID()
	d.At = s.clock()
	if err := Check(s.store.Spec(), rolePushDelivery, d); err != nil {
		return models.PushDelivery{}, err
	}
	row, err := encode(s.store.Object(rolePushDelivery), d)
	if err != nil {
		return models.PushDelivery{}, err
	}
	if err := s.store.Query(ctx, rolePushDelivery).Create(row).Error; err != nil {
		return models.PushDelivery{}, classify(err)
	}
	return d, nil
}

func (s *PushStore) ListPushDeliveries(ctx context.Context, notificationID string) ([]models.PushDelivery, error) {
	q := s.store.Query(ctx, rolePushDelivery).
		Where("notification_id = ?", notificationID).
		Order("at, id")
	out, err := specstore.List[models.PushDelivery](s.store, q, rolePushDelivery)
	if err != nil {
		return nil, classify(err)
	}
	return out, nil
}

func (s *PushStore) PrunePushDeliveries(ctx context.Context, before time.Time) (int64, error) {
	res := s.store.Query(ctx, rolePushDelivery).Exec("delete from "+s.store.Table(rolePushDelivery)+" where at < ?", stamp(before))
	if res.Error != nil {
		return 0, classify(res.Error)
	}
	return res.RowsAffected, nil
}
