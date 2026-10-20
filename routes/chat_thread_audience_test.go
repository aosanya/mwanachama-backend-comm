package routes_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func leadingOf(leads map[string][]string) mwanachamacomm.Leading {
	return func(_ context.Context, actorID, structureID string) (bool, error) {
		for _, a := range leads[structureID] {
			if a == actorID {
				return true, nil
			}
		}
		return false, nil
	}
}

func roomWithALeadAndAMember(t *testing.T) (*httptest.Server, *mwanachamacomm.CommManager) {
	t.Helper()
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{
		"s-1": {"lead-a", "member-b"},
		"s-2": {"lead-z"},
	}))
	m.UseLeading(leadingOf(map[string][]string{"s-1": {"lead-a"}, "s-2": {"lead-z"}}))
	return srv, m
}

func startAs(t *testing.T, srv *httptest.Server, structureID, caller, label string) models.ChatThread {
	t.Helper()
	got := call(t, srv, http.MethodPost, "/groups/"+structureID+"/chat/threads", caller, map[string]string{"label": label})
	if got.code != http.StatusCreated {
		t.Fatalf("start %q as %s = %d: %s", label, caller, got.code, got.body)
	}
	var th models.ChatThread
	got.decode(t, &th)
	return th
}

func listedIDs(t *testing.T, srv *httptest.Server, structureID, caller string) map[string]mwanachamacomm.ChatThreadView {
	t.Helper()
	got := call(t, srv, http.MethodGet, "/groups/"+structureID+"/chat/threads", caller, nil)
	if got.code != http.StatusOK {
		t.Fatalf("list as %s = %d: %s", caller, got.code, got.body)
	}
	var views []mwanachamacomm.ChatThreadView
	got.decode(t, &views)
	out := map[string]mwanachamacomm.ChatThreadView{}
	for _, v := range views {
		out[v.ID] = v
	}
	return out
}

func TestAStartedThreadBeginsWithTheLeadsAndAMemberCannotSeeItExists(t *testing.T) {
	srv, _ := roomWithALeadAndAMember(t)
	th := startAs(t, srv, "s-1", "lead-a", "Budget")
	if th.Audience != models.ChatAudienceLeads {
		t.Fatalf("a started thread's audience = %q, want leads", th.Audience)
	}
	if posted := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/messages", "lead-a", map[string]string{"body": "first draft"}); posted.code != http.StatusCreated {
		t.Fatalf("the lead posting = %d: %s", posted.code, posted.body)
	}

	if _, listed := listedIDs(t, srv, "s-1", "lead-a")[th.ID]; !listed {
		t.Fatal("the lead does not see the thread they started")
	}
	if _, listed := listedIDs(t, srv, "s-1", "member-b")[th.ID]; listed {
		t.Fatal("a member who does not lead sees a thread still with the leads")
	}

	for _, p := range chatPaths("s-1", th.ID)[1:] {
		hidden := call(t, srv, p.method, p.path, "member-b", map[string]string{"body": "hello"})
		invented := call(t, srv, p.method, "/groups/s-1/chat/threads/cthread-never"+p.path[len("/groups/s-1/chat/threads/"+th.ID):], "member-b", map[string]string{"body": "hello"})
		if hidden.code != http.StatusNotFound {
			t.Fatalf("%s on a leads thread as a member = %d, want 404: %s", p.name, hidden.code, hidden.body)
		}
		if invented.code != hidden.code || invented.message(t) != hidden.message(t) {
			t.Fatalf("%s: a leads thread answers %d %q and an invented one %d %q; the difference is an enumeration oracle",
				p.name, hidden.code, hidden.message(t), invented.code, invented.message(t))
		}
	}
}

