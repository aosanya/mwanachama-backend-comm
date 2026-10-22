package routes_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func newStandingFixture(t *testing.T) (func(method, path, caller string, body any) reply, *mwanachamacomm.CommManager, string) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a", "actor-b"}, "s-2": {"actor-c"}}))
	thread := seedChatThread(t, m, "s-1", "general")
	return func(method, path, caller string, body any) reply {
		return call(t, srv, method, path, caller, body)
	}, m, thread
}

func standingBody(thread string) map[string]any {
	return map[string]any{
		"thread_id": thread,
		"body":      "Bring your questions to the monthly meeting.",
		"rule":      "FREQ=MONTHLY",
		"zone":      "Africa/Nairobi",
		"starts_at": "2026-01-05T06:00:00Z",
		"author_id": "actor-z",
	}
}

func TestAStandingMessageIsSetUpReadListedAndStoppedOverTheMux(t *testing.T) {
	do, _, thread := newStandingFixture(t)

	created := do(http.MethodPost, "/groups/s-1/chat/standing", "actor-a", standingBody(thread))
	if created.code != http.StatusCreated {
		t.Fatalf("create = %d: %s", created.code, created.body)
	}
	var sm mwanachamacomm.StandingMessage
	created.decode(t, &sm)
	if sm.AuthorID != "actor-a" {
		t.Fatalf("author = %q, want the session's actor-a, never the body's", sm.AuthorID)
	}

	if got := do(http.MethodGet, "/groups/s-1/chat/standing/"+sm.ID, "actor-b", nil); got.code != http.StatusOK {
		t.Fatalf("get by a member = %d: %s", got.code, got.body)
	}
	listed := do(http.MethodGet, "/groups/s-1/chat/standing", "actor-b", nil)
	var all []mwanachamacomm.StandingMessage
	listed.decode(t, &all)
	if listed.code != http.StatusOK || len(all) != 1 {
		t.Fatalf("list = %d with %d, want one: %s", listed.code, len(all), listed.body)
	}

	stopped := do(http.MethodPost, "/groups/s-1/chat/standing/"+sm.ID+"/stop", "actor-b", map[string]any{"stopped_by": "actor-z"})
	if stopped.code != http.StatusOK {
		t.Fatalf("stop = %d: %s", stopped.code, stopped.body)
	}
	stopped.decode(t, &sm)
	if sm.StoppedBy != "actor-b" || !sm.Stopped() {
		t.Fatalf("stopped by %q, want the session's actor-b", sm.StoppedBy)
	}
	if again := do(http.MethodPost, "/groups/s-1/chat/standing/"+sm.ID+"/stop", "actor-a", nil); again.code != http.StatusConflict {
		t.Fatalf("second stop = %d, want 409", again.code)
	}
}

func TestAStandingMessageAnswersAnOutsiderExactlyAsAMissingOne(t *testing.T) {
	do, _, thread := newStandingFixture(t)
	created := do(http.MethodPost, "/groups/s-1/chat/standing", "actor-a", standingBody(thread))
	var sm mwanachamacomm.StandingMessage
	created.decode(t, &sm)

	for _, c := range []struct{ name, method, path string }{
		{"create", http.MethodPost, "/groups/s-1/chat/standing"},
		{"get", http.MethodGet, "/groups/s-1/chat/standing/" + sm.ID},
		{"list", http.MethodGet, "/groups/s-1/chat/standing"},
		{"stop", http.MethodPost, "/groups/s-1/chat/standing/" + sm.ID + "/stop"},
	} {
		var body any
		if c.name == "create" {
			body = standingBody(thread)
		}
		if got := do(c.method, c.path, "actor-c", body); got.code != http.StatusNotFound {
			t.Errorf("%s by an outsider = %d, want 404", c.name, got.code)
		}
	}
	if got := do(http.MethodGet, "/groups/s-1/chat/standing/invented", "actor-a", nil); got.code != http.StatusNotFound {
		t.Errorf("an invented id = %d, want 404", got.code)
	}
	if got := do(http.MethodGet, "/groups/s-2/chat/standing/"+sm.ID, "actor-c", nil); got.code != http.StatusNotFound {
		t.Errorf("another structure's id = %d, want 404", got.code)
	}
}

func TestAStandingMessageWithABadScheduleIsRefused(t *testing.T) {
	do, _, thread := newStandingFixture(t)
	body := standingBody(thread)
	body["zone"] = "Nowhere/Atall"
	if got := do(http.MethodPost, "/groups/s-1/chat/standing", "actor-a", body); got.code != http.StatusBadRequest {
		t.Fatalf("bad zone = %d, want 400: %s", got.code, got.body)
	}
	body = standingBody(thread)
	body["starts_at"] = "monday"
	if got := do(http.MethodPost, "/groups/s-1/chat/standing", "actor-a", body); got.code != http.StatusBadRequest {
		t.Fatalf("bad start = %d, want 400: %s", got.code, got.body)
	}
}

func TestTheHostSendsAStandingMessageSetUpOverTheMux(t *testing.T) {
	do, m, thread := newStandingFixture(t)
	created := do(http.MethodPost, "/groups/s-1/chat/standing", "actor-a", standingBody(thread))
	if created.code != http.StatusCreated {
		t.Fatalf("create = %d: %s", created.code, created.body)
	}
	sent, err := m.SendDueStandingMessages(context.Background(), time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0].AuthorID != "actor-a" || sent[0].ThreadID != thread {
		t.Fatalf("sent = %+v, want one post by actor-a into the thread", sent)
	}
}

func TestAStandingMessageOnAListOfDatesIsSetUpOverTheMux(t *testing.T) {
	do, _, thread := newStandingFixture(t)
	body := standingBody(thread)
	delete(body, "rule")
	body["dates"] = []string{"2026-01-05", "2026-05-04"}

	created := do(http.MethodPost, "/groups/s-1/chat/standing", "actor-a", body)
	if created.code != http.StatusCreated {
		t.Fatalf("create = %d: %s", created.code, created.body)
	}
	var sm mwanachamacomm.StandingMessage
	created.decode(t, &sm)
	if len(sm.Dates) != 2 || sm.Dates[1] != "2026-05-04" || sm.Rule != "" {
		t.Fatalf("dates = %v rule = %q, want the two dates and no rule", sm.Dates, sm.Rule)
	}

	body["rule"] = "FREQ=MONTHLY"
	if both := do(http.MethodPost, "/groups/s-1/chat/standing", "actor-a", body); both.code != http.StatusBadRequest {
		t.Fatalf("rule and dates together = %d, want 400: %s", both.code, both.body)
	}
}
