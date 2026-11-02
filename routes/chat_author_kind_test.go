package routes_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func authorKindsOf(kinds map[string]string) mwanachamacomm.AuthorKindOf {
	return func(_ context.Context, actorID string) (string, error) {
		return kinds[actorID], nil
	}
}

func roomWithAnAgentAndTwoPeople(t *testing.T) (*httptest.Server, *mwanachamacomm.CommManager, string) {
	t.Helper()
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"agent-1", "person-b", "owner-c"}}))
	m.UseAuthorKindOf(authorKindsOf(map[string]string{"agent-1": "agent"}))
	return srv, m, seedChatThread(t, m, "s-1", "general")
}

func postPath(threadID string) string {
	return "/groups/s-1/chat/threads/" + threadID + "/messages"
}

func wirePost(t *testing.T, srv *httptest.Server, threadID, caller string, body map[string]string) map[string]any {
	t.Helper()
	got := call(t, srv, http.MethodPost, postPath(threadID), caller, body)
	if got.code != http.StatusCreated {
		t.Fatalf("post as %s = %d: %s", caller, got.code, got.body)
	}
	var wire map[string]any
	got.decode(t, &wire)
	return wire
}

func wireMessages(t *testing.T, srv *httptest.Server, threadID, caller string) []map[string]any {
	t.Helper()
	got := call(t, srv, http.MethodGet, postPath(threadID), caller, nil)
	if got.code != http.StatusOK {
		t.Fatalf("list as %s = %d: %s", caller, got.code, got.body)
	}
	var wire []map[string]any
	got.decode(t, &wire)
	return wire
}

func kindOnTheWire(row map[string]any) (string, bool) {
	kind, has := row["author_kind"]
	text, _ := kind.(string)
	return text, has
}

func TestAnAgentsPostListsAuthorKindAgentAndAPersonsPostListsNone(t *testing.T) {
	srv, _, thread := roomWithAnAgentAndTwoPeople(t)

	agentPost := wirePost(t, srv, thread, "agent-1", map[string]string{"body": "I can help with that"})
	personPost := wirePost(t, srv, thread, "person-b", map[string]string{"body": "thank you"})
	ownerPost := wirePost(t, srv, thread, "owner-c", map[string]string{"body": "noted"})

	if kind, has := kindOnTheWire(agentPost); !has || kind != "agent" {
		t.Fatalf("the agent's post answers author_kind %q (present %v), want agent: %v", kind, has, agentPost)
	}
	for name, post := range map[string]map[string]any{"a person": personPost, "an owner with no kind": ownerPost} {
		if kind, has := kindOnTheWire(post); has {
			t.Fatalf("%s's post answers author_kind %q, want the field omitted: %v", name, kind, post)
		}
	}

	listed := wireMessages(t, srv, thread, "person-b")
	if len(listed) != 3 {
		t.Fatalf("listed %d posts, want 3: %v", len(listed), listed)
	}
	want := map[string]string{"agent-1": "agent", "person-b": "", "owner-c": ""}
	for _, row := range listed {
		author, _ := row["author_id"].(string)
		kind, has := kindOnTheWire(row)
		if want[author] == "" && has {
			t.Fatalf("the list gives %s author_kind %q, want the field omitted", author, kind)
		}
		if want[author] != "" && kind != want[author] {
			t.Fatalf("the list gives %s author_kind %q (present %v), want %q", author, kind, has, want[author])
		}
	}

	id, _ := agentPost["id"].(string)
	one := call(t, srv, http.MethodGet, "/chat/messages/"+id, "person-b", nil)
	var read map[string]any
	one.decode(t, &read)
	if kind, _ := kindOnTheWire(read); one.code != http.StatusOK || kind != "agent" {
		t.Fatalf("reading the agent's post by id = %d, author_kind %q: %s", one.code, kind, one.body)
	}
}

