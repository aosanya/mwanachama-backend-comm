package routes_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestASubjectThreadReadByIdAnswersWhatItIsAboutBehindTheStandingFence(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseStanding(standingOf(formStandings()))
	c := func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }
	private := openSubjectThread(t, c, "doc", "d-1", "actor-a", "private")

	got := call(t, srv, http.MethodGet, "/subject-threads/"+private.ID, "actor-a", nil)
	if got.code != http.StatusOK {
		t.Fatalf("the owner reading their thread = %d: %s", got.code, got.body)
	}
	var thread struct {
		SubjectKind string `json:"subject_kind"`
		SubjectID   string `json:"subject_id"`
		Audience    string `json:"audience"`
	}
	got.decode(t, &thread)
	if thread.SubjectKind != "doc" || thread.SubjectID != "d-1" || thread.Audience != "private" {
		t.Fatalf("thread = %+v, want doc/d-1 private", thread)
	}

	invented := call(t, srv, http.MethodGet, "/subject-threads/sthread-never", "actor-a", nil)
	if invented.code != http.StatusNotFound {
		t.Fatalf("an invented thread = %d, want 404: %s", invented.code, invented.body)
	}
	for _, caller := range []string{"actor-b", "actor-c", "stranger", ""} {
		miss := call(t, srv, http.MethodGet, "/subject-threads/"+private.ID, caller, nil)
		if miss.code != http.StatusNotFound || miss.message(t) != invented.message(t) {
			t.Fatalf("%q reading another's private thread = %d %q, want the invented thread's 404 %q",
				caller, miss.code, miss.message(t), invented.message(t))
		}
	}
}

func TestAChatMessageReadByIdNamesItsThreadAndStaysReadableToItsAuthorOnceRemoved(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a", "actor-b"}, "s-2": {"actor-c"}}))
	threadID := seedChatThread(t, m, "s-1", "general")
	ctx := context.Background()
	if _, err := m.ChatStore.OpenThread(ctx, "s-1", threadID); err != nil {
		t.Fatalf("open the thread: %v", err)
	}
	posted := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+threadID+"/messages", "actor-a", map[string]string{"body": "hello"})
	if posted.code != http.StatusCreated {
		t.Fatalf("post = %d: %s", posted.code, posted.body)
	}
	var msg models.ChatMessage
	posted.decode(t, &msg)
	path := "/chat/messages/" + msg.ID

	for _, caller := range []string{"actor-a", "actor-b"} {
		got := call(t, srv, http.MethodGet, path, caller, nil)
		if got.code != http.StatusOK {
			t.Fatalf("%s reading a standing message = %d: %s", caller, got.code, got.body)
		}
		var read models.ChatMessage
		got.decode(t, &read)
		if read.StructureID != "s-1" || read.ThreadID != threadID {
			t.Fatalf("%s read %+v, want s-1/%s", caller, read, threadID)
		}
	}

	invented := call(t, srv, http.MethodGet, "/chat/messages/chmsg-never", "actor-a", nil)
	if invented.code != http.StatusNotFound {
		t.Fatalf("an invented message = %d, want 404: %s", invented.code, invented.body)
	}
	for _, caller := range []string{"actor-c", "outsider-9", ""} {
		miss := call(t, srv, http.MethodGet, path, caller, nil)
		if miss.code != http.StatusNotFound || miss.message(t) != invented.message(t) {
			t.Fatalf("%q outside the structure = %d %q, want %q", caller, miss.code, miss.message(t), invented.message(t))
		}
	}

	if _, err := m.CreateRemoval(ctx, models.Removal{
		MessageID: msg.ID, StructureID: "s-1", RemovedBy: "mod-1",
		ActorRoleClass: "Coordinator", Reason: models.RemovalReasonAbuse,
	}, "wall", models.Actor{ID: "mod-1", StructureID: "s-1"}); err != nil {
		t.Fatalf("remove: %v", err)
	}

	author := call(t, srv, http.MethodGet, path, "actor-a", nil)
	if author.code != http.StatusOK {
		t.Fatalf("the author reading their removed message = %d: %s", author.code, author.body)
	}
	other := call(t, srv, http.MethodGet, path, "actor-b", nil)
	if other.code != http.StatusNotFound || other.message(t) != invented.message(t) {
		t.Fatalf("another member reading a removed message = %d %q, want %q", other.code, other.message(t), invented.message(t))
	}
}
