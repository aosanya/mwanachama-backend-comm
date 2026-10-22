package routes_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func roomWithLeadsAndAsker(t *testing.T) (*httptest.Server, *mwanachamacomm.CommManager) {
	t.Helper()
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{
		"s-1": {"lead-a", "operator-o", "asker-b", "other-c"},
		"s-2": {"lead-z"},
	}))
	m.UseLeading(leadingOf(map[string][]string{"s-1": {"lead-a", "operator-o"}, "s-2": {"lead-z"}}))
	m.UseLeads(func(_ context.Context, structureID string) ([]string, error) {
		return map[string][]string{
			"s-1": {"lead-a", "operator-o", "lead-a", "asker-b", "", "other-c", "stranger"},
		}[structureID], nil
	})
	return srv, m
}

func memberStartAs(t *testing.T, srv *httptest.Server, structureID, caller, label string) models.ChatThread {
	t.Helper()
	got := call(t, srv, http.MethodPost, "/groups/"+structureID+"/chat/threads", caller, map[string]string{"label": label})
	if got.code != http.StatusCreated {
		t.Fatalf("start %q as %s = %d: %s", label, caller, got.code, got.body)
	}
	var th models.ChatThread
	got.decode(t, &th)
	return th
}

func toldOf(t *testing.T, m *mwanachamacomm.CommManager, actorID string, event models.NotificationEvent) []models.Notification {
	t.Helper()
	got, err := m.NotificationStore.List(context.Background(), actorID, 0)
	if err != nil {
		t.Fatalf("list %s's notifications: %v", actorID, err)
	}
	var out []models.Notification
	for _, n := range got {
		if n.Event == event {
			out = append(out, n)
		}
	}
	return out
}

func TestAThreadAMemberStartsIsTheLeadsAndTheStartersAndTellsEveryLead(t *testing.T) {
	srv, m := roomWithLeadsAndAsker(t)
	th := memberStartAs(t, srv, "s-1", "asker-b", "  Water point   at the school ")
	if th.Audience != models.ChatAudienceLeads || th.RequestedBy != "asker-b" || th.Label != "Water point at the school" {
		t.Fatalf("a requested thread = %+v, want the leads' audience, requested by asker-b, the topic as its label", th)
	}

	for _, who := range []string{"asker-b", "lead-a", "operator-o"} {
		if _, listed := listedIDs(t, srv, "s-1", who)[th.ID]; !listed {
			t.Fatalf("%s does not list the requested thread", who)
		}
		if posted := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/messages", who, map[string]string{"body": "hello from " + who}); posted.code != http.StatusCreated {
			t.Fatalf("%s posting in the requested thread = %d: %s", who, posted.code, posted.body)
		}
	}
	if _, listed := listedIDs(t, srv, "s-1", "other-c")[th.ID]; listed {
		t.Fatal("a member who neither leads nor asked lists a requested thread")
	}
	hidden := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads/"+th.ID+"/messages", "other-c", nil)
	invented := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads/cthread-never/messages", "other-c", nil)
	if hidden.code != http.StatusNotFound || invented.code != hidden.code || invented.message(t) != hidden.message(t) {
		t.Fatalf("another member reading a requested thread = %d %q, an invented one %d %q; want the same 404",
			hidden.code, hidden.message(t), invented.code, invented.message(t))
	}

	for _, lead := range []string{"lead-a", "operator-o"} {
		got := toldOf(t, m, lead, models.EventThreadRequested)
		if len(got) != 1 {
			t.Fatalf("%s was told %d times, want once", lead, len(got))
		}
		n := got[0]
		if n.Category != models.CategoryChat || n.SubjectKind != models.SubjectChatThread || n.SubjectID != th.ID ||
			n.AuthorActorID != "asker-b" || n.StructureID != "s-1" {
			t.Fatalf("%s's notification = %+v, want chat, the chat thread, authored by the asker, in s-1", lead, n)
		}
	}
	for _, who := range []string{"asker-b", "other-c", "stranger"} {
		if got := toldOf(t, m, who, models.EventThreadRequested); len(got) != 0 {
			t.Fatalf("%s was told %v, want nothing: the asker is never told and a name that does not lead is skipped", who, got)
		}
	}
}

