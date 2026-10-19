package routes_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func belongingOf(roster map[string][]string) mwanachamacomm.Belonging {
	return func(_ context.Context, actorID, structureID string) (bool, error) {
		for _, a := range roster[structureID] {
			if a == actorID {
				return true, nil
			}
		}
		return false, nil
	}
}

func seedChatThread(t *testing.T, m *mwanachamacomm.CommManager, structureID, tagPath string) string {
	t.Helper()
	th, err := m.MintThread(context.Background(), structureID, tagPath)
	if err != nil {
		t.Fatalf("seed a thread: %v", err)
	}
	return th.ID
}

func chatPaths(structureID, threadID string) []struct{ name, method, path string } {
	return []struct{ name, method, path string }{
		{"the threads", http.MethodGet, "/groups/" + structureID + "/chat/threads"},
		{"the posts", http.MethodGet, "/groups/" + structureID + "/chat/threads/" + threadID + "/messages"},
		{"a post", http.MethodPost, "/groups/" + structureID + "/chat/threads/" + threadID + "/messages"},
	}
}

func TestAStructureFenceAnswersAnOutsiderExactlyAsAnInventedThread(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a"}, "s-2": {"actor-b"}}))
	threadID := seedChatThread(t, m, "s-1", "general")
	otherThread := seedChatThread(t, m, "s-2", "general")
	post := map[string]string{"body": "hello"}

	for _, c := range chatPaths("s-1", threadID) {
		t.Run(c.name, func(t *testing.T) {
			var body any
			if c.method == http.MethodPost {
				body = post
			}
			outsider := call(t, srv, c.method, c.path, "outsider-9", body)
			if outsider.code != http.StatusNotFound {
				t.Fatalf("an outsider on %s = %d, want 404: %s", c.name, outsider.code, outsider.body)
			}
			probes := []reply{call(t, srv, c.method, replaceID(c.path, "s-1", "s-never"), "outsider-9", body)}
			if c.name != "the threads" {
				probes = append(probes,
					call(t, srv, c.method, replaceID(c.path, threadID, "cht-never"), "actor-a", body),
					call(t, srv, c.method, replaceID(c.path, threadID, otherThread), "actor-a", body))
			}
			for _, probe := range probes {
				if probe.code != outsider.code || probe.message(t) != outsider.message(t) {
					t.Fatalf("an outsider answers %d %q and a probe %d %q; the difference is an enumeration oracle",
						outsider.code, outsider.message(t), probe.code, probe.message(t))
				}
			}
		})
	}

	got, err := m.ListChatMessages(context.Background(), "s-1", threadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("a refused post landed: %+v", got)
	}
	other, err := m.ListChatMessages(context.Background(), "s-2", otherThread)
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatalf("a post addressed under s-1 landed on s-2's thread: %+v", other)
	}
}

func TestWithNoBelongingSuppliedNobodyReadsOrPosts(t *testing.T) {
	srv, m := newCommServer(t)
	threadID := seedChatThread(t, m, "s-1", "general")

	for _, c := range chatPaths("s-1", threadID) {
		var body any
		if c.method == http.MethodPost {
			body = map[string]string{"body": "hello"}
		}
		if got := call(t, srv, c.method, c.path, "actor-a", body); got.code != http.StatusNotFound {
			t.Errorf("%s with no belonging mounted = %d, want 404: %s", c.name, got.code, got.body)
		}
	}
}

func TestAPostsAuthorIsTheSessionNotTheBody(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a"}}))
	threadID := seedChatThread(t, m, "s-1", "general")
	path := "/groups/s-1/chat/threads/" + threadID + "/messages"

	posted := call(t, srv, http.MethodPost, path, "actor-a", map[string]string{
		"body": "hello", "author_id": "somebody-else", "id": "cht-chosen", "structure_id": "s-2",
	})
	if posted.code != http.StatusCreated {
		t.Fatalf("post = %d, want 201: %s", posted.code, posted.body)
	}
	var msg mwanachamacomm.ChatMessage
	posted.decode(t, &msg)
	if msg.AuthorID != "actor-a" {
		t.Fatalf("author = %q, want the session's actor-a", msg.AuthorID)
	}
	if msg.ID == "cht-chosen" || msg.StructureID != "s-1" || msg.ThreadID != threadID {
		t.Fatalf("the body chose server-owned fields: %+v", msg)
	}

	listed := call(t, srv, http.MethodGet, path, "actor-a", nil)
	if listed.code != http.StatusOK {
		t.Fatalf("list = %d, want 200: %s", listed.code, listed.body)
	}
	var msgs []mwanachamacomm.ChatMessage
	listed.decode(t, &msgs)
	if len(msgs) != 1 || msgs[0].AuthorID != "actor-a" || msgs[0].Body != "hello" {
		t.Fatalf("the thread reads back %+v", msgs)
	}

	threads := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads", "actor-a", nil)
	var out []struct {
		ID string `json:"id"`
	}
	threads.decode(t, &out)
	if threads.code != http.StatusOK || len(out) != 1 || out[0].ID != threadID {
		t.Fatalf("threads = %d %s", threads.code, threads.body)
	}
}

func TestAnEmptyPostIsRefused(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a"}}))
	threadID := seedChatThread(t, m, "s-1", "general")

	for _, body := range []any{map[string]string{"body": "  "}, map[string]string{}} {
		got := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+threadID+"/messages", "actor-a", body)
		if got.code != http.StatusBadRequest {
			t.Errorf("post %v = %d, want 400: %s", body, got.code, got.body)
		}
	}
}

func TestABelongingFailureIsNeitherNotFoundNorForbidden(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(func(context.Context, string, string) (bool, error) {
		return false, errors.New("the directory is down: secret-host:5432")
	})
	got := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads", "actor-a", nil)
	if got.code != http.StatusInternalServerError {
		t.Fatalf("a failing lookup = %d, want 500: %s", got.code, got.body)
	}
	if msg := got.message(t); msg != "internal error" {
		t.Fatalf("a failing lookup reported %q; the host's reason must not reach the caller", msg)
	}
}
