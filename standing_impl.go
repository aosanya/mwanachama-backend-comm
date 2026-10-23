package mwanachamacomm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/recur"
	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type StandingStore struct {
	db    *gorm.DB
	spec  *spec.Spec
	store *store
	chat  *ChatStore
	clock Clock
}

func NewStandingStore(db *gorm.DB, s *spec.Spec, clock Clock) (*StandingStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewStandingStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	st, err := newStore(db, s, map[string]any{roleStandingMessage: models.StandingMessage{}})
	if err != nil {
		return nil, err
	}
	chat, err := NewChatStore(db, s, clock)
	if err != nil {
		return nil, err
	}
	return &StandingStore{db: db, spec: s, store: st, chat: chat, clock: clock}, nil
}

func scheduleOf(m models.StandingMessage) (recur.Schedule, error) {
	zone, err := time.LoadLocation(m.Zone)
	if err != nil || m.Zone == "" {
		return recur.Schedule{}, fmt.Errorf("%w: %q is not a time zone", models.ErrStandingInvalid, m.Zone)
	}
	sch := recur.Schedule{Start: m.StartsAt, Zone: zone}
	if m.Rule != "" {
		rule, err := recur.ParseRule(m.Rule)
		if err != nil {
			return recur.Schedule{}, fmt.Errorf("%w: %v", models.ErrStandingInvalid, err)
		}
		sch.Rule = &rule
	}
	for _, raw := range m.Dates {
		d, err := recur.ParseDate(raw)
		if err != nil {
			return recur.Schedule{}, fmt.Errorf("%w: %v", models.ErrStandingInvalid, err)
		}
		sch.Dates = append(sch.Dates, d)
	}
	if m.EndsAt != nil {
		sch.End = *m.EndsAt
	}
	if err := sch.Validate(); err != nil {
		return recur.Schedule{}, fmt.Errorf("%w: %v", models.ErrStandingInvalid, err)
	}
	return sch, nil
}

func (s *StandingStore) CreateStanding(ctx context.Context, m models.StandingMessage) (models.StandingMessage, error) {
	m.Body = strings.TrimSpace(m.Body)
	m.ID = newID()
	m.CreatedAt = s.clock()
	m.LastSentAt, m.StoppedAt, m.StoppedBy = nil, nil, ""
	if err := Check(s.store.Spec(), roleStandingMessage, m); err != nil {
		return models.StandingMessage{}, err
	}
	if _, err := scheduleOf(m); err != nil {
		return models.StandingMessage{}, err
	}
	if err := s.store.Insert(ctx, roleStandingMessage, m); err != nil {
		return models.StandingMessage{}, classify(err)
	}
	return s.GetStanding(ctx, m.StructureID, m.ID)
}

func (s *StandingStore) GetStanding(ctx context.Context, structureID, id string) (models.StandingMessage, error) {
	var m models.StandingMessage
	q := s.store.Query(ctx, roleStandingMessage).Where("id = ? AND structure_id = ?", id, structureID)
	if err := s.store.Take(q, roleStandingMessage, &m, models.ErrStandingNotFound); err != nil {
		return models.StandingMessage{}, err
	}
	return m, nil
}

func (s *StandingStore) ListStanding(ctx context.Context, structureID string) ([]models.StandingMessage, error) {
	q := s.store.Query(ctx, roleStandingMessage).Where("structure_id = ?", structureID).Order("created_at, id")
	return specstore.List[models.StandingMessage](s.store, q, roleStandingMessage)
}

func (s *StandingStore) StopStanding(ctx context.Context, structureID, id, stoppedBy string) (models.StandingMessage, error) {
	if strings.TrimSpace(stoppedBy) == "" {
		return models.StandingMessage{}, fmt.Errorf("%w: a stop is somebody's", models.ErrStandingInvalid)
	}
	res := s.store.Query(ctx, roleStandingMessage).
		Where("id = ? AND structure_id = ? AND stopped_at IS NULL", id, structureID).
		Updates(map[string]any{"stopped_at": stamp(s.clock()), "stopped_by": stoppedBy})
	if res.Error != nil {
		return models.StandingMessage{}, classify(res.Error)
	}
	m, err := s.GetStanding(ctx, structureID, id)
	if err != nil {
		return models.StandingMessage{}, err
	}
	if res.RowsAffected == 0 {
		return m, models.ErrStandingStopped
	}
	return m, nil
}

func (s *StandingStore) SendDueStandingMessages(ctx context.Context, now time.Time) ([]models.ChatMessage, error) {
	q := s.store.Query(ctx, roleStandingMessage).Where("stopped_at IS NULL").Order("created_at, id")
	running, err := specstore.List[models.StandingMessage](s.store, q, roleStandingMessage)
	if err != nil {
		return nil, err
	}
	var sent []models.ChatMessage
	var failed []error
	for _, m := range running {
		msg, ok, err := s.sendDue(ctx, m, now)
		if err != nil {
			failed = append(failed, fmt.Errorf("standing message %s: %w", m.ID, err))
			continue
		}
		if ok {
			sent = append(sent, msg)
		}
	}
	return sent, errors.Join(failed...)
}

func (s *StandingStore) sendDue(ctx context.Context, m models.StandingMessage, now time.Time) (models.ChatMessage, bool, error) {
	sch, err := scheduleOf(m)
	if err != nil {
		return models.ChatMessage{}, false, nil
	}
	from := m.StartsAt
	if m.LastSentAt != nil {
		from = m.LastSentAt.Add(time.Nanosecond)
	}
	due := sch.Between(from, now.Add(time.Nanosecond))
	if len(due) == 0 {
		return models.ChatMessage{}, false, nil
	}
	occurrence := due[len(due)-1]
	var posted models.ChatMessage
	var claimed bool
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		claimed, err = claimOccurrence(ctx, tx, s.spec, m, occurrence)
		if err != nil || !claimed {
			return err
		}
		chat, err := NewChatStore(tx, s.spec, s.chat.clock)
		if err != nil {
			return err
		}
		posted, err = chat.Post(ctx, models.ChatMessage{
			StructureID: m.StructureID,
			ThreadID:    m.ThreadID,
			AuthorID:    m.AuthorID,
			Body:        m.Body,
		})
		return err
	})
	if err != nil {
		return models.ChatMessage{}, false, err
	}
	return posted, claimed, nil
}

func claimOccurrence(ctx context.Context, tx *gorm.DB, s *spec.Spec, m models.StandingMessage, occurrence time.Time) (bool, error) {
	st, err := newStore(tx, s, map[string]any{roleStandingMessage: models.StandingMessage{}})
	if err != nil {
		return false, err
	}
	claim := st.Query(ctx, roleStandingMessage).Where("id = ? AND stopped_at IS NULL", m.ID)
	if m.LastSentAt == nil {
		claim = claim.Where("last_sent_at IS NULL")
	} else {
		claim = claim.Where("last_sent_at = ?", stamp(*m.LastSentAt))
	}
	res := claim.Updates(map[string]any{"last_sent_at": stamp(occurrence)})
	if res.Error != nil {
		return false, classify(res.Error)
	}
	return res.RowsAffected > 0, nil
}
