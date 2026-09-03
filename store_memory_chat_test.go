package mwanachamacomm

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// monotonicClock returns a clock that ticks forward one second per call — so
// created-at timestamps compare deterministically without racing wall time.
func monotonicClock(start time.Time) Clock {
	var n int64
	return func() time.Time {
		i := atomic.AddInt64(&n, 1)
		return start.Add(time.Duration(i) * time.Second)
	}
}

func TestChatMintThreadIsIdempotent(t *testing.T) {
	s := NewChatMemoryStore(NewIDGen(), monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	ctx := context.Background()

	th1, err := s.MintThread(ctx, "c-1", "/announcements/first")
	if err != nil {
		t.Fatalf("mint 1: %v", err)
	}
	th2, err := s.MintThread(ctx, "c-1", "/announcements/first")
	if err != nil {
		t.Fatalf("mint 2: %v", err)
	}
	if th1.ID != th2.ID {
		t.Fatalf("expected idempotent mint to return same thread, got %q and %q", th1.ID, th2.ID)
	}

	// A different chapter with the same tag_path must NOT share the thread —
	// the pathKey composes chapter + tag_path.
	th3, err := s.MintThread(ctx, "c-2", "/announcements/first")
	if err != nil {
		t.Fatalf("mint other chapter: %v", err)
	}
	if th3.ID == th1.ID {
		t.Fatalf("different chapters must not share thread id, got %q", th3.ID)
	}
}

func TestChatResolveThread(t *testing.T) {
	s := NewChatMemoryStore(NewIDGen(), monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	ctx := context.Background()

	if _, err := s.ResolveThread(ctx, "c-1", "/nope"); !errors.Is(err, ErrChatNotFound) {
		t.Fatalf("expected ErrChatNotFound before mint, got %v", err)
	}
	th, _ := s.MintThread(ctx, "c-1", "/known")
	got, err := s.ResolveThread(ctx, "c-1", "/known")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.ID != th.ID {
		t.Fatalf("resolve returned wrong thread: %q vs %q", got.ID, th.ID)
	}
}

func TestChatListThreadsOrderedAndScoped(t *testing.T) {
	s := NewChatMemoryStore(NewIDGen(), monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	ctx := context.Background()

	a, _ := s.MintThread(ctx, "c-1", "/a")
	b, _ := s.MintThread(ctx, "c-1", "/b")
	_, _ = s.MintThread(ctx, "other", "/a") // must not leak into c-1

	list, err := s.ListThreads(ctx, "c-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 threads at c-1, got %d", len(list))
	}
	// Threads sort oldest-first by CreatedAt; the monotonic clock guarantees
	// a strict order.
	if list[0].ID != a.ID || list[1].ID != b.ID {
		t.Fatalf("expected oldest-first ordering [%q,%q], got [%q,%q]",
			a.ID, b.ID, list[0].ID, list[1].ID)
	}
}

func TestChatPostAndListMessages(t *testing.T) {
	s := NewChatMemoryStore(NewIDGen(), monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	ctx := context.Background()

	th, _ := s.MintThread(ctx, "c-1", "/x")
	_, _ = s.Post(ctx, ChatMessage{ChapterID: "c-1", ThreadID: th.ID, AuthorID: "m-1", Body: "one"})
	_, _ = s.Post(ctx, ChatMessage{ChapterID: "c-1", AuthorID: "m-2", Body: "off-thread"})
	_, _ = s.Post(ctx, ChatMessage{ChapterID: "c-1", ThreadID: th.ID, AuthorID: "m-3", Body: "two"})
	// Message in a different chapter must not appear.
	_, _ = s.Post(ctx, ChatMessage{ChapterID: "other", ThreadID: th.ID, AuthorID: "m-9", Body: "leak"})

	all, err := s.ListMessages(ctx, "c-1", "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 messages at c-1, got %d", len(all))
	}
	// Oldest-first ordering.
	bodies := []string{all[0].Body, all[1].Body, all[2].Body}
	want := []string{"one", "off-thread", "two"}
	for i, w := range want {
		if bodies[i] != w {
			t.Fatalf("message[%d] want %q got %q (all=%v)", i, w, bodies[i], bodies)
		}
	}

	// Thread-filtered listing returns only that thread's messages.
	byThread, err := s.ListMessages(ctx, "c-1", th.ID)
	if err != nil {
		t.Fatalf("list thread: %v", err)
	}
	if len(byThread) != 2 {
		t.Fatalf("expected 2 in-thread messages, got %d", len(byThread))
	}
	if byThread[0].Body != "one" || byThread[1].Body != "two" {
		t.Fatalf("wrong in-thread bodies: %+v", byThread)
	}
}