func TestOpeningAThreadGivesItToTheGroupWithWhatWasWrittenBefore(t *testing.T) {
	srv, _ := roomWithALeadAndAMember(t)
	th := startAs(t, srv, "s-1", "lead-a", "Budget")
	call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/messages", "lead-a", map[string]string{"body": "first draft"})

	opened := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/open", "lead-a", nil)
	if opened.code != http.StatusOK {
		t.Fatalf("open = %d, want 200: %s", opened.code, opened.body)
	}
	var got models.ChatThread
	opened.decode(t, &got)
	if got.ID != th.ID || got.Audience != models.ChatAudienceGroup {
		t.Fatalf("opened thread = %+v, want %s with the group's audience", got, th.ID)
	}

	view, listed := listedIDs(t, srv, "s-1", "member-b")[th.ID]
	if !listed || view.Audience != models.ChatAudienceGroup || view.Unread != 1 {
		t.Fatalf("the member lists %+v (listed %v), want the opened thread with the lead's one post unread", view, listed)
	}
	msgs := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads/"+th.ID+"/messages", "member-b", nil)
	var posts []models.ChatMessage
	msgs.decode(t, &posts)
	if len(posts) != 1 || posts[0].Body != "first draft" {
		t.Fatalf("the member reads %+v, want what the lead wrote before opening", posts)
	}
	if posted := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/messages", "member-b", map[string]string{"body": "thanks"}); posted.code != http.StatusCreated {
		t.Fatalf("the member posting in the opened thread = %d: %s", posted.code, posted.body)
	}

	again := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/open", "lead-a", nil)
	if again.code != http.StatusOK {
		t.Fatalf("opening an open thread again = %d, want 200: %s", again.code, again.body)
	}
}

func TestOnlyALeadOfTheStructureOpensOrStartsAThread(t *testing.T) {
	srv, m := roomWithALeadAndAMember(t)
	th := startAs(t, srv, "s-1", "lead-a", "Budget")

	refusals := map[string]reply{
		"a member":              call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/open", "member-b", nil),
		"another lead":          call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/open", "lead-z", nil),
		"another structure":     call(t, srv, http.MethodPost, "/groups/s-2/chat/threads/"+th.ID+"/open", "lead-z", nil),
		"an invented thread":    call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/cthread-never/open", "lead-a", nil),
		"a member starting one": call(t, srv, http.MethodPost, "/groups/s-1/chat/threads", "member-b", map[string]string{"label": "Budget"}),
	}
	want := refusals["an invented thread"]
	for who, got := range refusals {
		if got.code != http.StatusNotFound || got.message(t) != want.message(t) {
			t.Errorf("%s = %d %q, want the same 404 as an invented thread: %s", who, got.code, got.message(t), got.body)
		}
	}

	threads, err := m.ListThreads(context.Background(), "s-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 || threads[0].Audience != models.ChatAudienceLeads {
		t.Fatalf("after the refusals s-1 holds %+v, want the one thread still with the leads", threads)
	}
}

func TestAThreadMintedByItsTopicIsTheGroups(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"member-b"}}))
	id := seedChatThread(t, m, "s-1", "general")

	view, listed := listedIDs(t, srv, "s-1", "member-b")[id]
	if !listed || view.Audience != models.ChatAudienceGroup {
		t.Fatalf("a topic-minted thread lists as %+v (listed %v), want the group's with no leads resolver mounted", view, listed)
	}
	if posted := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+id+"/messages", "member-b", map[string]string{"body": "hi"}); posted.code != http.StatusCreated {
		t.Fatalf("posting in a topic-minted thread = %d: %s", posted.code, posted.body)
	}
}

func TestWithNoLeadingSuppliedNobodyLeads(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a"}}))
	th, err := m.StartThread(context.Background(), "s-1", "Budget")
	if err != nil {
		t.Fatal(err)
	}

	for _, got := range []reply{
		call(t, srv, http.MethodPost, "/groups/s-1/chat/threads", "actor-a", map[string]string{"label": "Other"}),
		call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/open", "actor-a", nil),
		call(t, srv, http.MethodGet, "/groups/s-1/chat/threads/"+th.ID+"/messages", "actor-a", nil),
	} {
		if got.code != http.StatusNotFound {
			t.Fatalf("with no leading mounted = %d, want 404: %s", got.code, got.body)
		}
	}
	if _, listed := listedIDs(t, srv, "s-1", "actor-a")[th.ID]; listed {
		t.Fatal("with no leading mounted a leads thread is listed")
	}
}

func TestAHostOpensAThreadItStartedSoTheRoomHasAnOpenGeneral(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"member-b"}}))
	ctx := context.Background()
	th, err := m.StartThread(ctx, "s-1", "General")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.OpenThread(ctx, "s-1", th.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.OpenThread(ctx, "s-2", th.ID); err == nil {
		t.Fatal("a thread opened under another structure's id")
	}
	if _, listed := listedIDs(t, srv, "s-1", "member-b")[th.ID]; !listed {
		t.Fatal("the member does not see a General the host opened")
	}
}
