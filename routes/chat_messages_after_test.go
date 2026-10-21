package routes_test

import (
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func (f roomFixture) messages(threadID, caller, query string) reply {
	f.t.Helper()
	return f.call(http.MethodGet, "/groups/s-1/chat/threads/"+threadID+"/messages"+query, caller, nil)
}

func bodiesOf(t *testing.T, got reply) []string {
	t.Helper()
	if got.code != http.StatusOK {
		t.Fatalf("list messages = %d: %s", got.code, got.body)
	}
	var msgs []mwanachamacomm.ChatMessage
	got.decode(t, &msgs)
	out := []string{}
	for _, m := range msgs {
		out = append(out, m.Body)
	}
	return out
}

func TestListingMessagesAfterAPostReturnsOnlyTheNewerOnes(t *testing.T) {
	f := newRoomFixture(t)
	f.post(f.threadID, "actor-b", "one")
	second := f.post(f.threadID, "actor-b", "two")
	f.post(f.other, "actor-b", "elsewhere")
	f.post(f.threadID, "actor-a", "three")
	last := f.post(f.threadID, "actor-b", "four")

	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "")); len(got) != 4 || got[0] != "one" || got[3] != "four" {
		t.Fatalf("without after = %v, want every post in the thread", got)
	}
	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "?after="+second.ID)); len(got) != 2 || got[0] != "three" || got[1] != "four" {
		t.Fatalf("after the second = %v, want three and four", got)
	}
	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "?after="+last.ID)); len(got) != 0 {
		t.Fatalf("after the last = %v, want nothing", got)
	}
}

func TestAnAfterOutsideTheThreadAnswersNotFound(t *testing.T) {
	f := newRoomFixture(t)
	f.post(f.threadID, "actor-b", "one")
	elsewhere := f.post(f.other, "actor-b", "elsewhere")

	for name, query := range map[string]string{
		"another thread's post": "?after=" + elsewhere.ID,
		"an invented post":      "?after=msg-invented",
	} {
		if got := f.messages(f.threadID, "actor-a", query); got.code != http.StatusNotFound {
			t.Fatalf("%s = %d: %s, want 404", name, got.code, got.body)
		}
	}
}

func TestAnOutsiderNamingAnAfterStillAnswersNotFound(t *testing.T) {
	f := newRoomFixture(t)
	first := f.post(f.threadID, "actor-b", "one")
	if got := f.messages(f.threadID, "actor-outsider", "?after="+first.ID); got.code != http.StatusNotFound {
		t.Fatalf("outsider = %d: %s, want 404", got.code, got.body)
	}
}
