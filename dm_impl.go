package mwanachamacomm

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type DMStore struct {
	store *store
	clock Clock
}

func NewDMStore(db *gorm.DB, s *spec.Spec, clock Clock) (*DMStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewDMStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	st, err := newStore(db, s, map[string]any{
		roleDMThread:      models.DMThread{},
		roleDMParticipant: models.DMParticipant{},
		roleDMMessage:     models.DMMessage{},
		roleDMReaction:    models.DMReaction{},
		roleDMDeviceKey:   models.DMDeviceKey{},
	})
	if err != nil {
		return nil, err
	}
	return &DMStore{store: st, clock: clock}, nil
}

func (s *DMStore) CreateThread(ctx context.Context, t models.DMThread, initial []string) (models.DMThread, error) {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = s.clock()
	}
	if t.ID == "" {
		t.ID = mintID(prefixDMThread)
	}

	threads, participants := s.store.Object(roleDMThread), s.store.Object(roleDMParticipant)
	threadRow, err := encode(threads, t)
	if err != nil {
		return models.DMThread{}, err
	}

	err = s.store.Query(ctx, roleDMThread).Session(&gorm.Session{}).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(s.store.Table(roleDMThread)).Create(threadRow).Error; err != nil {
			return err
		}
		add := func(actorID string, state models.DMParticipantState, admin bool) error {
			row, err := encode(participants, models.DMParticipant{
				ThreadID: t.ID, ActorID: actorID, State: state, IsAdmin: admin, UpdatedAt: s.clock(),
			})
			if err != nil {
				return err
			}
			return tx.Table(s.store.Table(roleDMParticipant)).Create(row).Error
		}
		if t.CreatedBy != "" {
			if err := add(t.CreatedBy, models.DMStateActive, true); err != nil {
				return err
			}
		}
		for _, m := range initial {
			if m == t.CreatedBy {
				continue
			}
			if err := add(m, models.DMStateInvited, false); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.DMThread{}, classify(err)
	}
	return t, nil
}

func (s *DMStore) GetThread(ctx context.Context, id string) (models.DMThread, error) {
	var t models.DMThread
	q := s.store.Query(ctx, roleDMThread).Where("id = ?", id)
	if err := s.store.Take(q, roleDMThread, &t, models.ErrDMNotFound); err != nil {
		return models.DMThread{}, err
	}
	return t.WithDerived(), nil
}

func (s *DMStore) ListThreadsFor(ctx context.Context, actorID string) ([]models.DMThread, error) {
	threads, participants := s.store.Table(roleDMThread), s.store.Table(roleDMParticipant)
	q := s.store.Query(ctx, roleDMThread).
		Table(threads+" AS t").
		Select("t.*").
		Joins("JOIN "+participants+" AS p ON p.thread_id = t.id").
		Where("p.actor_id = ? AND p.state IN ?", actorID,
			[]string{string(models.DMStateActive), string(models.DMStateInvited)}).
		Order("t.created_at, t.id")

	out, err := specstore.List[models.DMThread](s.store, q, roleDMThread)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i] = out[i].WithDerived()
	}
	return out, nil
}

func (s *DMStore) ListParticipants(ctx context.Context, threadID string) ([]models.DMParticipant, error) {
	q := s.store.Query(ctx, roleDMParticipant).
		Where("thread_id = ?", threadID).Order("updated_at, actor_id")
	return specstore.List[models.DMParticipant](s.store, q, roleDMParticipant)
}

var _ models.DMRepository = (*DMStore)(nil)
