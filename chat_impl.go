// chat_impl.go — GORM-backed ChatRepository implementation. Was
// store_postgres_chat.go + store_memory_chat.go: one store now, run against
// Postgres in production and sqlite in tests (see the root package's
// *_test.go), mirroring mwanachama-backend-actor's storage swap.
package mwanachamacomm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

// ChatStore is the GORM implementation of [models.ChatRepository].
type ChatStore struct {
	db     *gorm.DB
	tables TableNames
	clock  Clock
}

// NewChatStore constructs a ChatStore backed by db, reading and writing the
// tables named by t (see [DefaultTableNames]). Callers must run [Migrate]
// against the same db and t before use. clock defaults to [SystemClock]
// when nil. Returns an error if db is nil.
func NewChatStore(db *gorm.DB, t TableNames, clock Clock) (*ChatStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewChatStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	return &ChatStore{db: db, tables: t, clock: clock}, nil
}

func (s *ChatStore) MintThread(ctx context.Context, chapterID, tagPath string) (models.ChatThread, error) {
	if t, err := s.ResolveThread(ctx, chapterID, tagPath); err == nil {
		return t, nil
	} else if !errors.Is(err, models.ErrChatNotFound) {
		return models.ChatThread{}, err
	}
	row := gormstore.ChatThreadToRow(models.ChatThread{StructureID: chapterID, TagPath: tagPath, CreatedAt: s.clock()})
	if err := s.db.WithContext(ctx).Table(s.tables.ChatThreads).Create(&row).Error; err != nil {
		if t, rerr := s.ResolveThread(ctx, chapterID, tagPath); rerr == nil {
			return t, nil
		}
		return models.ChatThread{}, classify(err)
	}
	return gormstore.ChatThreadFromRow(row), nil
}

// ResolveThread returns the thread for a tag_path when one exists.
func (s *ChatStore) ResolveThread(ctx context.Context, chapterID, tagPath string) (models.ChatThread, error) {
	var row gormstore.ChatThreadRow
	err := s.db.WithContext(ctx).Table(s.tables.ChatThreads).
		Where("chapter_id = ? AND tag_path = ?", chapterID, tagPath).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ChatThread{}, models.ErrChatNotFound
	}
	if err != nil {
		return models.ChatThread{}, err
	}
	return gormstore.ChatThreadFromRow(row), nil
}

func (s *ChatStore) ListThreads(ctx context.Context, chapterID string) ([]models.ChatThread, error) {
	var rows []gormstore.ChatThreadRow
	err := s.db.WithContext(ctx).Table(s.tables.ChatThreads).
		Where("chapter_id = ?", chapterID).Order("created_at, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]models.ChatThread, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.ChatThreadFromRow(r))
	}
	return out, nil
}

// Post stores a message. created_at is always the store's own clock, never
// a caller-supplied value: a message must not be able to write its own
// place in the room's ordering.
func (s *ChatStore) Post(ctx context.Context, m models.ChatMessage) (models.ChatMessage, error) {
	m.CreatedAt = s.clock()
	row := gormstore.ChatMessageToRow(m)
	if err := s.db.WithContext(ctx).Table(s.tables.ChatMessages).Create(&row).Error; err != nil {
		return models.ChatMessage{}, classify(err)
	}
	return gormstore.ChatMessageFromRow(row), nil
}

// GetMessage returns one message by id.
func (s *ChatStore) GetMessage(ctx context.Context, id string) (models.ChatMessage, error) {
	var row gormstore.ChatMessageRow
	err := s.db.WithContext(ctx).Table(s.tables.ChatMessages).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ChatMessage{}, models.ErrChatNotFound
	}
	if err != nil {
		return models.ChatMessage{}, err
	}
	return gormstore.ChatMessageFromRow(row), nil
}

func (s *ChatStore) ListMessages(ctx context.Context, chapterID, threadID string) ([]models.ChatMessage, error) {
	q := s.db.WithContext(ctx).Table(s.tables.ChatMessages).Where("chapter_id = ?", chapterID)
	if threadID != "" {
		q = q.Where("thread_id = ?", threadID)
	}
	var rows []gormstore.ChatMessageRow
	if err := q.Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]models.ChatMessage, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.ChatMessageFromRow(r))
	}
	return out, nil
}

var _ models.ChatRepository = (*ChatStore)(nil)

const chatRoomFilter = "/*room*/"

// chatActivitySQL is built against s.tables at call time (the table names
// are configurable — see TableNames — so this can't be a package-level
// const the way the old hard-coded-table-name version was).
func chatActivitySQL(threads, messages string) string {
	return `
	SELECT COALESCE(t.chapter_id, m.chapter_id) AS chapter_id,
	       COALESCE(t.threads, 0),
	       COALESCE(m.messages, 0),
	       m.last_post_at
	  FROM (SELECT chapter_id, count(*) AS threads
	          FROM ` + threads + ` ` + chatRoomFilter + ` GROUP BY chapter_id) t
	  FULL OUTER JOIN
	       (SELECT chapter_id, count(*) AS messages, max(created_at) AS last_post_at
	          FROM ` + messages + ` ` + chatRoomFilter + ` GROUP BY chapter_id) m
	    ON m.chapter_id = t.chapter_id`
}

// Activity returns one page of the chat activity summary plus the totals
// over the whole filtered set. See [models.ChatActivityReader] for what it
// deliberately does not return.
func (s *ChatStore) Activity(ctx context.Context, q models.ChatActivityQuery) (models.ChatActivityPage, error) {
	where := ""
	var args []any
	if q.StructureID != "" {
		// chatRoomFilter is substituted on BOTH sides of the join below, so
		// the "?" placeholder it introduces appears twice in the finished
		// query — bind the same value twice to match, positionally (unlike
		// Postgres's $1, a plain "?" placeholder can't be reused by index).
		args = append(args, q.StructureID, q.StructureID)
		where = "WHERE chapter_id = ?"
	}
	// (m.last_post_at IS NULL) ASC sorts populated rows before NULL ones —
	// the portable equivalent of Postgres's "DESC NULLS LAST", since this
	// query also runs against sqlite in this package's fast tests.
	query := strings.ReplaceAll(chatActivitySQL(s.tables.ChatThreads, s.tables.ChatMessages), chatRoomFilter, where) + `
	  ORDER BY (m.last_post_at IS NULL) ASC, m.last_post_at DESC, COALESCE(t.chapter_id, m.chapter_id)`

	rows, err := s.db.WithContext(ctx).Raw(query, args...).Rows()
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

		// The window is applied while walking rather than in SQL, so the
		// totals above stay totals rather than describing the page.
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
