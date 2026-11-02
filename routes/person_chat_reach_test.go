package routes_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

type reachFixture struct {
	srv     *httptest.Server
	m       *mwanachamacomm.CommManager
	general string
	budget  string
	water   string
	roof    string
	news    string
	staff   string
}

func roomsOfPeople(rooms map[string][]string) mwanachamacomm.RoomsOf {
	return func(_ context.Context, actorID string) ([]string, error) { return rooms[actorID], nil }
}

func twoRooms(t *testing.T) reachFixture {
	t.Helper()
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{
		"s-1": {"lead-a", "asker-b", "other-c"},
		"s-2": {"lead-z", "asker-b"},
	}))
	m.UseLeading(leadingOf(map[string][]string{"s-1": {"lead-a"}, "s-2": {"lead-z"}}))
	m.UseLeads(func(_ context.Context, structureID string) ([]string, error) {
		return map[string][]string{"s-1": {"lead-a"}, "s-2": {"lead-z"}}[structureID], nil
	})
	m.UseRoomsOf(roomsOfPeople(map[string][]string{
		"lead-a":  {"s-1"},
		"asker-b": {"s-1", "s-2"},
		"other-c": {"s-1", "s-2"},
		"lead-z":  {"s-2"},
	}))
	f := reachFixture{srv: srv, m: m}
	f.general = seedChatThread(t, m, "s-1", "general")
	f.news = seedChatThread(t, m, "s-2", "news")
	f.budget = startAs(t, srv, "s-1", "lead-a", "Budget").ID
	f.water = memberStartAs(t, srv, "s-1", "asker-b", "Water point").ID
	f.roof = memberStartAs(t, srv, "s-1", "other-c", "Roof").ID
	f.staff = startAs(t, srv, "s-2", "lead-z", "Staff").ID
	return f
}

func (f reachFixture) reach(t *testing.T, caller, person string) []models.ChatReach {
	t.Helper()
	got := call(t, f.srv, http.MethodGet, "/people/"+person+"/chat/reach", caller, nil)
	if got.code != http.StatusOK {
		t.Fatalf("reach of %s as %s = %d: %s", person, caller, got.code, got.body)
	}
	var rows []models.ChatReach
	got.decode(t, &rows)
	return rows
}

func reasonsByThread(rows []models.ChatReach) map[string]models.ChatReachReason {
	out := map[string]models.ChatReachReason{}
	for _, r := range rows {
		out[r.ThreadID] = r.Reason
	}
	return out
}

func TestAPersonWhoOnlyBelongsReadsTheGroupThreadsAndThoseSheAskedFor(t *testing.T) {
	f := twoRooms(t)
	got := reasonsByThread(f.reach(t, "manager-m", "asker-b"))
	want := map[string]models.ChatReachReason{
		f.general: models.ChatReachGroup,
		f.water:   models.ChatReachAsked,
		f.news:    models.ChatReachGroup,
	}
	if len(got) != len(want) {
		t.Fatalf("asker-b reaches %v, want %v", got, want)
	}
	for id, reason := range want {
		if got[id] != reason {
			t.Errorf("asker-b reaches %s as %q, want %q", id, got[id], reason)
		}
	}
	for name, id := range map[string]string{"the leads' own thread": f.budget, "another's request": f.roof, "the other room's leads thread": f.staff} {
		if _, leaked := got[id]; leaked {
			t.Errorf("asker-b reaches %s, which is not hers to read", name)
		}
	}
}

func TestALeadAlsoReadsTheThreadsStillWithTheLeads(t *testing.T) {
	f := twoRooms(t)
	got := reasonsByThread(f.reach(t, "manager-m", "lead-a"))
	want := map[string]models.ChatReachReason{
		f.general: models.ChatReachGroup,
		f.budget:  models.ChatReachLeads,
		f.water:   models.ChatReachLeads,
		f.roof:    models.ChatReachLeads,
	}
	if len(got) != len(want) {
		t.Fatalf("lead-a reaches %v, want %v", got, want)
	}
	for id, reason := range want {
		if got[id] != reason {
			t.Errorf("lead-a reaches %s as %q, want %q", id, got[id], reason)
		}
	}
	for _, id := range []string{f.news, f.staff} {
		if _, leaked := got[id]; leaked {
			t.Errorf("lead-a reaches %s in a room she does not belong to", id)
		}
	}
}

