package mwanachamacomm

import (
	"context"
	"sort"
	"sync"
	"time"
)

// ChatMemoryStore is the in-memory implementation of ChatRepository.
type ChatMemoryStore struct {
	mu       sync.RWMutex
	threads  map[string]ChatThread
	byPath   map[string]string // chapterID + "\x00" + tagPath -> threadID
	messages map[string]ChatMessage
	ids      *IDGen
	clock    Clock
}

// NewChatMemoryStore constructs an empty ChatMemoryStore.
func NewChatMemoryStore(ids *IDGen, clock Clock) *ChatMemoryStore {
	return &ChatMemoryStore{
		threads:  map[string]ChatThread{},
		byPath:   map[string]string{},
		messages: map[string]ChatMessage{},
		ids:      ids,
		clock:    clock,
	}
}

func chatPathKey(chapterID, tagPath string) string { return chapterID + "\x00" + tagPath }

// MintThread returns the existing thread for a tag_path or creates one.
func (s *ChatMemoryStore) MintThread(_ context.Context, chapterID, tagPath string) (ChatThread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.byPath[chatPathKey(chapterID, tagPath)]; ok {
		return s.threads[id], nil
	}
	t := ChatThread{
		ID:        s.ids.New("cthread"),
		ChapterID: chapterID,
		TagPath:   tagPath,
		CreatedAt: s.clock(),
	}
	s.threads[t.ID] = t
	s.byPath[chatPathKey(chapterID, tagPath)] = t.ID
	return t, nil
}

// ResolveThread returns the thread for a tag_path when one exists.
func (s *ChatMemoryStore) ResolveThread(_ context.Context, chapterID, tagPath string) (ChatThread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byPath[chatPathKey(chapterID, tagPath)]
	if !ok {
		return ChatThread{}, ErrChatNotFound
	}
	return s.threads[id], nil
}

// ListThreads returns every thread in a chapter.
func (s *ChatMemoryStore) ListThreads(_ context.Context, chapterID string) ([]ChatThread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []ChatThread{}
	for _, t := range s.threads {
		if t.ChapterID == chapterID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return lessByTimeThenID(out[i].CreatedAt, out[j].CreatedAt, out[i].ID, out[j].ID)
	})
	return out, nil
}

// Post stores a message.
func (s *ChatMemoryStore) Post(_ context.Context, m ChatMessage) (ChatMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ID == "" {
		m.ID = s.ids.New("msg")
	} else if _, exists := s.messages[m.ID]; exists {
		// DEV-1172 · comm_chat_message.id is a primary key, so Postgres answers
		// a duplicate with a unique violation and the row already there
		// survives. A bare map assignment here would be silent destruction,
		// which makes the two backends disagree — refuse it the way Postgres
		// does.
		return ChatMessage{}, Conflict("comm_chat_message_pkey")
	}
	// DEV-1173 · stamped unconditionally: no production caller sets CreatedAt,
	// and the clock is injectable so deterministic tests are unaffected.
	m.CreatedAt = s.clock()
	s.messages[m.ID] = m
	return m, nil
}

// GetMessage returns one message by id.
func (s *ChatMemoryStore) GetMessage(_ context.Context, id string) (ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.messages[id]
	if !ok {
		return ChatMessage{}, ErrChatNotFound
	}
	return m, nil
}

// ListMessages returns messages in a chapter room, optionally filtered by
// thread.
func (s *ChatMemoryStore) ListMessages(_ context.Context, chapterID, threadID string) ([]ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []ChatMessage{}
	for _, m := range s.messages {
		if m.ChapterID != chapterID {
			continue
		}
		if threadID != "" && m.ThreadID != threadID {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return lessByTimeThenID(out[i].CreatedAt, out[j].CreatedAt, out[i].ID, out[j].ID)
	})
	return out, nil
}

// Activity is the in-memory chat activity summary. What has to match exactly
// is the answers — which chapters appear, how they are ordered, what Limit 0
// means, and that the totals describe the whole filtered set rather than the
// page — since Postgres expresses this ordering in one statement and this
// store exists so the suite can exercise the handlers without it.
func (s *ChatMemoryStore) Activity(_ context.Context, q ChatActivityQuery) (ChatActivityPage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	byChapter := map[string]*ChatActivityRow{}
	row := func(chapterID string) *ChatActivityRow {
		if r, ok := byChapter[chapterID]; ok {
			return r
		}
		r := &ChatActivityRow{ChapterID: chapterID}
		byChapter[chapterID] = r
		return r
	}
	// Threads first, so a room that has been opened and never posted in is a
	// row with a thread and no last-post time rather than no row at all.
	for _, t := range s.threads {
		if q.ChapterID != "" && t.ChapterID != q.ChapterID {
			continue
		}
		row(t.ChapterID).Threads++
	}
	for _, m := range s.messages {
		if q.ChapterID != "" && m.ChapterID != q.ChapterID {
			continue
		}
		r := row(m.ChapterID)
		r.Messages++
		if r.LastPostAt == nil || m.CreatedAt.After(*r.LastPostAt) {
			at := m.CreatedAt
			r.LastPostAt = &at
		}
	}

	rows := make([]ChatActivityRow, 0, len(byChapter))
	for _, r := range byChapter {
		rows = append(rows, *r)
	}
	// Most recently active first — rooms that have said nothing sort to the
	// bottom rather than being given a stand-in time. Ties break on chapter
	// id so the order is total and paging cannot repeat a row.
	sort.Slice(rows, func(i, j int) bool {
		li, lj := rows[i].LastPostAt, rows[j].LastPostAt
		if (li == nil) != (lj == nil) {
			return lj == nil
		}
		if li != nil && !li.Equal(*lj) {
			return li.After(*lj)
		}
		return rows[i].ChapterID < rows[j].ChapterID
	})

	page := ChatActivityPage{Rows: []ChatActivityRow{}, Total: len(rows)}
	var latest *time.Time
	for _, r := range rows {
		page.Threads += r.Threads
		page.Messages += r.Messages
		if r.LastPostAt != nil && (latest == nil || r.LastPostAt.After(*latest)) {
			at := *r.LastPostAt
			latest = &at
		}
	}
	page.LastPostAt = latest

	from := q.Offset
	if from > len(rows) {
		from = len(rows)
	}
	window := rows[from:]
	if q.Limit != nil {
		n := *q.Limit
		if n < 0 {
			n = 0
		}
		if n < len(window) {
			window = window[:n]
		}
	}
	page.Rows = append(page.Rows, window...)
	return page, nil
}

var _ ChatRepository = (*ChatMemoryStore)(nil)
