// address_impl.go — GORM-backed AddressRepository implementation. Ported
// from mwanachama-backend-api-gateway's internal/store/{postgres,memory}/
// address_store.go: one store now, run against Postgres in production and
// sqlite in tests, mirroring this repo's other domains' storage swap.
package mwanachamacomm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

// AddressStore is the GORM implementation of [models.AddressRepository].
type AddressStore struct {
	db     *gorm.DB
	tables TableNames
	clock  Clock
}

// NewAddressStore constructs an AddressStore backed by db, reading and
// writing the tables named by t. Callers must run [Migrate] against the
// same db and t before use. clock defaults to [SystemClock] when nil.
func NewAddressStore(db *gorm.DB, t TableNames, clock Clock) (*AddressStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewAddressStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	return &AddressStore{db: db, tables: t, clock: clock}, nil
}

// Publish records an address, refusing a duplicate.
//
// The refusal comes from the primary key (Hash) and the
// (member_id, address_index) unique index rather than a read-then-write,
// so two devices racing on the same address cannot both win.
func (s *AddressStore) Publish(ctx context.Context, a models.Address) (models.Address, error) {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = s.clock()
	}
	row, err := gormstore.AddressToRow(a)
	if err != nil {
		return models.Address{}, err
	}
	if err := s.db.WithContext(ctx).Table(s.tables.Addresses).Create(&row).Error; err != nil {
		return models.Address{}, classify(err)
	}
	return gormstore.AddressFromRow(row), nil
}

// ListFor returns every address a member holds, retired included — the
// member's own view of their several phones, and a retired one they can
// still see is how they know it is retired.
func (s *AddressStore) ListFor(ctx context.Context, memberID string) ([]models.Address, error) {
	var rows []gormstore.AddressRow
	err := s.db.WithContext(ctx).Table(s.tables.Addresses).
		Where("member_id = ?", memberID).
		Order("address_index").
		Find(&rows).Error
	if err != nil {
		return nil, classify(err)
	}
	out := make([]models.Address, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.AddressFromRow(r))
	}
	return out, nil
}

// Resolve finds the live address behind a hash.
//
// A retired address and an unknown one are the same answer on purpose: a
// caller walking the space must not be able to tell "nobody has this" from
// "somebody had this and stopped using it", which would turn probing into
// a census. An expired address is the same answer again — all three come
// back as one ErrAddressNotFound.
//
// The expiry test binds this store's own clock rather than calling a SQL
// now(), so the comparison is identical on both dialects this package
// supports and deterministic under this package's test clock.
func (s *AddressStore) Resolve(ctx context.Context, hash []byte) (models.Address, error) {
	var row gormstore.AddressRow
	err := s.db.WithContext(ctx).Table(s.tables.Addresses).
		Where("hash = ? AND retired_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", hash, s.clock()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Address{}, models.ErrAddressNotFound
	}
	if err != nil {
		return models.Address{}, classify(err)
	}
	return gormstore.AddressFromRow(row), nil
}

// Retire stops an address accepting new threads.
//
// Scoped to its owner in the WHERE clause, so naming someone else's
// address touches nothing and reports ErrAddressNotFound. Already-retired
// rows are excluded too, so retiring twice cannot move the timestamp.
func (s *AddressStore) Retire(ctx context.Context, memberID string, index int, at time.Time) error {
	if at.IsZero() {
		at = s.clock()
	}
	res := s.db.WithContext(ctx).Table(s.tables.Addresses).
		Where("member_id = ? AND address_index = ? AND retired_at IS NULL", memberID, index).
		UpdateColumn("retired_at", at)
	if res.Error != nil {
		return classify(res.Error)
	}
	if res.RowsAffected == 0 {
		return models.ErrAddressNotFound
	}
	return nil
}

// CountPublic returns how many of a member's rows hold a plaintext
// address. Retired rows included, deliberately — see the interface doc.
func (s *AddressStore) CountPublic(ctx context.Context, memberID string) (int, error) {
	var n int64
	err := s.db.WithContext(ctx).Table(s.tables.Addresses).
		Where("member_id = ? AND public_address IS NOT NULL", memberID).
		Count(&n).Error
	if err != nil {
		return 0, classify(err)
	}
	return int(n), nil
}

