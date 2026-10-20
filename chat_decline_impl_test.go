package mwanachamacomm_test

import (
	"context"
	"testing"
)

func TestADeclinedLabelIsStartedOnceAgainNotOnEveryStart(t *testing.T) {
	s := newChatStore(t)
	ctx := context.Background()
	asked, _, err := s.RequestThread(ctx, "c-1", "General", "asker", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if _, _, err := s.DeclineThread(ctx, "c-1", asked.ID, "lead"); err != nil {
		t.Fatalf("decline: %v", err)
	}
	first, err := s.StartThread(ctx, "c-1", "General")
	if err != nil {
		t.Fatalf("start 1: %v", err)
	}
	second, err := s.StartThread(ctx, "c-1", "general")
	if err != nil {
		t.Fatalf("start 2: %v", err)
	}
	if first.ID == asked.ID {
		t.Fatal("starting a declined label answered the declined thread")
	}
	if first.ID != second.ID {
		t.Fatalf("a declined label was started twice: %q and %q", first.ID, second.ID)
	}
	if first.TagPath != "/general" {
		t.Fatalf("the new thread is at %q, want /general", first.TagPath)
	}
	threads, err := s.ListThreads(ctx, "c-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("the room holds %d threads, want the declined one and one new", len(threads))
	}
}
