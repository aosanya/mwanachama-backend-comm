package mwanachamacomm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

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
		roleChatThread:     models.ChatThread{},
		roleChatMessage:    models.ChatMessage{},
		roleChatReadMarker: models.ChatReadMarker{},
		roleChatArchive:    models.ChatArchive{},
	})
	if err != nil {
		return nil, err
	}
	return &ChatStore{store: st, clock: clock}, nil
}

func (s *ChatStore) MintThread(ctx context.Context, structureID, tagPath string) (models.ChatThread, error) {
	return s.mintThread(ctx, structureID, tagPath, "", models.ChatAudienceGroup)
}

func (s *ChatStore) StartThread(ctx context.Context, structureID, label string) (models.ChatThread, error) {
	label = strings.Join(strings.Fields(label), " ")
	tagPath, ok := labelTagPath(label)
	if !ok {
		return models.ChatThread{}, fmt.Errorf("%w: label is required", models.ErrChatInvalid)
	}
	return s.mintThread(ctx, structureID, tagPath, label, models.ChatAudienceLeads)
}

func (s *ChatStore) OpenThread(ctx context.Context, structureID, threadID string) (models.ChatThread, error) {
	t, err := s.threadIn(ctx, structureID, threadID)
	if err != nil {
		return models.ChatThread{}, err
	}
	if t.Declined() {
		return models.ChatThread{}, models.ErrChatNotFound
	}
	if t.Audience == models.ChatAudienceGroup {
		return t, nil
	}
	err = s.store.Query(ctx, roleChatThread).
		Where("id = ? AND structure_id = ?", threadID, structureID).
		Updates(map[string]any{"audience": string(models.ChatAudienceGroup)}).Error
	if err != nil {
		return models.ChatThread{}, classify(err)
	}
	t.Audience = models.ChatAudienceGroup
	return t, nil
}

func labelTagPath(label string) (string, bool) {
	var words []string
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			words = append(words, word.String())
			word.Reset()
		}
	}
	for _, r := range strings.ToLower(label) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	if len(words) == 0 {
		return "", false
	}
	return "/" + strings.Join(words, "-"), true
}

func (s *ChatStore) mintThread(ctx context.Context, structureID, tagPath, label string, audience models.ChatAudience) (models.ChatThread, error) {
	t, _, err := s.mintUnlessReusable(ctx, models.ChatThread{
		StructureID: structureID, TagPath: tagPath, Label: label, Audience: audience,
	}, func(models.ChatThread) bool { return true })
	return t, err
}

func (s *ChatStore) RequestThread(ctx context.Context, structureID, label, requestedBy string, reusable func(models.ChatThread) bool) (models.ChatThread, bool, error) {
	label = strings.Join(strings.Fields(label), " ")
	tagPath, ok := labelTagPath(label)
	if !ok {
		return models.ChatThread{}, false, fmt.Errorf("%w: label is required", models.ErrChatInvalid)
	}
	if requestedBy == "" {
		return models.ChatThread{}, false, models.ErrChatNotFound
	}
	return s.mintUnlessReusable(ctx, models.ChatThread{
		StructureID: structureID, TagPath: tagPath, Label: label,
		Audience: models.ChatAudienceLeads, RequestedBy: requestedBy,
	}, reusable)
}

func (s *ChatStore) mintUnlessReusable(ctx context.Context, t models.ChatThread, reusable func(models.ChatThread) bool) (models.ChatThread, bool, error) {
	t.ID = mintID(prefixChatThread)
	existing, err := s.ResolveThread(ctx, t.StructureID, t.TagPath)
	switch {
	case err == nil && !existing.Declined() && reusable(existing):
		return existing, false, nil
	case err == nil:
		t.TagPath = t.TagPath + "-" + strings.TrimPrefix(t.ID, prefixChatThread+"-")
	case !errors.Is(err, models.ErrChatNotFound):
		return models.ChatThread{}, false, err
	}
	t.CreatedAt = s.clock()
	if err := s.store.Insert(ctx, roleChatThread, t); err != nil {
		if existing, rerr := s.ResolveThread(ctx, t.StructureID, t.TagPath); rerr == nil && !existing.Declined() && reusable(existing) {
			return existing, false, nil
		}
		return models.ChatThread{}, false, classify(err)
	}
	return t, true, nil
}

func (s *ChatStore) DeclineThread(ctx context.Context, structureID, threadID, declinedBy string) (models.ChatThread, bool, error) {
	t, err := s.threadIn(ctx, structureID, threadID)
	if err != nil {
		return models.ChatThread{}, false, err
	}
	if t.Declined() {
		return t, false, nil
	}
	if !t.Declinable() {
		return models.ChatThread{}, false, models.ErrChatNotDeclinable
	}
	at := s.clock()
	asidePath := t.TagPath + "-" + strings.TrimPrefix(t.ID, prefixChatThread+"-")
	res := s.store.Query(ctx, roleChatThread).
		Where("id = ? AND structure_id = ? AND audience = ?", threadID, structureID, string(models.ChatAudienceLeads)).
		Where(s.store.Unset(roleChatThread, "DeclinedAt")).
		Updates(map[string]any{"declined_at": stamp(at), "declined_by": declinedBy, "tag_path": asidePath})
	if res.Error != nil {
		return models.ChatThread{}, false, classify(res.Error)
	}
	if res.RowsAffected == 0 {
		return s.DeclineThread(ctx, structureID, threadID, declinedBy)
	}
	t.DeclinedAt, t.DeclinedBy, t.TagPath = &at, declinedBy, asidePath
	return t, true, nil
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

func (s *ChatStore) threadIn(ctx context.Context, structureID, threadID string) (models.ChatThread, error) {
	var t models.ChatThread
	q := s.store.Query(ctx, roleChatThread).Where("id = ? AND structure_id = ?", threadID, structureID)
	if err := s.store.Take(q, roleChatThread, &t, models.ErrChatNotFound); err != nil {
		return models.ChatThread{}, err
	}
	return t, nil
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

func (s *ChatStore) ListMessagesAfter(ctx context.Context, after models.ChatMessage) ([]models.ChatMessage, error) {
	at := stamp(after.CreatedAt)
	q := s.store.Query(ctx, roleChatMessage).
		Where("structure_id = ? AND thread_id = ?", after.StructureID, after.ThreadID).
		Where("created_at > ? OR (created_at = ? AND id > ?)", at, at, after.ID)
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
