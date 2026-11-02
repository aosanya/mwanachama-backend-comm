package routes_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func standingDailyBody(thread string) map[string]any {
	return map[string]any{"thread_id": thread, "body": "weekly", "rule": "FREQ=DAILY", "zone": "Africa/Nairobi", "start": "2026-01-05T06:00:00Z"}
}

func TestAStandingPostByAnAgentCarriesAuthorKindAgentAndAPersonsCarriesNone(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"agent-1", "person-b"}}))
	m.UseLeading(leadingOf(map[string][]string{"s-1": {"agent-1", "person-b"}}))
	m.UseAuthorKindOf(authorKindsOf(map[string]string{"agent-1": "agent"}))
	thread := seedChatThread(t, m, "s-1", "general")
	for _, author := range []string{"agent-1", "person-b"} {
		created := call(t, srv, http.MethodPost, "/groups/s-1/chat/standing", author, standingDailyBody(thread))
		if created.code != http.StatusCreated {
			t.Fatalf("create as %s = %d %s", author, created.code, created.body)
		}
	}

	if _, err := m.SendDueStandingMessages(context.Background(), time.Date(2026, 1, 5, 7, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("send: %v", err)
	}

	kinds := map[string]bool{}
	for _, row := range wireMessages(t, srv, thread, "person-b") {
		kind, has := kindOnTheWire(row)
		kinds[kind+"|"+boolWord(has)] = true
	}
	if !kinds["agent|has"] || !kinds["|none"] || len(kinds) != 2 {
		t.Fatalf("standing posts carry %v, want one agent post with author_kind and one person post without", kinds)
	}
}

func boolWord(has bool) string {
	if has {
		return "has"
	}
	return "none"
}

func TestAStandingPostWhoseHostCannotNameTheKindIsNotClaimedAndGoesOutOnTheNextTick(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"agent-1"}}))
	m.UseLeading(leadingOf(map[string][]string{"s-1": {"agent-1"}}))
	down := true
	m.UseAuthorKindOf(func(context.Context, string) (string, error) {
		if down {
			return "", context.DeadlineExceeded
		}
		return "agent", nil
	})
	thread := seedChatThread(t, m, "s-1", "general")
	if created := call(t, srv, http.MethodPost, "/groups/s-1/chat/standing", "agent-1", standingDailyBody(thread)); created.code != http.StatusCreated {
		t.Fatalf("create = %d %s", created.code, created.body)
	}
	due := time.Date(2026, 1, 5, 7, 0, 0, 0, time.UTC)

	if _, err := m.SendDueStandingMessages(context.Background(), due); err == nil {
		t.Fatal("a failing host left the send without an error")
	}
	if rows := wireMessages(t, srv, thread, "agent-1"); len(rows) != 0 {
		t.Fatalf("the failed tick stored %v", rows)
	}

	down = false
	if sent, err := m.SendDueStandingMessages(context.Background(), due); err != nil || len(sent) != 1 {
		t.Fatalf("the next tick sent %d with %v, want the post the failed tick left", len(sent), err)
	}
}

func TestAHostKindThatBreaksTheSubjectKindPatternIsAnInternalErrorAndStoresNothing(t *testing.T) {
	odd := map[string]string{
		"spaces and capitals": "Agent Bot!",
		"too long":            strings.Repeat("a", 5000),
		"a newline":           "agent\nx",
	}
	for name, kind := range odd {
		t.Run(name, func(t *testing.T) {
			srv, m, thread := roomWithAnAgentAndTwoPeople(t)
			m.UseAuthorKindOf(func(context.Context, string) (string, error) { return kind, nil })

			got := call(t, srv, http.MethodPost, postPath(thread), "agent-1", map[string]string{"body": "hello"})
			if got.code != http.StatusInternalServerError {
				t.Fatalf("post = %d %.120s, want 500", got.code, got.body)
			}
			text := string(got.body)
			if strings.Contains(text, "author_kind") || strings.Contains(text, "subject_kind") || strings.Contains(text, "Agent Bot") {
				t.Fatalf("the 500 names the host's mistake: %.200s", text)
			}
			if rows := wireMessages(t, srv, thread, "agent-1"); len(rows) != 0 {
				t.Fatalf("the refused post stored %v", rows)
			}
		})
	}
}
