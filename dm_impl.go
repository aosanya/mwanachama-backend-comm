// dm_impl.go — GORM-backed DMRepository implementation, thread reads +
// creation. Roster writes (Invite/Accept/Leave/Kick/Promote/ReEnable) live
// in dm_roster_impl.go, and message/device-key/reaction methods in
// dm_message_impl.go — a three-way, not the original two-way,
// [[file-length-limit]] split, since this port's single GORM store folds
// what used to be separate Postgres and memory implementations into one.
package mwanachamacomm

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

// DMStore is the GORM implementation of [models.DMRepository].
type DMStore struct {
	db     *gorm.DB
	tables TableNames
	clock  Clock
}

// NewDMStore constructs a DMStore backed by db. See [NewChatStore] for the
// shared constructor contract (nil db, default clock).
func NewDMStore(db *gorm.DB, t TableNames, clock Clock) (*DMStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewDMStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	return &DMStore{db: db, tables: t, clock: clock}, nil
}

func (s *DMStore) CreateThread(ctx context.Context, t models.DMThread, initial []string) (models.DMThread, error) {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = s.clock()
	}
	row := gormstore.DMThreadToRow(t)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(s.tables.DMThreads).Create(&row).Error; err != nil {
			return err
		}
		if row.CreatedBy != "" {
			p := gormstore.DMParticipantRow{ThreadID: row.ID, MemberID: row.CreatedBy, State: string(models.DMStateActive), IsAdmin: true, UpdatedAt: s.clock()}
			if err := tx.Table(s.tables.DMParticipants).Create(&p).Error; err != nil {
				return err
			}
		}
		for _, m := range initial {
			if m == row.CreatedBy {
				continue
			}
			p := gormstore.DMParticipantRow{ThreadID: row.ID, MemberID: m, State: string(models.DMStateInvited), UpdatedAt: s.clock()}
			if err := tx.Table(s.tables.DMParticipants).Create(&p).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.DMThread{}, classify(err)
	}
	return gormstore.DMThreadFromRow(row), nil
}

// GetThread returns a thread by id.
func (s *DMStore) GetThread(ctx context.Context, id string) (models.DMThread, error) {
	var row gormstore.DMThreadRow
	err := s.db.WithContext(ctx).Table(s.tables.DMThreads).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.DMThread{}, models.ErrDMNotFound
	}
	if err != nil {
		return models.DMThread{}, err
	}
	return gormstore.DMThreadFromRow(row), nil
}

func (s *DMStore) ListThreadsFor(ctx context.Context, memberID string) ([]models.DMThread, error) {
	var rows []gormstore.DMThreadRow
	err := s.db.WithContext(ctx).Table(s.tables.DMThreads+" AS t").
		Select("t.*").
		Joins("JOIN "+s.tables.DMParticipants+" AS p ON p.thread_id = t.id").
		Where("p.member_id = ? AND p.state IN ?", memberID, []string{string(models.DMStateActive), string(models.DMStateInvited)}).
		Order("t.created_at, t.id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]models.DMThread, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.DMThreadFromRow(r))
	}
	return out, nil
}

// ListParticipants returns every participant row for a thread.
func (s *DMStore) ListParticipants(ctx context.Context, threadID string) ([]models.DMParticipant, error) {
	var rows []gormstore.DMParticipantRow
	err := s.db.WithContext(ctx).Table(s.tables.DMParticipants).
		Where("thread_id = ?", threadID).Order("updated_at, member_id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]models.DMParticipant, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.DMParticipantFromRow(r))
	}
	return out, nil
}

var _ models.DMRepository = (*DMStore)(nil)
