package mwanachamacomm

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ChatPostgresStore is the Postgres implementation of ChatRepository.
// Tables: comm_chat_thread, comm_chat_message.
type ChatPostgresStore struct {
	db *sql.DB
}

// NewChatPostgresStore constructs a store over the given pool. db must be
// the SAME *sql.DB the gateway uses for its own custody store when this is
// wired for moderation's ActWriter — see CLAUDE.md.
func NewChatPostgresStore(db *sql.DB) *ChatPostgresStore { return &ChatPostgresStore{db: db} }

// MintThread returns the existing thread for a (chapter, tag_path) or creates
// one. The ON CONFLICT ... DO UPDATE keeps the row returnable whether it was
// just inserted or already present, mirroring the in-memory get-or-create.
func (s *ChatPostgresStore) MintThread(ctx context.Context, chapterID, tagPath string) (ChatThread, error) {
	const q = `INSERT INTO comm_chat_thread (chapter_id, tag_path)
	           VALUES ($1, $2)
	           ON CONFLICT (chapter_id, tag_path)
	           DO UPDATE SET tag_path = EXCLUDED.tag_path
	           RETURNING id, chapter_id, tag_path, created_at`
	var t ChatThread
	err := s.db.QueryRowContext(ctx, q, chapterID, tagPath).
		Scan(&t.ID, &t.ChapterID, &t.TagPath, &t.CreatedAt)
	if err != nil {
		return ChatThread{}, classify(err)
	}
	return t, nil
}

// ResolveThread returns the thread for a tag_path when one exists.
func (s *ChatPostgresStore) ResolveThread(ctx context.Context, chapterID, tagPath string) (ChatThread, error) {
	const q = `SELECT id, chapter_id, tag_path, created_at FROM comm_chat_thread
	           WHERE chapter_id = $1 AND tag_path = $2`
	var t ChatThread
	err := s.db.QueryRowContext(ctx, q, chapterID, tagPath).
		Scan(&t.ID, &t.ChapterID, &t.TagPath, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ChatThread{}, ErrChatNotFound
	}
	if err != nil {
		return ChatThread{}, err
	}
	return t, nil
}

// ListThreads returns every thread in a chapter, oldest first.
func (s *ChatPostgresStore) ListThreads(ctx context.Context, chapterID string) ([]ChatThread, error) {
	const q = `SELECT id, chapter_id, tag_path, created_at FROM comm_chat_thread
	           WHERE chapter_id = $1 ORDER BY created_at, id`
	rows, err := s.db.QueryContext(ctx, q, chapterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChatThread{}
	for rows.Next() {
		var t ChatThread
		if err := rows.Scan(&t.ID, &t.ChapterID, &t.TagPath, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Post stores a message; empty ThreadID lands as NULL.
func (s *ChatPostgresStore) Post(ctx context.Context, m ChatMessage) (ChatMessage, error) {
	// created_at is now(), never a caller-supplied value: no production
	// caller sets it, and a message must not be able to write its own place
	// in the room's ordering.
	const q = `INSERT INTO comm_chat_message (id, chapter_id, thread_id, author_id, body, created_at)
	           VALUES (COALESCE(NULLIF($1,''), 'msg-' || nextval('comm_chat_message_seq')),
	                   $2, $3, $4, $5, now())
	           RETURNING id, chapter_id, thread_id, author_id, body, created_at`
	var (
		out    ChatMessage
		thread sql.NullString
	)
	err := s.db.QueryRowContext(ctx, q,
		m.ID, m.ChapterID, nullStr(m.ThreadID), m.AuthorID, m.Body,
	).Scan(&out.ID, &out.ChapterID, &thread, &out.AuthorID, &out.Body, &out.CreatedAt)
	if err != nil {
		return ChatMessage{}, classify(err)
	}
	out.ThreadID = strOf(thread)
	return out, nil
}

// GetMessage returns one message by id.
func (s *ChatPostgresStore) GetMessage(ctx context.Context, id string) (ChatMessage, error) {
	const q = `SELECT id, chapter_id, thread_id, author_id, body, created_at
	           FROM comm_chat_message WHERE id = $1`
	var (
		m      ChatMessage
		thread sql.NullString
	)
	err := s.db.QueryRowContext(ctx, q, id).
		Scan(&m.ID, &m.ChapterID, &thread, &m.AuthorID, &m.Body, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ChatMessage{}, ErrChatNotFound
	}
	if err != nil {
		return ChatMessage{}, err
	}
	m.ThreadID = strOf(thread)
	return m, nil
}

// ListMessages returns messages in a chapter room; when threadID is set, only
// that thread's messages.
func (s *ChatPostgresStore) ListMessages(ctx context.Context, chapterID, threadID string) ([]ChatMessage, error) {
	const q = `SELECT id, chapter_id, thread_id, author_id, body, created_at
	           FROM comm_chat_message
	           WHERE chapter_id = $1 AND ($2 = '' OR thread_id = $2)
	           ORDER BY created_at, id`
	rows, err := s.db.QueryContext(ctx, q, chapterID, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChatMessage{}
	for rows.Next() {
		var (
			m      ChatMessage
			thread sql.NullString
		)
		if err := rows.Scan(&m.ID, &m.ChapterID, &thread, &m.AuthorID, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.ThreadID = strOf(thread)
		out = append(out, m)
	}
	return out, rows.Err()
}

var _ ChatRepository = (*ChatPostgresStore)(nil)

// chatRoomFilter is the placeholder the caller's predicate replaces on BOTH
// sides of the join in Activity's query — a filter applied to one and not the
// other would count one chapter's threads against another chapter's messages.
const chatRoomFilter = "/*room*/"

const chatActivitySQL = `
	SELECT COALESCE(t.chapter_id, m.chapter_id) AS chapter_id,
	       COALESCE(t.threads, 0),
	       COALESCE(m.messages, 0),
	       m.last_post_at
	  FROM (SELECT chapter_id, count(*) AS threads
	          FROM comm_chat_thread /*room*/ GROUP BY chapter_id) t
	  FULL OUTER JOIN
	       (SELECT chapter_id, count(*) AS messages, max(created_at) AS last_post_at
	          FROM comm_chat_message /*room*/ GROUP BY chapter_id) m
	    ON m.chapter_id = t.chapter_id`

// Activity returns one page of the chat activity summary plus the totals over
// the whole filtered set. See ChatActivityReader for what it deliberately
// does not return.
func (s *ChatPostgresStore) Activity(ctx context.Context, q ChatActivityQuery) (ChatActivityPage, error) {
	where := ""
	var args []interface{}
	if q.ChapterID != "" {
		args = append(args, q.ChapterID)
		where = "WHERE chapter_id = $1"
	}
	query := strings.ReplaceAll(chatActivitySQL, chatRoomFilter, where) + `
	  ORDER BY m.last_post_at DESC NULLS LAST, COALESCE(t.chapter_id, m.chapter_id)`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return ChatActivityPage{}, err
	}
	defer rows.Close()

	page := ChatActivityPage{Rows: []ChatActivityRow{}}
	var latest *time.Time
	i := 0
	for rows.Next() {
		var (
			r    ChatActivityRow
			last sql.NullTime
		)
		if err := rows.Scan(&r.ChapterID, &r.Threads, &r.Messages, &last); err != nil {
			return ChatActivityPage{}, err
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
		return ChatActivityPage{}, err
	}
	page.LastPostAt = latest
	return page, nil
}