func TestAMemberStartingIsFencedLikeEveryOtherRoomAddress(t *testing.T) {
	srv, m := roomWithLeadsAndAsker(t)
	outsider := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads", "lead-z", map[string]string{"label": "Budget"})
	invented := call(t, srv, http.MethodPost, "/groups/s-never/chat/threads", "lead-z", map[string]string{"label": "Budget"})
	if outsider.code != http.StatusNotFound || invented.code != outsider.code || invented.message(t) != outsider.message(t) {
		t.Fatalf("an outsider asking = %d %q, an invented structure %d %q; want the same 404",
			outsider.code, outsider.message(t), invented.code, invented.message(t))
	}
	if empty := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads", "asker-b", map[string]string{"label": " - "}); empty.code != http.StatusBadRequest {
		t.Fatalf("starting with no label = %d, want 400: %s", empty.code, empty.body)
	}
	if got := toldOf(t, m, "lead-a", models.EventThreadRequested); len(got) != 0 {
		t.Fatalf("a refused request told the lead %v", got)
	}
}

func TestAMemberStartingNeverRevealsAThreadTheyCannotRead(t *testing.T) {
	srv, m := roomWithLeadsAndAsker(t)
	general := seedChatThread(t, m, "s-1", "/general")
	started := startAs(t, srv, "s-1", "lead-a", "Budget")
	theirs := memberStartAs(t, srv, "s-1", "other-c", "Market day")

	if got := memberStartAs(t, srv, "s-1", "asker-b", "budget"); got.ID == started.ID || got.RequestedBy != "asker-b" {
		t.Fatalf("asking for a topic the leads hold privately answered %+v, want a thread of the asker's own", got)
	}
	if got := memberStartAs(t, srv, "s-1", "asker-b", "Market Day"); got.ID == theirs.ID || got.RequestedBy != "asker-b" {
		t.Fatalf("asking for a topic another member asked for answered %+v, want a thread of the asker's own", got)
	}

	before := len(toldOf(t, m, "lead-a", models.EventThreadRequested))
	if got := memberStartAs(t, srv, "s-1", "asker-b", "General"); got.ID != general {
		t.Fatalf("asking for the room's open General answered %s, want General %s", got.ID, general)
	}
	mine := memberStartAs(t, srv, "s-1", "asker-b", "Water point")
	if again := memberStartAs(t, srv, "s-1", "asker-b", "water POINT"); again.ID != mine.ID {
		t.Fatalf("asking twice for one topic answered %s then %s, want the same thread", mine.ID, again.ID)
	}
	if got := len(toldOf(t, m, "lead-a", models.EventThreadRequested)) - before; got != 1 {
		t.Fatalf("the lead was told %d more times, want once: a thread the asker already reads tells nobody", got)
	}
}

