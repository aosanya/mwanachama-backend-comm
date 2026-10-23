package routes_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

type recipientsAsk struct {
	event    models.NotificationEvent
	room     mwanachamacomm.RaiseRoom
	defaults []string
}

func declineAs(t *testing.T, call func(method, path, caller string, body any) reply, threadID, caller string) reply {
	t.Helper()
	return call(http.MethodPost, "/groups/s-1/chat/threads/"+threadID+"/decline", caller, nil)
}

func TestWithNoRecipientsHookADeclineTellsTheAskerAlone(t *testing.T) {
	srv, m := roomWithLeadsAndAsker(t)
	c := func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }
	th := memberStartAs(t, srv, "s-1", "asker-b", "Water point")
	if got := declineAs(t, c, th.ID, "lead-a"); got.code != http.StatusOK {
		t.Fatalf("decline = %d: %s", got.code, got.body)
	}
	if got := toldOf(t, m, "asker-b", models.EventThreadDeclined); len(got) != 1 {
		t.Fatalf("the asker was told %d declines, want 1", len(got))
	}
	for _, who := range []string{"lead-a", "operator-o", "other-c"} {
		if got := toldOf(t, m, who, models.EventThreadDeclined); len(got) != 0 {
			t.Fatalf("%s was told of a decline with no hook: %+v", who, got)
		}
	}
}

func TestARecipientsHookDecidesWhoIsToldOfADecline(t *testing.T) {
	srv, m := roomWithLeadsAndAsker(t)
	c := func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }
	var asked []recipientsAsk
	m.UseRecipients(func(_ context.Context, event models.NotificationEvent, room mwanachamacomm.RaiseRoom, defaults []string) ([]string, error) {
		asked = append(asked, recipientsAsk{event, room, defaults})
		return []string{"other-c", "", "other-c"}, nil
	})
	th := memberStartAs(t, srv, "s-1", "asker-b", "Water point")
	if got := declineAs(t, c, th.ID, "lead-a"); got.code != http.StatusOK {
		t.Fatalf("decline = %d: %s", got.code, got.body)
	}
	if len(asked) != 1 {
		t.Fatalf("the hook was asked %d times, want once: %+v", len(asked), asked)
	}
	a := asked[0]
	if a.event != models.EventThreadDeclined || a.room.StructureID != "s-1" || a.room.ThreadID != th.ID || len(a.defaults) != 1 || a.defaults[0] != "asker-b" {
		t.Fatalf("the hook was asked %+v, want thread_declined in s-1 on %s with the asker as the default", a, th.ID)
	}
	if got := toldOf(t, m, "asker-b", models.EventThreadDeclined); len(got) != 0 {
		t.Fatalf("the asker was told although the hook left them out: %+v", got)
	}
	if got := toldOf(t, m, "other-c", models.EventThreadDeclined); len(got) != 1 {
		t.Fatalf("other-c was told %d times, want once: duplicates and empty names are dropped", len(got))
	}
}

func TestARecipientsFailureRefusesTheDeclineAndLeavesTheThreadOpen(t *testing.T) {
	srv, m := roomWithLeadsAndAsker(t)
	c := func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }
	th := memberStartAs(t, srv, "s-1", "asker-b", "Water point")
	m.UseRecipients(func(context.Context, models.NotificationEvent, mwanachamacomm.RaiseRoom, []string) ([]string, error) {
		return nil, errors.New("rule unreachable")
	})
	if got := declineAs(t, c, th.ID, "lead-a"); got.code != http.StatusInternalServerError {
		t.Fatalf("a failing recipients hook = %d, want 500: %s", got.code, got.body)
	}
	if listed, ok := listedIDs(t, srv, "s-1", "asker-b")[th.ID]; !ok || listed.Declined() {
		t.Fatal("the refused decline was stored")
	}
	if got := toldOf(t, m, "asker-b", models.EventThreadDeclined); len(got) != 0 {
		t.Fatalf("the refused decline told the asker: %+v", got)
	}
}

func TestARecipientsHookDecidesWhoIsToldOfAReply(t *testing.T) {
	c, m := requestServer(t)
	m.UseAnswerers(answerersOf(map[string][]string{"request/r-1": {"decider-1", "decider-2"}}))
	var asked []recipientsAsk
	m.UseRecipients(func(_ context.Context, event models.NotificationEvent, room mwanachamacomm.RaiseRoom, defaults []string) ([]string, error) {
		asked = append(asked, recipientsAsk{event, room, defaults})
		return []string{"decider-2"}, nil
	})
	private := openSubjectThread(t, c, "request", "r-1", "applicant-1", "private")
	post(t, c, private.ID, "applicant-1", "may I join?")
	if len(asked) != 1 {
		t.Fatalf("the hook was asked %d times, want once", len(asked))
	}
	a := asked[0]
	if a.event != models.EventThreadReply || a.room.ThreadID != private.ID || a.room.SubjectKind != "request" || a.room.SubjectID != "r-1" || a.room.StructureID != "" {
		t.Fatalf("the hook was asked %+v, want thread_reply on request/r-1", a)
	}
	if len(a.defaults) != 2 || a.defaults[0] != "decider-1" || a.defaults[1] != "decider-2" {
		t.Fatalf("the hook's defaults = %v, want the two answerers comm would tell", a.defaults)
	}
	if got := repliesTo(t, m, "decider-1"); len(got) != 0 {
		t.Fatalf("decider-1 was told although the hook left them out: %v", got)
	}
	if got := repliesTo(t, m, "decider-2"); len(got) != 1 {
		t.Fatalf("decider-2 was told %v, want one reply", got)
	}
}

func TestWithNoRecipientsHookAReplyTellsTodaysRecipients(t *testing.T) {
	c, m := requestServer(t)
	m.UseAnswerers(answerersOf(map[string][]string{"request/r-1": {"decider-1", "decider-2"}}))
	private := openSubjectThread(t, c, "request", "r-1", "applicant-1", "private")
	post(t, c, private.ID, "applicant-1", "may I join?")
	for _, who := range []string{"decider-1", "decider-2"} {
		if got := repliesTo(t, m, who); len(got) != 1 {
			t.Fatalf("%s was told %v, want one reply", who, got)
		}
	}
}

func TestARecipientsFailureRefusesTheReplyAndStoresNothing(t *testing.T) {
	c, m := requestServer(t)
	private := openSubjectThread(t, c, "request", "r-1", "applicant-1", "private")
	m.UseRecipients(func(context.Context, models.NotificationEvent, mwanachamacomm.RaiseRoom, []string) ([]string, error) {
		return nil, errors.New("rule unreachable")
	})
	got := c(http.MethodPost, "/subject-threads/"+private.ID+"/messages", "applicant-1", map[string]string{"body": "hi"})
	if got.code != http.StatusInternalServerError {
		t.Fatalf("a failing recipients hook = %d, want 500: %s", got.code, got.body)
	}
	var msgs []map[string]any
	c(http.MethodGet, "/subject-threads/"+private.ID+"/messages", "applicant-1", nil).decode(t, &msgs)
	if len(msgs) != 0 {
		t.Fatalf("the refused post was stored: %v", msgs)
	}
}
