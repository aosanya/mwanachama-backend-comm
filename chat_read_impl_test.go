package mwanachamacomm_test

import (
	"context"
	"testing"
	"time"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

type unreadRoom struct {
	t      *testing.T
	s      *mwanachamacomm.ChatStore
	thread string
}

func newUnreadRoom(t *testing.T) unreadRoom {
	t.Helper()
	s := newChatStore(t)
	th, err := s.MintThread(context.Background(), "s-1", "/general")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	return unreadRoom{t: t, s: s, thread: th.ID}
}

func (r unreadRoom) post(author string, times int) mwanachamacomm.ChatMessage {
	r.t.Helper()
	var last mwanachamacomm.ChatMessage
	for i := 0; i < times; i++ {
		m, err := r.s.Post(context.Background(), mwanachamacomm.ChatMessage{StructureID: "s-1", ThreadID: r.thread, AuthorID: author, Body: "x"})
		if err != nil {
			r.t.Fatalf("post: %v", err)
		}
		last = m
	}
	return last
}

func (r unreadRoom) unread(reader string) int {
	r.t.Helper()
	counts, err := r.s.UnreadCounts(context.Background(), "s-1", reader)
	if err != nil {
		r.t.Fatalf("UnreadCounts: %v", err)
	}
	return counts[r.thread]
}

func TestTheUnreadCountStopsAtTheCap(t *testing.T) {
	cases := []struct{ posts, want int }{
		{0, 0},
		{1, 1},
		{mwanachamacomm.UnreadCap - 1, mwanachamacomm.UnreadCap - 1},
		{mwanachamacomm.UnreadCap, mwanachamacomm.UnreadCap},
		{mwanachamacomm.UnreadCap + 1, mwanachamacomm.UnreadCap},
		{mwanachamacomm.UnreadCap * 3, mwanachamacomm.UnreadCap},
	}
	for _, c := range cases {
		r := newUnreadRoom(t)
		r.post("actor-b", c.posts)
		if got := r.unread("actor-a"); got != c.want {
			t.Fatalf("%d posts by somebody else: unread = %d, want %d", c.posts, got, c.want)
		}
	}
}

func TestTheCapIsOneHundredSoTheAppCanTellNinetyNineFromMore(t *testing.T) {
	if mwanachamacomm.UnreadCap != 100 {
		t.Fatalf("UnreadCap = %d, decision 97 says the count stops at 100", mwanachamacomm.UnreadCap)
	}
}

func TestOwnPostsDoNotSpendTheCap(t *testing.T) {
	r := newUnreadRoom(t)
	r.post("actor-b", 60)
	r.post("actor-a", 80)
	r.post("actor-b", 60)
	if got := r.unread("actor-a"); got != mwanachamacomm.UnreadCap {
		t.Fatalf("120 posts by somebody else among 80 own: unread = %d, want %d", got, mwanachamacomm.UnreadCap)
	}
	if got := r.unread("actor-b"); got != 80 {
		t.Fatalf("80 posts by somebody else: unread = %d, want 80", got)
	}
}

func TestTheCapCountsOnlyWhatIsAfterTheReadPoint(t *testing.T) {
	r := newUnreadRoom(t)
	r.post("actor-b", 150)
	last := r.post("actor-b", 1)
	if _, err := r.s.MarkRead(context.Background(), "s-1", r.thread, "actor-a", last.CreatedAt); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if got := r.unread("actor-a"); got != 0 {
		t.Fatalf("after reading everything: unread = %d, want 0", got)
	}
	r.post("actor-b", 130)
	if got := r.unread("actor-a"); got != mwanachamacomm.UnreadCap {
		t.Fatalf("130 after the read point: unread = %d, want %d", got, mwanachamacomm.UnreadCap)
	}
	r2 := newUnreadRoom(t)
	r2.post("actor-b", 150)
	mid := r2.post("actor-b", 1)
	r2.post("actor-b", 7)
	if _, err := r2.s.MarkRead(context.Background(), "s-1", r2.thread, "actor-a", mid.CreatedAt); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if got := r2.unread("actor-a"); got != 7 {
		t.Fatalf("7 after the read point: unread = %d, want 7", got)
	}
}

func TestTheCapIsPerThreadAndLeavesOtherRoomsAlone(t *testing.T) {
	ctx := context.Background()
	r := newUnreadRoom(t)
	other, err := r.s.MintThread(ctx, "s-1", "/events")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	elsewhere, err := r.s.MintThread(ctx, "s-2", "/general")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	r.post("actor-b", 130)
	for i := 0; i < 3; i++ {
		if _, err := r.s.Post(ctx, mwanachamacomm.ChatMessage{StructureID: "s-1", ThreadID: other.ID, AuthorID: "actor-b", Body: "x"}); err != nil {
			t.Fatalf("post: %v", err)
		}
		if _, err := r.s.Post(ctx, mwanachamacomm.ChatMessage{StructureID: "s-2", ThreadID: elsewhere.ID, AuthorID: "actor-b", Body: "x"}); err != nil {
			t.Fatalf("post: %v", err)
		}
	}
	counts, err := r.s.UnreadCounts(ctx, "s-1", "actor-a")
	if err != nil {
		t.Fatalf("UnreadCounts: %v", err)
	}
	if len(counts) != 2 || counts[r.thread] != mwanachamacomm.UnreadCap || counts[other.ID] != 3 {
		t.Fatalf("counts = %v, want %d on general, 3 on events and nothing from the other room", counts, mwanachamacomm.UnreadCap)
	}
}

func TestLatestPostsNamesOnlyThreadsThatHaveOne(t *testing.T) {
	ctx := context.Background()
	r := newUnreadRoom(t)
	quiet, err := r.s.MintThread(ctx, "s-1", "/quiet")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	r.post("actor-b", 3)
	last := r.post("actor-a", 1)
	if _, err := r.s.Post(ctx, mwanachamacomm.ChatMessage{StructureID: "s-1", AuthorID: "actor-b", Body: "room level"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if _, err := r.s.Post(ctx, mwanachamacomm.ChatMessage{StructureID: "s-2", AuthorID: "actor-b", Body: "other room"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	latest, err := r.s.LatestPosts(ctx, "s-1")
	if err != nil {
		t.Fatalf("LatestPosts: %v", err)
	}
	if _, ok := latest[quiet.ID]; ok {
		t.Fatalf("latest = %v, a thread nobody posted in must be absent", latest)
	}
	got, ok := latest[r.thread]
	if !ok || len(latest) != 1 {
		t.Fatalf("latest = %v, want only the general thread", latest)
	}
	if !got.Equal(last.CreatedAt.UTC().Truncate(time.Microsecond)) && !got.Equal(last.CreatedAt.UTC()) {
		t.Fatalf("latest = %v, want the newest post %v, whoever wrote it", got, last.CreatedAt)
	}
}
