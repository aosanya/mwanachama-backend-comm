package mwanachamacomm

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type SubjectStore struct {
	store *store
	clock Clock
}

func NewSubjectStore(db *gorm.DB, s *spec.Spec, clock Clock) (*SubjectStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewSubjectStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	st, err := newStore(db, s, map[string]any{
		roleSubjectThread:  models.SubjectThread{},
		roleSubjectMessage: models.SubjectThreadMessage{},
	})
	if err != nil {
		return nil, err
	}
	return &SubjectStore{store: st, clock: clock}, nil
}

func (s *SubjectStore) Resolve(ctx context.Context, kind, id string, audience models.SubjectAudience, ownerID string) (models.SubjectThread, error) {
	var t models.SubjectThread
	q := s.store.Query(ctx, roleSubjectThread).
		Where("subject_kind = ? AND subject_id = ? AND audience = ? AND owner_id = ?", kind, id, string(audience), ownerID)
	if err := s.store.Take(q, roleSubjectThread, &t, models.ErrSubjectNotFound); err != nil {
		return models.SubjectThread{}, err
	}
	return t, nil
}

func (s *SubjectStore) Open(ctx context.Context, kind, id string, audience models.SubjectAudience, ownerID string) (models.SubjectThread, error) {
	if t, err := s.Resolve(ctx, kind, id, audience, ownerID); err == nil {
		return t, nil
	} else if !errors.Is(err, models.ErrSubjectNotFound) {
		return models.SubjectThread{}, err
	}
	t := models.SubjectThread{
		ID: mintID(prefixSubjectThread), SubjectKind: kind, SubjectID: id,
		Audience: audience, OwnerID: ownerID, CreatedAt: s.clock(),
	}
	if err := Check(s.store.Spec(), roleSubjectThread, t); err != nil {
		return models.SubjectThread{}, err
	}
	if err := s.store.Insert(ctx, roleSubjectThread, t); err != nil {
		if existing, rerr := s.Resolve(ctx, kind, id, audience, ownerID); rerr == nil {
			return existing, nil
		}
		return models.SubjectThread{}, classify(err)
	}
	return t, nil
}

func (s *SubjectStore) Thread(ctx context.Context, threadID string) (models.SubjectThread, error) {
	var t models.SubjectThread
	q := s.store.Query(ctx, roleSubjectThread).Where("id = ?", threadID)
	if err := s.store.Take(q, roleSubjectThread, &t, models.ErrSubjectNotFound); err != nil {
		return models.SubjectThread{}, err
	}
	return t, nil
}

func (s *SubjectStore) Threads(ctx context.Context, kind, id string) ([]models.SubjectThread, error) {
	q := s.store.Query(ctx, roleSubjectThread).
		Where("subject_kind = ? AND subject_id = ?", kind, id).
		Order("created_at, id")
	return specstore.List[models.SubjectThread](s.store, q, roleSubjectThread)
}

func (s *SubjectStore) Views(ctx context.Context, threads []models.SubjectThread) ([]models.SubjectThreadView, error) {
	out := make([]models.SubjectThreadView, len(threads))
	if len(threads) == 0 {
		return out, nil
	}
	ids := make([]string, len(threads))
	at := make(map[string]int, len(threads))
	for i, t := range threads {
		ids[i] = t.ID
		at[t.ID] = i
		out[i] = models.SubjectThreadView{SubjectThread: t}
	}
	rows, err := s.store.Query(ctx, roleSubjectMessage).
		Select("thread_id, count(*), max(created_at)").
		Where("thread_id IN ?", ids).
		Group("thread_id").Rows()
	if err != nil {
		return nil, classify(err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			threadID string
			count    int
			last     flexTime
		)
		if err := rows.Scan(&threadID, &count, &last); err != nil {
			return nil, classify(err)
		}
		i := at[threadID]
		out[i].MessageCount = count
		if last.Valid {
			when := last.Time
			out[i].LastMessageAt = &when
		}
	}
	return out, rows.Err()
}

func (s *SubjectStore) Messages(ctx context.Context, threadID string) ([]models.SubjectThreadMessage, error) {
	q := s.store.Query(ctx, roleSubjectMessage).Where("thread_id = ?", threadID).Order("created_at, id")
	return specstore.List[models.SubjectThreadMessage](s.store, q, roleSubjectMessage)
}

func (s *SubjectStore) Post(ctx context.Context, m models.SubjectThreadMessage) (models.SubjectThreadMessage, error) {
	m.ID = mintID(prefixSubjectMessage)
	m.CreatedAt = s.clock()
	if err := Check(s.store.Spec(), roleSubjectMessage, m); err != nil {
		return models.SubjectThreadMessage{}, err
	}
	if err := s.store.Insert(ctx, roleSubjectMessage, m); err != nil {
		return models.SubjectThreadMessage{}, classify(err)
	}
	return m, nil
}

func (s *SubjectStore) AudienceAuthors(ctx context.Context, threadID string) ([]string, error) {
	var authors []string
	err := s.store.Query(ctx, roleSubjectMessage).
		Where("thread_id = ? AND by_answerer = ?", threadID, false).
		Distinct().Order("author_id").Pluck("author_id", &authors).Error
	if err != nil {
		return nil, classify(err)
	}
	return authors, nil
}