func TestEachRowNamesItsRoomItsLabelItsAudienceAndWhoAskedForIt(t *testing.T) {
	f := twoRooms(t)
	var water *models.ChatReach
	for _, r := range f.reach(t, "manager-m", "asker-b") {
		if r.ThreadID == f.water {
			r := r
			water = &r
		}
	}
	if water == nil {
		t.Fatal("asker-b does not reach the thread she asked for")
	}
	if water.StructureID != "s-1" || water.Label != "Water point" || water.Audience != models.ChatAudienceLeads || water.RequestedBy != "asker-b" {
		t.Fatalf("the row = %+v, want s-1, Water point, the leads' audience, asked by asker-b", *water)
	}
}

func TestAReachCarriesNoPostsNoUnreadCountsAndNoReadPoints(t *testing.T) {
	f := twoRooms(t)
	secret := "the budget figure nobody else may see"
	if posted := call(t, f.srv, http.MethodPost, "/groups/s-1/chat/threads/"+f.budget+"/messages", "lead-a", map[string]string{"body": secret}); posted.code != http.StatusCreated {
		t.Fatalf("post = %d: %s", posted.code, posted.body)
	}
	if marked := call(t, f.srv, http.MethodPost, "/groups/s-1/chat/threads/"+f.general+"/read", "lead-a", map[string]string{}); marked.code != http.StatusOK {
		t.Fatalf("mark read = %d: %s", marked.code, marked.body)
	}
	got := call(t, f.srv, http.MethodGet, "/people/lead-a/chat/reach", "manager-m", nil)
	if got.code != http.StatusOK {
		t.Fatalf("reach = %d: %s", got.code, got.body)
	}
	if strings.Contains(string(got.body), secret) {
		t.Fatalf("a reach quotes a post: %s", got.body)
	}
	var rows []map[string]any
	got.decode(t, &rows)
	allowed := map[string]bool{"structure_id": true, "thread_id": true, "label": true, "audience": true, "requested_by": true, "reason": true}
	for _, row := range rows {
		for key := range row {
			if !allowed[key] {
				t.Errorf("a reach row carries %q, which is not one of the fields it names", key)
			}
		}
	}
}

func TestTheReachIsOfThePersonInThePathAndNeverOfTheCaller(t *testing.T) {
	f := twoRooms(t)
	asLead := reasonsByThread(f.reach(t, "asker-b", "lead-a"))
	if asLead[f.budget] != models.ChatReachLeads {
		t.Fatalf("asking about lead-a as asker-b gives %v, want lead-a's reach", asLead)
	}
	asAsker := reasonsByThread(f.reach(t, "lead-a", "asker-b"))
	if _, leaked := asAsker[f.budget]; leaked {
		t.Fatalf("asking about asker-b as lead-a gives %v, want asker-b's reach", asAsker)
	}
}

func TestAPersonWhoBelongsToNoRoomIsNotFoundLikeAnInventedOne(t *testing.T) {
	f := twoRooms(t)
	f.m.UseRoomsOf(roomsOfPeople(map[string][]string{
		"stranger": {"s-1", "s-2"},
		"no-rooms": nil,
	}))
	invented := call(t, f.srv, http.MethodGet, "/people/nobody-at-all/chat/reach", "manager-m", nil)
	if invented.code != http.StatusNotFound {
		t.Fatalf("an invented person = %d, want 404: %s", invented.code, invented.body)
	}
	for _, person := range []string{"stranger", "no-rooms"} {
		got := call(t, f.srv, http.MethodGet, "/people/"+person+"/chat/reach", "manager-m", nil)
		if got.code != http.StatusNotFound || got.message(t) != invented.message(t) {
			t.Errorf("%s = %d %q, want the invented person's %d %q", person, got.code, got.message(t), invented.code, invented.message(t))
		}
	}
}

