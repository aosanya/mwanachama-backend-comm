package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func postAs(t *testing.T, srv *httptest.Server, structureID, threadID, caller, body string) models.ChatMessage {
	t.Helper()
	got := call(t, srv, http.MethodPost, "/groups/"+structureID+"/chat/threads/"+threadID+"/messages", caller, map[string]string{"body": body})
	if got.code != http.StatusCreated {
		t.Fatalf("post as %s = %d: %s", caller, got.code, got.body)
	}
	var msg models.ChatMessage
	got.decode(t, &msg)
	return msg
}

func TestEachListedThreadCarriesItsLatestPostAndAnEmptyThreadNone(t *testing.T) {
	srv, m := roomWithALeadAndAMember(t)
	general := seedChatThread(t, m, "s-1", "general")
	quiet := seedChatThread(t, m, "s-1", "quiet")

	postAs(t, srv, "s-1", general, "member-b", "first")
	last := postAs(t, srv, "s-1", general, "lead-a", "second")

	listed := listedIDs(t, srv, "s-1", "member-b")
	got := listed[general].LastMessageAt
	if got == nil || !got.Equal(last.CreatedAt) {
		t.Fatalf("general's last_message_at = %v, want the latest post's %v", got, last.CreatedAt)
	}
	if listed[quiet].LastMessageAt != nil {
		t.Fatalf("a thread nobody posted in carries last_message_at %v, want none", listed[quiet].LastMessageAt)
	}

	raw := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads", "member-b", nil)
	var wire []map[string]any
	raw.decode(t, &wire)
	for _, row := range wire {
		_, has := row["last_message_at"]
		if row["id"] == quiet && has {
			t.Fatalf("the empty thread's row carries last_message_at on the wire: %v", row)
		}
		if row["id"] == general && !has {
			t.Fatalf("general's row carries no last_message_at on the wire: %v", row)
		}
	}
}

func TestALeadsThreadsPostDoesNotMoveARoomForSomebodyWhoCannotReadIt(t *testing.T) {
	srv, m := roomWithALeadAndAMember(t)
	general := seedChatThread(t, m, "s-1", "general")
	seen := postAs(t, srv, "s-1", general, "member-b", "hello")

	private := startAs(t, srv, "s-1", "lead-a", "budget")
	hidden := postAs(t, srv, "s-1", private.ID, "lead-a", "later, and only for the leads")
	if !hidden.CreatedAt.After(seen.CreatedAt) {
		t.Fatalf("the leads' post %v is not after the group's %v", hidden.CreatedAt, seen.CreatedAt)
	}

	member := listedIDs(t, srv, "s-1", "member-b")
	if _, ok := member[private.ID]; ok {
		t.Fatalf("member-b lists the leads' thread %s", private.ID)
	}
	for id, v := range member {
		if v.LastMessageAt != nil && v.LastMessageAt.After(seen.CreatedAt) {
			t.Fatalf("thread %s shows member-b activity at %v, after the last post they may read", id, v.LastMessageAt)
		}
	}

	lead := listedIDs(t, srv, "s-1", "lead-a")
	if at := lead[private.ID].LastMessageAt; at == nil || !at.Equal(hidden.CreatedAt) {
		t.Fatalf("the lead's view of their own thread carries %v, want %v", at, hidden.CreatedAt)
	}
}

func TestAMembersOwnRequestCarriesItsLatestPostToThemButNotToAnotherMember(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"lead-a", "member-b", "member-c"}}))
	m.UseLeading(leadingOf(map[string][]string{"s-1": {"lead-a"}}))

	asked := startAs(t, srv, "s-1", "member-b", "a question")
	reply := postAs(t, srv, "s-1", asked.ID, "lead-a", "an answer")

	if at := listedIDs(t, srv, "s-1", "member-b")[asked.ID].LastMessageAt; at == nil || !at.Equal(reply.CreatedAt) {
		t.Fatalf("the requester sees last_message_at %v, want %v", at, reply.CreatedAt)
	}
	if _, ok := listedIDs(t, srv, "s-1", "member-c")[asked.ID]; ok {
		t.Fatalf("another member lists somebody else's request")
	}
}