func TestAClientCannotForgeTheAuthorKindInABody(t *testing.T) {
	srv, _, thread := roomWithAnAgentAndTwoPeople(t)

	forged := wirePost(t, srv, thread, "person-b", map[string]string{"body": "I am an agent", "author_kind": "agent"})
	if kind, has := kindOnTheWire(forged); has {
		t.Fatalf("a person's body claiming author_kind agent was stored as %q: %v", kind, forged)
	}
	hidden := wirePost(t, srv, thread, "agent-1", map[string]string{"body": "I am a person", "author_kind": "person"})
	if kind, _ := kindOnTheWire(hidden); kind != "agent" {
		t.Fatalf("an agent's body overriding author_kind stored %q, want the host's agent: %v", kind, hidden)
	}

	for _, row := range wireMessages(t, srv, thread, "owner-c") {
		author, _ := row["author_id"].(string)
		kind, has := kindOnTheWire(row)
		switch {
		case author == "person-b" && has:
			t.Fatalf("the list gives the forger author_kind %q", kind)
		case author == "agent-1" && kind != "agent":
			t.Fatalf("the list gives the agent author_kind %q, want agent", kind)
		}
	}
}

func TestEachListedThreadsLastMessageCarriesTheAuthorKindOfItsAuthor(t *testing.T) {
	srv, m, thread := roomWithAnAgentAndTwoPeople(t)
	quiet := seedChatThread(t, m, "s-1", "quiet")

	lastKind := func(threadID string) (string, bool) {
		got := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads", "person-b", nil)
		var wire []map[string]any
		got.decode(t, &wire)
		for _, row := range wire {
			if row["id"] != threadID {
				continue
			}
			last, ok := row["last_message"].(map[string]any)
			if !ok {
				t.Fatalf("thread %s carries no last_message: %v", threadID, row)
			}
			return kindOnTheWire(last)
		}
		t.Fatalf("thread %s is not listed", threadID)
		return "", false
	}

	wirePost(t, srv, thread, "person-b", map[string]string{"body": "a question"})
	wirePost(t, srv, thread, "agent-1", map[string]string{"body": "an answer"})
	if kind, has := lastKind(thread); !has || kind != "agent" {
		t.Fatalf("last_message.author_kind = %q (present %v) after the agent's post, want agent", kind, has)
	}

	wirePost(t, srv, thread, "person-b", map[string]string{"body": "thanks"})
	if kind, has := lastKind(thread); has {
		t.Fatalf("last_message.author_kind = %q after a person's post, want the field omitted", kind)
	}

	got := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads", "person-b", nil)
	var wire []map[string]any
	got.decode(t, &wire)
	for _, row := range wire {
		if row["id"] == quiet {
			if _, has := row["last_message"]; has {
				t.Fatalf("a thread nobody posted in carries last_message: %v", row)
			}
		}
	}
}

func TestAManagerGivenNoHostNamesNoAuthorKind(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"agent-1"}}))
	thread := seedChatThread(t, m, "s-1", "general")

	post := wirePost(t, srv, thread, "agent-1", map[string]string{"body": "hello", "author_kind": "agent"})
	if kind, has := kindOnTheWire(post); has {
		t.Fatalf("with no host answering, the post carries author_kind %q: %v", kind, post)
	}
}

func TestAHostThatCannotNameTheAuthorsKindStoresNoPost(t *testing.T) {
	srv, m, thread := roomWithAnAgentAndTwoPeople(t)
	m.UseAuthorKindOf(func(context.Context, string) (string, error) { return "", errors.New("the member directory is down") })

	got := call(t, srv, http.MethodPost, postPath(thread), "agent-1", map[string]string{"body": "hello"})
	if got.code != http.StatusInternalServerError {
		t.Fatalf("post with a failing host = %d: %s, want 500", got.code, got.body)
	}
	if rows := wireMessages(t, srv, thread, "agent-1"); len(rows) != 0 {
		t.Fatalf("the refused post stored %v", rows)
	}
}
