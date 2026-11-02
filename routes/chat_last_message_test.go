package routes_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestEachListedThreadCarriesItsLastMessageAndAnEmptyThreadNone(t *testing.T) {
	srv, m := roomWithALeadAndAMember(t)
	general := seedChatThread(t, m, "s-1", "general")
	quiet := seedChatThread(t, m, "s-1", "quiet")

	postAs(t, srv, "s-1", general, "member-b", "first")
	posted := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+general+"/messages", "lead-a",
		map[string]string{"body": "second", "attachment_kind": "form", "attachment_id": "form-123"})
	if posted.code != http.StatusCreated {
		t.Fatalf("post = %d: %s", posted.code, posted.body)
	}
	var last models.ChatMessage
	posted.decode(t, &last)

	listed := listedIDs(t, srv, "s-1", "member-b")
	got := listed[general].LastMessage
	if got == nil {
		t.Fatalf("general carries no last_message")
	}
	if got.ID != last.ID || got.AuthorID != "lead-a" || got.Body != "second" || got.AttachmentKind != "form" || got.AttachmentID != "form-123" || !got.CreatedAt.Equal(last.CreatedAt) {
		t.Fatalf("general's last_message = %+v, want the second post %+v", got, last)
	}
	if listed[quiet].LastMessage != nil {
		t.Fatalf("a thread nobody posted in carries last_message %+v, want none", listed[quiet].LastMessage)
	}

	raw := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads", "member-b", nil)
	var wire []map[string]any
	raw.decode(t, &wire)
	for _, row := range wire {
		_, has := row["last_message"]
		if row["id"] == quiet && has {
			t.Fatalf("the empty thread's row carries last_message on the wire: %v", row)
		}
		if row["id"] == general && !has {
			t.Fatalf("general's row carries no last_message on the wire: %v", row)
		}
	}
}

func TestALeadsThreadsLastMessageNeverReachesAMemberWhoCannotReadIt(t *testing.T) {
	srv, m := roomWithALeadAndAMember(t)
	general := seedChatThread(t, m, "s-1", "general")
	seen := postAs(t, srv, "s-1", general, "member-b", "hello")

	private := startAs(t, srv, "s-1", "lead-a", "budget")
	hidden := postAs(t, srv, "s-1", private.ID, "lead-a", "later, and only for the leads")

	member := listedIDs(t, srv, "s-1", "member-b")
	if _, ok := member[private.ID]; ok {
		t.Fatalf("member-b lists the leads' thread %s", private.ID)
	}
	if got := member[general].LastMessage; got == nil || got.ID != seen.ID {
		t.Fatalf("member-b's general carries %+v, want their own post %s", got, seen.ID)
	}
	raw := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads", "member-b", nil)
	if strings.Contains(string(raw.body), hidden.Body) {
		t.Fatalf("member-b's list carries the leads' post on the wire: %s", raw.body)
	}

	lead := listedIDs(t, srv, "s-1", "lead-a")
	if got := lead[private.ID].LastMessage; got == nil || got.ID != hidden.ID || got.Body != hidden.Body {
		t.Fatalf("the lead's view of their own thread carries %+v, want %s", got, hidden.ID)
	}
}

func TestAMembersOwnRequestCarriesItsLastMessageToThemButNotToAnotherMember(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"lead-a", "member-b", "member-c"}}))
	m.UseLeading(leadingOf(map[string][]string{"s-1": {"lead-a"}}))

	asked := startAs(t, srv, "s-1", "member-b", "a question")
	reply := postAs(t, srv, "s-1", asked.ID, "lead-a", "an answer")

	if got := listedIDs(t, srv, "s-1", "member-b")[asked.ID].LastMessage; got == nil || got.ID != reply.ID {
		t.Fatalf("the requester sees last_message %+v, want %s", got, reply.ID)
	}
	other := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads", "member-c", nil)
	if strings.Contains(string(other.body), reply.Body) {
		t.Fatalf("another member's list carries the reply: %s", other.body)
	}
}
