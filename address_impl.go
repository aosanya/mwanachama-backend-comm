package mwanachamacomm

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type AddressStore struct {
	store *store
	clock Clock
}

func NewAddressStore(db *gorm.DB, s *spec.Spec, clock Clock) (*AddressStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewAddressStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	st, err := newStore(db, s, map[string]any{
		roleAddress:      models.Address{},
		roleAddressBlock: models.AddressBlock{},
	})
	if err != nil {
		return nil, err
	}
	return &AddressStore{store: st, clock: clock}, nil
}

func (s *AddressStore) Publish(ctx context.Context, a models.Address) (models.Address, error) {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = s.clock()
	}
	if err := s.store.Insert(ctx, roleAddress, a); err != nil {
		return models.Address{}, classify(err)
	}
	return a, nil
}

func (s *AddressStore) ListFor(ctx context.Context, actorID string) ([]models.Address, error) {
	q := s.store.Query(ctx, roleAddress).
		Where("actor_id = ?", actorID).
		Order("address_index")
	out, err := specstore.List[models.Address](s.store, q, roleAddress)
	if err != nil {
		return nil, classify(err)
	}
	return out, nil
}

func (s *AddressStore) Resolve(ctx context.Context, hash []byte) (models.Address, error) {
	var a models.Address
	q := s.store.Query(ctx, roleAddress).
		Where("hash = ? AND retired_at IS NULL AND (expires_at IS NULL OR expires_at > ?)",
			models.HashHex(hash), stamp(s.clock()))
	if err := s.store.Take(q, roleAddress, &a, models.ErrAddressNotFound); err != nil {
		return models.Address{}, err
	}
	return a, nil
}

func (s *AddressStore) Retire(ctx context.Context, actorID string, index int, at time.Time) error {
	if at.IsZero() {
		at = s.clock()
	}
	res := s.store.Query(ctx, roleAddress).
		Where("actor_id = ? AND address_index = ? AND retired_at IS NULL", actorID, index).
		UpdateColumn("retired_at", stamp(at))
	if res.Error != nil {
		return classify(res.Error)
	}
	if res.RowsAffected == 0 {
		return models.ErrAddressNotFound
	}
	return nil
}

func (s *AddressStore) CountPublic(ctx context.Context, actorID string) (int, error) {
	var n int64
	err := s.store.Query(ctx, roleAddress).
		Where("actor_id = ? AND public_address <> ''", actorID).
		Count(&n).Error
	if err != nil {
		return 0, classify(err)
	}
	return int(n), nil
}

func (s *AddressStore) UpdateSettings(ctx context.Context, actorID string, index int, set models.AddressSettings) (models.Address, error) {
	if err := set.Validate(); err != nil {
		return models.Address{}, err
	}

	o := s.store.Object(roleAddress)
	full, err := encode(o, models.Address{}.WithSettings(set))
	if err != nil {
		return models.Address{}, err
	}
	values := make(map[string]any, 8)
	for _, column := range []string{
		"expires_at", "expiry_mode", "message_ttl_seconds",
		"public_address", "disabled_at", "disabled_mode", "hours", "listed_at",
	} {
		values[column] = full[column]
	}

	res := s.store.Query(ctx, roleAddress).
		Where("actor_id = ? AND address_index = ? AND retired_at IS NULL", actorID, index).
		Updates(values)
	if res.Error != nil {
		return models.Address{}, classify(res.Error)
	}
	if res.RowsAffected == 0 {
		return models.Address{}, models.ErrAddressNotFound
	}

	var out models.Address
	q := s.store.Query(ctx, roleAddress).
		Where("actor_id = ? AND address_index = ?", actorID, index)
	if err := s.store.Take(q, roleAddress, &out, models.ErrAddressNotFound); err != nil {
		return models.Address{}, classify(err)
	}
	return out, nil
}

func (s *AddressStore) Block(ctx context.Context, b models.AddressBlock) error {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = s.clock()
	}
	row, err := encode(s.store.Object(roleAddressBlock), b)
	if err != nil {
		return err
	}
	err = s.store.Query(ctx, roleAddressBlock).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(row).Error
	if err != nil {
		return classify(err)
	}
	return nil
}

func (s *AddressStore) IsBlocked(ctx context.Context, actorID string, hash []byte) (bool, error) {
	var n int64
	err := s.store.Query(ctx, roleAddressBlock).
		Where("actor_id = ? AND hash = ?", actorID, models.HashHex(hash)).
		Count(&n).Error
	if err != nil {
		return false, classify(err)
	}
	return n > 0, nil
}

func (s *AddressStore) ListListed(ctx context.Context, excludeActorID string, now time.Time) ([]models.Address, error) {
	q := s.store.Query(ctx, roleAddress).
		Where("listed_at IS NOT NULL AND public_address <> '' AND retired_at IS NULL AND (expires_at IS NULL OR expires_at > ?)",
			stamp(now))
	if excludeActorID != "" {
		q = q.Where("actor_id <> ?", excludeActorID)
	}
	out, err := specstore.List[models.Address](s.store, q, roleAddress)
	if err != nil {
		return nil, classify(err)
	}
	return out, nil
}

var _ models.AddressRepository = (*AddressStore)(nil)
