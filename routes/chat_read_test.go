package routes_test

import (
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

type roomFixture struct {
	t        *testing.T
	call     func(method, path, caller string, body any) reply
	threadID string
	other    string
}

func newRoomFixture(t *testing.T) roomFixture {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a", "actor-b"}, "s-2": {"actor-a"}}))
	return roomFixture{
		t: t,
		call: func(method, path, caller string, body any) reply {
			return call(t, srv, method, path, caller, body)
		},
		threadID: seedChatThread(t, m, "s-1", "general"),
		other:    seedChatThread(t, m, "s-1", "events"),
	}
}

func (f roomFixture) post(threadID, author, body string) mwanachamacomm.ChatMessage {
	f.t.Helper()
	got := f.call(http.MethodPost, "/groups/s-1/chat/threads/"+threadID+"/messages", author, map[string]string{"body": body})
	if got.code != http.StatusCreated {
		f.t.Fatalf("post = %d: %s", got.code, got.body)
	}
	var msg mwanachamacomm.ChatMessage
	got.decode(f.t, &msg)
	return msg
}

func (f roomFixture) unread(caller string) map[string]int {
	f.t.Helper()
	got := f.call(http.MethodGet, "/groups/s-1/chat/threads", caller, nil)
	if got.code != http.StatusOK {
		f.t.Fatalf("list threads = %d: %s", got.code, got.body)
	}
	var views []mwanachamacomm.ChatThreadView
	got.decode(f.t, &views)
	out := map[string]int{}
	for _, v := range views {
		out[v.ID] = v.Unread
	}
	return out
}

func (f roomFixture) markRead(threadID, caller string, body any) reply {
	f.t.Helper()
	return f.call(http.MethodPost, "/groups/s-1/chat/threads/"+threadID+"/read", caller, body)
}

func TestARoomListCountsWhatSomebodyElsePostedSinceTheReadPoint(t *testing.T) {
	f := newRoomFixture(t)
	f.post(f.threadID, "actor-b", "one")
	f.post(f.threadID, "actor-b", "two")
	f.post(f.other, "actor-b", "elsewhere")
	f.post(f.threadID, "actor-a", "mine")

	if got := f.unread("actor-a"); got[f.threadID] != 2 || got[f.other] != 1 {
		t.Fatalf("actor-a's unread = %v, want 2 on general and 1 on events", got)
	}
	if got := f.unread("actor-b"); got[f.threadID] != 1 || got[f.other] != 0 {
		t.Fatalf("actor-b's unread = %v, want only actor-a's post, never their own", got)
	}

	marked := f.markRead(f.threadID, "actor-a", map[string]string{})
	if marked.code != http.StatusOK {
		t.Fatalf("mark read = %d: %s", marked.code, marked.body)
	}
	var marker mwanachamacomm.ChatReadMarker
	marked.decode(t, &marker)
	if marker.ReaderID != "actor-a" || marker.ThreadID != f.threadID || marker.StructureID != "s-1" {
		t.Fatalf("marker = %+v", marker)
	}
	if got := f.unread("actor-a"); got[f.threadID] != 0 || got[f.other] != 1 {
		t.Fatalf("after marking general read, actor-a's unread = %v, want 0 and 1", got)
	}

	f.post(f.threadID, "actor-b", "three")
	if got := f.unread("actor-a"); got[f.threadID] != 1 {
		t.Fatalf("a post after the read point = %v, want 1 unread", got)
	}
}

func TestAReadPointMovesToTheNamedPostAndNeverBackwards(t *testing.T) {
	f := newRoomFixture(t)
	first := f.post(f.threadID, "actor-b", "one")
	second := f.post(f.threadID, "actor-b", "two")
	third := f.post(f.threadID, "actor-b", "three")

	if got := f.markRead(f.threadID, "actor-a", map[string]string{"message_id": second.ID}); got.code != http.StatusOK {
		t.Fatalf("mark read up to the second = %d: %s", got.code, got.body)
	}
	if got := f.unread("actor-a"); got[f.threadID] != 1 {
		t.Fatalf("read up to the second leaves %v, want the third unread", got)
	}
	if got := f.markRead(f.threadID, "actor-a", map[string]string{"message_id": first.ID}); got.code != http.StatusOK {
		t.Fatalf("mark read up to the first = %d: %s", got.code, got.body)
	}
	if got := f.unread("actor-a"); got[f.threadID] != 1 {
		t.Fatalf("an older mark moved the read point back: %v", got)
	}
	if got := f.markRead(f.threadID, "actor-a", map[string]string{"message_id": third.ID}); got.code != http.StatusOK {
		t.Fatalf("mark read up to the third = %d: %s", got.code, got.body)
	}
	if got := f.unread("actor-a"); got[f.threadID] != 0 {
		t.Fatalf("read up to the last post leaves %v, want nothing unread", got)
	}
}

func TestNobodyMarksAThreadReadForSomebodyElse(t *testing.T) {
	f := newRoomFixture(t)
	f.post(f.threadID, "actor-a", "for b")

	got := f.markRead(f.threadID, "actor-a", map[string]string{"reader_id": "actor-b", "structure_id": "s-2", "thread_id": f.other})
	if got.code != http.StatusOK {
		t.Fatalf("mark read = %d: %s", got.code, got.body)
	}
	var marker mwanachamacomm.ChatReadMarker
	got.decode(t, &marker)
	if marker.ReaderID != "actor-a" || marker.ThreadID != f.threadID || marker.StructureID != "s-1" {
		t.Fatalf("the body chose the marker: %+v", marker)
	}
	if unread := f.unread("actor-b"); unread[f.threadID] != 1 {
		t.Fatalf("actor-b's unread = %v, want actor-a's post still unread for them", unread)
	}
}

func TestAPostOfAnotherThreadAnswersAsAnInventedOne(t *testing.T) {
	f := newRoomFixture(t)
	elsewhere := f.post(f.other, "actor-b", "elsewhere")

	invented := f.markRead(f.threadID, "actor-a", map[string]string{"message_id": "msg-never"})
	foreign := f.markRead(f.threadID, "actor-a", map[string]string{"message_id": elsewhere.ID})
	if invented.code != http.StatusNotFound {
		t.Fatalf("an invented post = %d, want 404: %s", invented.code, invented.body)
	}
	if foreign.code != invented.code || foreign.message(t) != invented.message(t) {
		t.Fatalf("another thread's post answers %d %q and an invented one %d %q",
			foreign.code, foreign.message(t), invented.code, invented.message(t))
	}
	if got := f.unread("actor-a"); got[f.other] != 1 {
		t.Fatalf("a refused mark moved a read point: %v", got)
	}
}

func TestARoomListShowsAHundredForARoomWithMoreUnreadThanThat(t *testing.T) {
	f := newRoomFixture(t)
	for i := 0; i < 130; i++ {
		f.post(f.threadID, "actor-b", "post")
	}
	f.post(f.other, "actor-b", "elsewhere")

	got := f.unread("actor-a")
	if got[f.threadID] != mwanachamacomm.UnreadCap || got[f.other] != 1 {
		t.Fatalf("actor-a's unread = %v, want %d on general and 1 on events", got, mwanachamacomm.UnreadCap)
	}
}