func TestADeclinedThreadStaysReadOnlyToItsAskerAndTheLeadsAndTellsTheAsker(t *testing.T) {
	srv, m := roomWithLeadsAndAsker(t)
	th := memberStartAs(t, srv, "s-1", "asker-b", "Water point")
	call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/messages", "asker-b", map[string]string{"body": "the tap is dry"})

	declined := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/decline", "operator-o", nil)
	if declined.code != http.StatusOK {
		t.Fatalf("decline = %d, want 200: %s", declined.code, declined.body)
	}
	var got models.ChatThread
	declined.decode(t, &got)
	if got.ID != th.ID || got.DeclinedAt == nil || got.DeclinedBy != "operator-o" || got.Audience != models.ChatAudienceLeads {
		t.Fatalf("declined thread = %+v, want it closed by operator-o and never opened", got)
	}

	told := toldOf(t, m, "asker-b", models.EventThreadDeclined)
	if len(told) != 1 || told[0].SubjectKind != models.SubjectChatThread || told[0].SubjectID != th.ID ||
		told[0].AuthorActorID != "operator-o" || told[0].Category != models.CategoryChat || told[0].StructureID != "s-1" {
		t.Fatalf("the asker was told %+v, want one decline naming the thread and who declined it", told)
	}

	for _, who := range []string{"asker-b", "lead-a", "operator-o"} {
		if listed, ok := listedIDs(t, srv, "s-1", who)[th.ID]; !ok || !listed.Declined() {
			t.Fatalf("%s no longer lists the declined thread as declined", who)
		}
		posted := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/messages", who, map[string]string{"body": "hello"})
		if posted.code != http.StatusConflict {
			t.Fatalf("%s posting to a declined thread = %d, want 409: %s", who, posted.code, posted.body)
		}
		read := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads/"+th.ID+"/messages", who, nil)
		if read.code != http.StatusOK {
			t.Fatalf("%s reading a declined thread = %d, want 200: %s", who, read.code, read.body)
		}
	}
	if _, listed := listedIDs(t, srv, "s-1", "other-c")[th.ID]; listed {
		t.Fatal("a member who neither asked nor leads lists a declined thread")
	}
	for _, p := range chatPaths("s-1", th.ID)[1:] {
		closed := call(t, srv, p.method, p.path, "other-c", map[string]string{"body": "hello"})
		invented := call(t, srv, p.method, replaceID(p.path, th.ID, "cthread-never"), "other-c", map[string]string{"body": "hello"})
		if closed.code != http.StatusNotFound || invented.code != closed.code || invented.message(t) != closed.message(t) {
			t.Fatalf("%s on a declined thread as other-c = %d %q, an invented one %d %q; want the same 404",
				p.name, closed.code, closed.message(t), invented.code, invented.message(t))
		}
	}
	if opened := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/open", "asker-b", nil); opened.code != http.StatusNotFound {
		t.Fatalf("the asker opening their declined thread = %d, want 404", opened.code)
	}

	again := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/decline", "lead-a", nil)
	if again.code != http.StatusOK {
		t.Fatalf("declining a declined thread = %d, want 200 unchanged: %s", again.code, again.body)
	}
	var unchanged models.ChatThread
	again.decode(t, &unchanged)
	if unchanged.DeclinedBy != "operator-o" {
		t.Fatalf("declining again moved declined_by to %q", unchanged.DeclinedBy)
	}
	if told := toldOf(t, m, "asker-b", models.EventThreadDeclined); len(told) != 1 {
		t.Fatalf("declining again told the asker %d times in all, want once", len(told))
	}

	fresh := memberStartAs(t, srv, "s-1", "asker-b", "Water point")
	if fresh.ID == th.ID || fresh.Declined() {
		t.Fatalf("asking again after a decline answered %+v, want a new thread", fresh)
	}
	if started := startAs(t, srv, "s-1", "lead-a", "Water point"); started.ID == th.ID || started.Declined() {
		t.Fatalf("a lead starting a declined thread's label answered %+v, want a new thread", started)
	}

	reopened := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/open", "lead-a", nil)
	var open models.ChatThread
	reopened.decode(t, &open)
	if reopened.code != http.StatusOK || open.Declined() || open.Audience != models.ChatAudienceGroup {
		t.Fatalf("a lead opening a declined thread = %d %+v, want an ordinary open thread", reopened.code, open)
	}
	if listed, ok := listedIDs(t, srv, "s-1", "other-c")[th.ID]; !ok || listed.Declined() {
		t.Fatal("a declined thread a lead opened is not an ordinary thread to the room")
	}
	if posted := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/messages", "other-c", map[string]string{"body": "hello"}); posted.code != http.StatusCreated && posted.code != http.StatusOK {
		t.Fatalf("posting to a declined thread a lead opened = %d: %s", posted.code, posted.body)
	}
}

