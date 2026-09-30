package mwanachamacomm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type ChatStore struct {
	store *store
	clock Clock
}

func NewChatStore(db *gorm.DB, s *spec.Spec, clock Clock) (*ChatStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewChatStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	st, err := newStore(db, s, map[string]any{
		roleChatThread:  models.ChatThread{},
		roleChatMessage: models.ChatMessage{},
	})
	if err != nil {
		return nil, err
	}
	return &ChatStore{store: st, clock: clock}, nil
}

func (s *ChatStore) MintThread(ctx context.Context, structureID, tagPath string) (models.ChatThread, error) {
	if t, err := s.ResolveThread(ctx, structureID, tagPath); err == nil {
		return t, nil
	} else if !errors.Is(err, models.ErrChatNotFound) {
		return models.ChatThread{}, err
	}

	t := models.ChatThread{
		ID:          mintID(prefixChatThread),
		StructureID: structureID,
		TagPath:     tagPath,
		CreatedAt:   s.clock(),
	}
	if err := s.store.Insert(ctx, roleChatThread, t); err != nil {
		if existing, rerr := s.ResolveThread(ctx, structureID, tagPath); rerr == nil {
			return existing, nil
		}
		return models.ChatThread{}, classify(err)
	}
	return t, nil
}

func (s *ChatStore) ResolveThread(ctx context.Context, structureID, tagPath string) (models.ChatThread, error) {
	var t models.ChatThread
	q := s.store.Query(ctx, roleChatThread).
		Where("structure_id = ? AND tag_path = ?", structureID, tagPath).
		Order("created_at, id")
	if err := s.store.Take(q, roleChatThread, &t, models.ErrChatNotFound); err != nil {
		return models.ChatThread{}, err
	}
	return t, nil
}

func (s *ChatStore) ListThreads(ctx context.Context, structureID string) ([]models.ChatThread, error) {
	q := s.store.Query(ctx, roleChatThread).Where("structure_id = ?", structureID).Order("created_at, id")
	return specstore.List[models.ChatThread](s.store, q, roleChatThread)
}

func (s *ChatStore) Post(ctx context.Context, m models.ChatMessage) (models.ChatMessage, error) {
	m.CreatedAt = s.clock()
	if m.ID == "" {
		m.ID = mintID(prefixChatMessage)
	}
	if err := s.store.Insert(ctx, roleChatMessage, m); err != nil {
		return models.ChatMessage{}, classify(err)
	}
	return m, nil
}

func (s *ChatStore) GetMessage(ctx context.Context, id string) (models.ChatMessage, error) {
	var m models.ChatMessage
	q := s.store.Query(ctx, roleChatMessage).Where("id = ?", id)
	if err := s.store.Take(q, roleChatMessage, &m, models.ErrChatNotFound); err != nil {
		return models.ChatMessage{}, err
	}
	return m, nil
}

func (s *ChatStore) ListMessages(ctx context.Context, structureID, threadID string) ([]models.ChatMessage, error) {
	q := s.store.Query(ctx, roleChatMessage).Where("structure_id = ?", structureID)
	if threadID != "" {
		q = q.Where("thread_id = ?", threadID)
	}
	return specstore.List[models.ChatMessage](s.store, q.Order("created_at, id"), roleChatMessage)
}

var _ models.ChatRepository = (*ChatStore)(nil)

const chatRoomFilter = "/*room*/"

func chatActivitySQL(threads, messages string) string {
	return `
	SELECT COALESCE(t.structure_id, m.structure_id) AS structure_id,
	       COALESCE(t.threads, 0),
	       COALESCE(m.messages, 0),
	       m.last_post_at
	  FROM (SELECT structure_id, count(*) AS threads
	          FROM ` + threads + ` ` + chatRoomFilter + ` GROUP BY structure_id) t
	  FULL OUTER JOIN
	       (SELECT structure_id, count(*) AS messages, max(created_at) AS last_post_at
	          FROM ` + messages + ` ` + chatRoomFilter + ` GROUP BY structure_id) m
	    ON m.structure_id = t.structure_id`
}

func (s *ChatStore) Activity(ctx context.Context, q models.ChatActivityQuery) (models.ChatActivityPage, error) {
	where := ""
	var args []any
	if q.StructureID != "" {
		args = append(args, q.StructureID, q.StructureID)
		where = "WHERE structure_id = ?"
	}
	query := strings.ReplaceAll(
		chatActivitySQL(s.store.Table(roleChatThread), s.store.Table(roleChatMessage)),
		chatRoomFilter, where,
	) + `
	  ORDER BY (m.last_post_at IS NULL) ASC, m.last_post_at DESC, COALESCE(t.structure_id, m.structure_id)`

	rows, err := s.store.Query(ctx, roleChatThread).Raw(query, args...).Rows()
	if err != nil {
		return models.ChatActivityPage{}, err
	}
	defer rows.Close()

	page := models.ChatActivityPage{Rows: []models.ChatActivityRow{}}
	var latest *time.Time
	i := 0
	for rows.Next() {
		var (
			r    models.ChatActivityRow
			last flexTime
		)
		if err := rows.Scan(&r.StructureID, &r.Threads, &r.Messages, &last); err != nil {
			return models.ChatActivityPage{}, err
		}
		if last.Valid {
			at := last.Time
			r.LastPostAt = &at
			if latest == nil || at.After(*latest) {
				latest = &at
			}
		}
		page.Total++
		page.Threads += r.Threads
		page.Messages += r.Messages

		if i >= q.Offset && (q.Limit == nil || len(page.Rows) < *q.Limit) {
			page.Rows = append(page.Rows, r)
		}
		i++
	}
	if err := rows.Err(); err != nil {
		return models.ChatActivityPage{}, err
	}
	page.LastPostAt = latest
	return page, nil
}
