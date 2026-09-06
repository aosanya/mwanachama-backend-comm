// notification_impl.go — GORM-backed NotificationRepository implementation.
// Ported from mwanachama-backend-api-gateway's internal/store/{postgres,
// memory}/notification_store.go: one store now, run against Postgres in
// production and sqlite in tests, mirroring this repo's other domains'
// storage swap.
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

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

// NotificationStore is the GORM implementation of
// [models.NotificationRepository].
type NotificationStore struct {
	db     *gorm.DB
	tables TableNames
	clock  Clock
}

// NewNotificationStore constructs a NotificationStore backed by db, reading
// and writing the tables named by t. Callers must run [Migrate] against the
// same db and t before use. clock defaults to [SystemClock] when nil.
func NewNotificationStore(db *gorm.DB, t TableNames, clock Clock) (*NotificationStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewNotificationStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	return &NotificationStore{db: db, tables: t, clock: clock}, nil
}

// List returns one member's notifications, newest first.
func (s *NotificationStore) List(ctx context.Context, memberID string, limit int) ([]models.Notification, error) {
	if limit <= 0 {
		limit = models.NotificationDefaultPage
	}
	var rows []gormstore.NotificationRow
	err := s.db.WithContext(ctx).Table(s.tables.Notifications).
		Where("member_id = ?", memberID).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, classify(err)
	}
	out := make([]models.Notification, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.NotificationFromRow(r))
	}
	return out, nil
}

// UnreadCount is the badge: count(*) where read_at is null, per member.
func (s *NotificationStore) UnreadCount(ctx context.Context, memberID string) (int, error) {
	var n int64
	err := s.db.WithContext(ctx).Table(s.tables.Notifications).
		Where("member_id = ? AND read_at IS NULL", memberID).
		Count(&n).Error
	if err != nil {
		return 0, classify(err)
	}
	return int(n), nil
}

// MarkRead stamps read_at on the caller's own unread rows named by ids and
// returns how many it changed. Ids that are not this member's, or that are
// already read, are silently skipped rather than refused.
func (s *NotificationStore) MarkRead(ctx context.Context, memberID string, ids []string, readAt time.Time) (int, error) {
	if len(ids) > models.NotificationMaxMarkRead {
		return 0, fmt.Errorf("%w: at most %d notifications in one call", models.ErrNotificationInvalid, models.NotificationMaxMarkRead)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	res := s.db.WithContext(ctx).Table(s.tables.Notifications).
		Where("member_id = ? AND id IN ? AND read_at IS NULL", memberID, ids).
		UpdateColumn("read_at", readAt)
	if res.Error != nil {
		return 0, classify(res.Error)
	}
	return int(res.RowsAffected), nil
}

// Raise inserts one notification as the side-effect of an act — see
// [models.NotificationRepository.Raise] for the mute/cap rules this
// enforces.
func (s *NotificationStore) Raise(ctx context.Context, n models.Notification) (models.Notification, error) {
	if err := n.Validate(); err != nil {
		return models.Notification{}, err
	}
	muted, err := s.IsMuted(ctx, n.MemberID, n.Category)
	if err != nil {
		return models.Notification{}, err
	}
	if muted {
		return models.Notification{}, models.ErrNotificationMuted
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = s.clock()
	}
	row := gormstore.NotificationToRow(n)
	if err := s.db.WithContext(ctx).Table(s.tables.Notifications).Create(&row).Error; err != nil {
		return models.Notification{}, classifyNotificationWrite(err)
	}
	return gormstore.NotificationFromRow(row), nil
}

// classifyNotificationWrite maps a Raise insert failure onto
// ErrNotificationCapSpent where it is one of the two partial unique
// indexes (see gormstore.syncNotificationCapIndexes) answering — the only
// unique constraint this table carries — and to classify's generic mapping
// otherwise.
func classifyNotificationWrite(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return models.ErrNotificationCapSpent
	}
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return models.ErrNotificationCapSpent
	}
	return classify(err)
}

// ListPreferences returns the rows one member has actually written, sorted
// by category. Absence means on — see [models.NotificationRepository.ListPreferences].
func (s *NotificationStore) ListPreferences(ctx context.Context, memberID string) ([]models.NotificationPreference, error) {
	var rows []gormstore.NotificationPreferenceRow
	err := s.db.WithContext(ctx).Table(s.tables.NotificationPreferences).
		Where("member_id = ?", memberID).
		Order("category").
		Find(&rows).Error
	if err != nil {
		return nil, classify(err)
	}
	out := make([]models.NotificationPreference, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.NotificationPreferenceFromRow(r))
	}
	return out, nil
}

// SetPreference upserts on (member_id, category) — Validate refuses
// `survey`/`security` before this ever reaches the table.
func (s *NotificationStore) SetPreference(ctx context.Context, memberID string, category models.NotificationCategory, muted bool) (models.NotificationPreference, error) {
	p := models.NotificationPreference{MemberID: memberID, Category: category, Muted: muted, ChangedAt: s.clock()}
	if err := p.Validate(); err != nil {
		return models.NotificationPreference{}, err
	}
	row := gormstore.NotificationPreferenceToRow(p)
	err := s.db.WithContext(ctx).Table(s.tables.NotificationPreferences).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "member_id"}, {Name: "category"}},
			DoUpdates: clause.AssignmentColumns([]string{"muted", "changed_at"}),
		}).Create(&row).Error
	if err != nil {
		return models.NotificationPreference{}, classify(err)
	}
	// Re-read rather than trust the row GORM's Create call was handed: on
	// the update arm of an ON CONFLICT, the existing row's own id is what
	// the table keeps, and not every dialect this package supports scans a
	// RETURNING clause back onto a conflicted row identically.
	var out gormstore.NotificationPreferenceRow
	err = s.db.WithContext(ctx).Table(s.tables.NotificationPreferences).
		Where("member_id = ? AND category = ?", memberID, string(category)).
		First(&out).Error
	if err != nil {
		return models.NotificationPreference{}, classify(err)
	}
	return gormstore.NotificationPreferenceFromRow(out), nil
}

// IsMuted is the send-time read Raise makes: not exists (… where muted) —
// absent is on.
func (s *NotificationStore) IsMuted(ctx context.Context, memberID string, category models.NotificationCategory) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Table(s.tables.NotificationPreferences).
		Where("member_id = ? AND category = ? AND muted", memberID, string(category)).
		Count(&n).Error
	if err != nil {
		return false, classify(err)
	}
	return n > 0, nil
}

var _ models.NotificationRepository = (*NotificationStore)(nil)
