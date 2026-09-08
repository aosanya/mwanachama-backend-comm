package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func newChatStore(t *testing.T) *mwanachamacomm.ChatStore {
	t.Helper()
	db, tables := newTestDB(t)
	s, err := mwanachamacomm.NewChatStore(db, tables, monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	return s
}

func TestChatMintThreadIsIdempotent(t *testing.T) {
	s := newChatStore(t)
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
		t.Fatalf("expected idempotent mint, got %q and %q", th1.ID, th2.ID)
	}
}

func TestChatResolveThreadNotFound(t *testing.T) {
	s := newChatStore(t)
	if _, err := s.ResolveThread(context.Background(), "c-1", "/nope"); !errors.Is(err, mwanachamacomm.ErrChatNotFound) {
		t.Fatalf("expected ErrChatNotFound, got %v", err)
	}
}

func TestChatPostAndListMessages(t *testing.T) {
	s := newChatStore(t)
	ctx := context.Background()
	th, err := s.MintThread(ctx, "c-1", "/general")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if _, err := s.Post(ctx, mwanachamacomm.ChatMessage{StructureID: "c-1", ThreadID: th.ID, AuthorID: "a-1", Body: "hi"}); err != nil {
		t.Fatalf("post threaded: %v", err)
	}
	if _, err := s.Post(ctx, mwanachamacomm.ChatMessage{StructureID: "c-1", AuthorID: "a-1", Body: "room level"}); err != nil {
		t.Fatalf("post room-level: %v", err)
	}
	all, err := s.ListMessages(ctx, "c-1", "")
	if err != nil || len(all) != 2 {
		t.Fatalf("ListMessages(all) = %+v, err %v, want 2", all, err)
	}
	threaded, err := s.ListMessages(ctx, "c-1", th.ID)
	if err != nil || len(threaded) != 1 {
		t.Fatalf("ListMessages(thread) = %+v, err %v, want 1", threaded, err)
	}
	if threaded[0].ThreadID != th.ID {
		t.Fatalf("threaded message ThreadID = %q, want %q", threaded[0].ThreadID, th.ID)
	}

	got, err := s.GetMessage(ctx, all[0].ID)
	if err != nil || got.ID != all[0].ID {
		t.Fatalf("GetMessage = %+v, err %v", got, err)
	}
	if _, err := s.GetMessage(ctx, "no-such-message"); !errors.Is(err, mwanachamacomm.ErrChatNotFound) {
		t.Fatalf("expected ErrChatNotFound, got %v", err)
	}
}

func TestChatActivityCountsAndOrdering(t *testing.T) {
	s := newChatStore(t)
	ctx := context.Background()

	// c-1 has a thread and a post; c-2 has a thread and never posted (a real
	// state, not zero-and-absent); c-3 has posts and no thread.
	if _, err := s.MintThread(ctx, "c-1", "/a"); err != nil {
		t.Fatalf("mint c-1: %v", err)
	}
	if _, err := s.Post(ctx, mwanachamacomm.ChatMessage{StructureID: "c-1", AuthorID: "a", Body: "hi"}); err != nil {
		t.Fatalf("post c-1: %v", err)
	}
	if _, err := s.MintThread(ctx, "c-2", "/b"); err != nil {
		t.Fatalf("mint c-2: %v", err)
	}
	if _, err := s.Post(ctx, mwanachamacomm.ChatMessage{StructureID: "c-3", AuthorID: "a", Body: "hi"}); err != nil {
		t.Fatalf("post c-3: %v", err)
	}

	page, err := s.Activity(ctx, mwanachamacomm.ChatActivityQuery{})
	if err != nil {
		t.Fatalf("Activity: %v", err)
	}
	if page.Total != 3 {
		t.Fatalf("Total = %d, want 3", page.Total)
	}
	if page.Threads != 2 || page.Messages != 2 {
		t.Fatalf("Threads/Messages = %d/%d, want 2/2", page.Threads, page.Messages)
	}
	// c-1 posted most recently (Post ran after c-2's thread mint), so it
	// sorts first; c-2 has no post at all and sorts last.
	if page.Rows[len(page.Rows)-1].StructureID != "c-2" {
		t.Fatalf("expected c-2 (no posts) last, got rows %+v", page.Rows)
	}
	for _, r := range page.Rows {
		if r.StructureID == "c-2" && r.LastPostAt != nil {
			t.Fatalf("c-2 has no posts, LastPostAt should be nil: %+v", r)
		}
	}

	filtered, err := s.Activity(ctx, mwanachamacomm.ChatActivityQuery{StructureID: "c-1"})
	if err != nil {
		t.Fatalf("Activity filtered: %v", err)
	}
	if filtered.Total != 1 || filtered.Rows[0].StructureID != "c-1" {
		t.Fatalf("filtered Activity = %+v, want only c-1", filtered)
	}

	zero := 0
	empty, err := s.Activity(ctx, mwanachamacomm.ChatActivityQuery{Limit: &zero})
	if err != nil {
		t.Fatalf("Activity limit 0: %v", err)
	}
	if len(empty.Rows) != 0 || empty.Total != 3 {
		t.Fatalf("Activity limit 0 = %+v, want no rows but Total 3", empty)
	}
}