// UpdateSettings replaces every setting on one of a member's own addresses.
//
// **One UPDATE, whole-object.** The caller sends the state it wants;
// absent means off, so every settings column is written on every call
// (via a map, not a struct — GORM's struct-based Updates skips Go zero
// values, which would make "cleared" and "unmentioned" indistinguishable
// here). Scoped to its owner in the WHERE clause; retired rows are
// excluded, since an address the member turned off has no terms left
// worth setting.
func (s *AddressStore) UpdateSettings(ctx context.Context, memberID string, index int, set models.AddressSettings) (models.Address, error) {
	if err := set.Validate(); err != nil {
		return models.Address{}, err
	}
	// nil rather than the JSON literal `null`, so the column is genuinely
	// NULL for an address on no schedule.
	var hours []byte
	if set.Hours != nil {
		h, err := json.Marshal(set.Hours)
		if err != nil {
			return models.Address{}, err
		}
		hours = h
	}
	values := map[string]any{
		"expires_at":          set.ExpiresAt,
		"expiry_mode":         set.ExpiryMode,
		"message_ttl_seconds": set.MessageTTLSeconds,
		"public_address":      gormstore.StringToNullable(set.PublicAddress),
		"disabled_at":         set.DisabledAt,
		"disabled_mode":       set.DisabledMode,
		"hours":               hours,
		"listed_at":           set.ListedAt,
	}
	res := s.db.WithContext(ctx).Table(s.tables.Addresses).
		Where("member_id = ? AND address_index = ? AND retired_at IS NULL", memberID, index).
		Updates(values)
	if res.Error != nil {
		return models.Address{}, classify(res.Error)
	}
	if res.RowsAffected == 0 {
		return models.Address{}, models.ErrAddressNotFound
	}
	var row gormstore.AddressRow
	if err := s.db.WithContext(ctx).Table(s.tables.Addresses).
		Where("member_id = ? AND address_index = ?", memberID, index).
		First(&row).Error; err != nil {
		return models.Address{}, classify(err)
	}
	return gormstore.AddressFromRow(row), nil
}

// Block records that a member refuses to be reached at an address.
//
// Idempotent by primary key: a member pressing Block again has asked for
// the state that already holds.
func (s *AddressStore) Block(ctx context.Context, b models.AddressBlock) error {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = s.clock()
	}
	row := gormstore.AddressBlockToRow(b)
	err := s.db.WithContext(ctx).Table(s.tables.AddressBlocks).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row).Error
	if err != nil {
		return classify(err)
	}
	return nil
}

// IsBlocked reports whether memberID has blocked addr.
func (s *AddressStore) IsBlocked(ctx context.Context, memberID string, hash []byte) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Table(s.tables.AddressBlocks).
		Where("member_id = ? AND hash = ?", memberID, hash).
		Count(&n).Error
	if err != nil {
		return false, classify(err)
	}
	return n > 0, nil
}

// ListListed returns every address that belongs in the directory — listed
// by its owner and not permanently stopped — with one member's own rows
// left out (empty excludeMemberID excludes nothing).
//
// Not part of [models.AddressRepository]: it exists for a caller building
// its own [models.AddressDirectory] by composing this method with a
// member-name lookup, rather than through [AddressDirectoryStore]'s SQL
// join — the shape a deployment needs when address's table and the
// member table it joins against do not share one physical database (the
// gateway's own memory-backend wiring is exactly this case; see that
// repo's internal/store/memory/address_directory.go).
func (s *AddressStore) ListListed(ctx context.Context, excludeMemberID string, now time.Time) ([]models.Address, error) {
	var rows []gormstore.AddressRow
	q := s.db.WithContext(ctx).Table(s.tables.Addresses).
		Where("listed_at IS NOT NULL AND public_address IS NOT NULL AND retired_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", now)
	if excludeMemberID != "" {
		q = q.Where("member_id <> ?", excludeMemberID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, classify(err)
	}
	out := make([]models.Address, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.AddressFromRow(r))
	}
	return out, nil
}

var _ models.AddressRepository = (*AddressStore)(nil)