func TestOnlyALeadOfTheStructureDeclinesAndOnlyARequest(t *testing.T) {
	srv, m := roomWithLeadsAndAsker(t)
	th := memberStartAs(t, srv, "s-1", "asker-b", "Water point")
	for _, who := range []string{"asker-b", "other-c", "lead-z"} {
		refused := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/decline", who, nil)
		invented := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/cthread-never/decline", who, nil)
		if refused.code != http.StatusNotFound || invented.code != refused.code || invented.message(t) != refused.message(t) {
			t.Fatalf("%s declining = %d %q, an invented thread %d %q; want the same 404",
				who, refused.code, refused.message(t), invented.code, invented.message(t))
		}
	}
	if other := call(t, srv, http.MethodPost, "/groups/s-2/chat/threads/"+th.ID+"/decline", "lead-z", nil); other.code != http.StatusNotFound {
		t.Fatalf("a lead of another structure declining through their own room = %d, want 404", other.code)
	}
	if listed := listedIDs(t, srv, "s-1", "asker-b"); listed[th.ID].Declined() {
		t.Fatal("a refused decline closed the thread")
	}

	started := startAs(t, srv, "s-1", "lead-a", "Budget")
	general := seedChatThread(t, m, "s-1", "/general")
	for name, id := range map[string]string{"a thread nobody asked for": started.ID, "a thread already the group's": general} {
		if got := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+id+"/decline", "lead-a", nil); got.code != http.StatusConflict {
			t.Fatalf("declining %s = %d, want 409: %s", name, got.code, got.body)
		}
	}
	opened := memberStartAs(t, srv, "s-1", "other-c", "Market day")
	call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+opened.ID+"/open", "lead-a", nil)
	if got := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+opened.ID+"/decline", "lead-a", nil); got.code != http.StatusConflict {
		t.Fatalf("declining a request already opened = %d, want 409: %s", got.code, got.body)
	}
}

func TestWithNoLeadsNamedAMemberStartingStillWorksAndTellsNobody(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"lead-a", "asker-b"}}))
	m.UseLeading(leadingOf(map[string][]string{"s-1": {"lead-a"}}))
	th := memberStartAs(t, srv, "s-1", "asker-b", "Water point")
	if _, listed := listedIDs(t, srv, "s-1", "lead-a")[th.ID]; !listed {
		t.Fatal("the lead does not list a requested thread")
	}
	if got := toldOf(t, m, "lead-a", models.EventThreadRequested); len(got) != 0 {
		t.Fatalf("with no Leads mounted the lead was told %v", got)
	}
}

func TestAnyMemberStartsAThreadAndOnlyALeadStartsOneWithNoStarter(t *testing.T) {
	srv, m := roomWithLeadsAndAsker(t)
	byMember := memberStartAs(t, srv, "s-1", "other-c", "Budget")
	if byMember.RequestedBy != "other-c" || byMember.Audience != models.ChatAudienceLeads || !byMember.Declinable() {
		t.Fatalf("a member's start = %+v, want a private thread with its starter beside the leads", byMember)
	}
	byLead := startAs(t, srv, "s-1", "operator-o", "Road repairs")
	if byLead.RequestedBy != "" || byLead.Audience != models.ChatAudienceLeads {
		t.Fatalf("a lead's start = %+v, want a private thread with no starter on it", byLead)
	}
	if got := toldOf(t, m, "lead-a", models.EventThreadRequested); len(got) != 1 || got[0].SubjectID != byMember.ID {
		t.Fatalf("lead-a was told %+v, want one notice, for the member's thread only", got)
	}
	if _, listed := listedIDs(t, srv, "s-1", "other-c")[byLead.ID]; listed {
		t.Fatal("a member lists a thread a lead started before it is opened")
	}
	if opened := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/"+byMember.ID+"/open", "lead-a", nil); opened.code != http.StatusOK {
		t.Fatalf("a lead opening a member's thread = %d: %s", opened.code, opened.body)
	}
	if _, listed := listedIDs(t, srv, "s-1", "asker-b")[byMember.ID]; !listed {
		t.Fatal("an opened member thread is not listed to the rest of the group")
	}
}

func TestTheRequestAddressIsGone(t *testing.T) {
	srv, _ := roomWithLeadsAndAsker(t)
	got := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads/request", "asker-b", map[string]string{"topic": "Budget"})
	if got.code == http.StatusCreated {
		t.Fatalf("the folded request address still mints a thread: %s", got.body)
	}
}