func TestARoomTheHostListsAndBelongingDeniesIsNotReached(t *testing.T) {
	f := twoRooms(t)
	f.m.UseRoomsOf(roomsOfPeople(map[string][]string{"other-c": {"s-1", "s-2"}}))
	got := reasonsByThread(f.reach(t, "manager-m", "other-c"))
	if got[f.roof] != models.ChatReachAsked || got[f.general] != models.ChatReachGroup {
		t.Fatalf("other-c reaches %v, want her request and the open thread in s-1", got)
	}
	if _, leaked := got[f.news]; leaked {
		t.Fatalf("other-c reaches s-2's open thread though she does not belong to s-2: %v", got)
	}
}

func TestWithNoRoomsOfSuppliedNobodyHasAReach(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a"}}))
	seedChatThread(t, m, "s-1", "general")
	if got := call(t, srv, http.MethodGet, "/people/actor-a/chat/reach", "manager-m", nil); got.code != http.StatusNotFound {
		t.Fatalf("with no rooms-of seam = %d, want 404: %s", got.code, got.body)
	}
}

func TestAFailingRoomsLookupIsNeitherNotFoundNorForbidden(t *testing.T) {
	f := twoRooms(t)
	f.m.UseRoomsOf(func(context.Context, string) ([]string, error) {
		return nil, errors.New("directory down: secret detail")
	})
	got := call(t, f.srv, http.MethodGet, "/people/lead-a/chat/reach", "manager-m", nil)
	if got.code != http.StatusInternalServerError || strings.Contains(string(got.body), "secret detail") {
		t.Fatalf("a failing lookup = %d %s, want a redacted 500", got.code, got.body)
	}
}

func TestAReachNeedsItsOwnAction(t *testing.T) {
	f := twoRooms(t)
	only := func(allowed string) dispatch.Authorizer {
		return func(_ context.Context, action string) error {
			if action != allowed {
				return dispatch.ErrForbidden
			}
			return nil
		}
	}
	mount := func(allowed string) routes.Mount {
		return routes.Mount{Caller: fromCtx(callerKey), Device: fromCtx(deviceKey), Authorize: only(allowed)}
	}
	without := newServer(t, f.m, mount("comm.chat_thread.list"))
	if got := call(t, without, http.MethodGet, "/people/lead-a/chat/reach", "manager-m", nil); got.code != http.StatusForbidden {
		t.Fatalf("a caller without comm.person_reach.list = %d, want 403: %s", got.code, got.body)
	}
	with := newServer(t, f.m, mount("comm.person_reach.list"))
	if got := call(t, with, http.MethodGet, "/people/lead-a/chat/reach", "manager-m", nil); got.code != http.StatusOK {
		t.Fatalf("a caller with comm.person_reach.list = %d, want 200: %s", got.code, got.body)
	}
	if got := call(t, with, http.MethodGet, "/groups/s-1/chat/threads", "lead-a", nil); got.code != http.StatusForbidden {
		t.Fatalf("the reach action also opened the room's own list: %d", got.code)
	}
}

func TestOneRoomsReachIsNothingForSomebodyWhoDoesNotBelong(t *testing.T) {
	f := twoRooms(t)
	ctx := context.Background()
	if got, err := f.m.ChatReachOf(ctx, "s-2", "other-c"); err != nil || len(got) != 0 {
		t.Fatalf("ChatReachOf for a person outside the room = %v, %v; want nothing", got, err)
	}
	if got, err := f.m.ChatReachOf(ctx, "s-never", "lead-a"); err != nil || len(got) != 0 {
		t.Fatalf("ChatReachOf for an invented room = %v, %v; want nothing", got, err)
	}
	got, err := f.m.ChatReachOf(ctx, "s-1", "asker-b")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, r := range got {
		ids = append(ids, r.ThreadID)
	}
	sort.Strings(ids)
	want := []string{f.general, f.water}
	sort.Strings(want)
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("ChatReachOf s-1 for asker-b = %v, want %v", ids, want)
	}
}
