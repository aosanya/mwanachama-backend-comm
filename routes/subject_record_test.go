package routes_test

import (
	"context"
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func recordServer(t *testing.T) (func(method, path, caller string, body any) reply, *mwanachamacomm.CommManager) {
	t.Helper()
	srv, m := newCommServer(t)
	m.UseStanding(standingOf(standings{
		"record/rec-1": {
			"outsider-1": mwanachamacomm.StandingAudience,
			"staff-1":    mwanachamacomm.StandingAnswerer,
			"staff-2":    mwanachamacomm.StandingAnswerer,
		},
	}))
	m.UseAnswerers(answerersOf(map[string][]string{"record/rec-1": {"staff-1", "staff-2"}}))
	return func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }, m
}

func eventsTo(t *testing.T, m *mwanachamacomm.CommManager, actorID string, event models.NotificationEvent) []models.Notification {
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

func TestARecordsPrivateThreadTellsTheOtherSideWithARecordMessage(t *testing.T) {
	c, m := recordServer(t)
	thread := openSubjectThread(t, c, "record", "rec-1", "outsider-1", "private")

	post(t, c, thread.ID, "outsider-1", "my address is wrong")
	for _, staff := range []string{"staff-1", "staff-2"} {
		got := eventsTo(t, m, staff, models.EventRecordMessage)
		if len(got) != 1 || got[0].SubjectKind != models.SubjectSubjectThread || got[0].SubjectID != thread.ID {
			t.Fatalf("%s was told %v, want one record_message naming thread %s", staff, got, thread.ID)
		}
		if len(eventsTo(t, m, staff, models.EventThreadReply)) != 0 {
			t.Fatalf("%s was told a thread_reply on a record's thread", staff)
		}
	}
	if got := eventsTo(t, m, "outsider-1", models.EventRecordMessage); len(got) != 0 {
		t.Fatalf("the outsider was told of their own message: %v", got)
	}

	post(t, c, thread.ID, "staff-1", "corrected")
	if got := eventsTo(t, m, "outsider-1", models.EventRecordMessage); len(got) != 1 || got[0].SubjectID != thread.ID {
		t.Fatalf("the outsider was told %v, want one record_message naming %s", got, thread.ID)
	}
}

func TestARecordsThreadIsReadByItsOutsiderAndStaffAlone(t *testing.T) {
	c, _ := recordServer(t)
	thread := openSubjectThread(t, c, "record", "rec-1", "outsider-1", "private")
	post(t, c, thread.ID, "outsider-1", "hello")

	for _, reader := range []string{"outsider-1", "staff-1"} {
		if got := c(http.MethodGet, "/subject-threads/"+thread.ID+"/messages", reader, nil); got.code != http.StatusOK {
			t.Fatalf("%s reading = %d: %s", reader, got.code, got.body)
		}
	}
	invented := c(http.MethodGet, "/subject-threads/no-such-thread/messages", "stranger-1", nil)
	stranger := c(http.MethodGet, "/subject-threads/"+thread.ID+"/messages", "stranger-1", nil)
	if stranger.code != http.StatusNotFound || stranger.code != invented.code || string(stranger.body) != string(invented.body) {
		t.Fatalf("a stranger got %d %s, an invented thread %d %s: they must be the same 404", stranger.code, stranger.body, invented.code, invented.body)
	}
}

func TestEndingRecordAccessTellsTheOutsiderAndCannotBeMuted(t *testing.T) {
	_, m := recordServer(t)
	ctx := context.Background()
	if _, err := m.NotificationStore.SetPreference(ctx, "outsider-1", models.CategoryChat, true); err != nil {
		t.Skipf("no mute setter to exercise: %v", err)
	}
	n, err := m.EndRecordAccess(ctx, "rec-1", "outsider-1")
	if err != nil {
		t.Fatalf("EndRecordAccess: %v", err)
	}
	if n.Event != models.EventRecordAccessEnded || n.SubjectKind != models.SubjectRecord || n.SubjectID != "rec-1" {
		t.Fatalf("raised %+v, want record_access_ended on record rec-1", n)
	}
	if got := eventsTo(t, m, "outsider-1", models.EventRecordAccessEnded); len(got) != 1 {
		t.Fatalf("the outsider's inbox holds %v, want the one notice", got)
	}
	if _, err := m.EndRecordAccess(ctx, "", "outsider-1"); err == nil {
		t.Fatalf("ending access to no record was accepted")
	}
}
