package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestTheRouteTableOverARunManagerWritesOnlyTheRunsCopyAndTheLiveTableStaysEmpty(t *testing.T) {
	db, live := newCommTestDB(t)
	live.UseBelonging(belongingOf(map[string][]string{"s-1": {"lead-a", "member-b"}}))
	live.UseLeading(leadingOf(map[string][]string{"s-1": {"lead-a"}}))
	run, err := live.ForRun("run-1")
	if err != nil {
		t.Fatal(err)
	}
	runSrv := newServer(t, run, testMount())
	liveSrv := newServer(t, live, testMount())

	started := call(t, runSrv, http.MethodPost, "/groups/s-1/chat/threads", "lead-a", map[string]string{"label": "Budget"})
	if started.code != http.StatusCreated {
		t.Fatalf("start = %d: %s", started.code, started.body)
	}
	var th models.ChatThread
	started.decode(t, &th)
	if opened := call(t, runSrv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/open", "lead-a", nil); opened.code != http.StatusOK {
		t.Fatalf("open = %d: %s", opened.code, opened.body)
	}
	if posted := call(t, runSrv, http.MethodPost, "/groups/s-1/chat/threads/"+th.ID+"/messages", "member-b", map[string]string{"body": "approved"}); posted.code != http.StatusCreated {
		t.Fatalf("post = %d: %s", posted.code, posted.body)
	}
	if muted := call(t, runSrv, http.MethodPut, "/notification-preferences/chat", "member-b", map[string]bool{"muted": true}); muted.code != http.StatusOK {
		t.Fatalf("mute = %d: %s", muted.code, muted.body)
	}

	listed := call(t, runSrv, http.MethodGet, "/groups/s-1/chat/threads/"+th.ID+"/messages", "lead-a", nil)
	var msgs []models.ChatMessage
	listed.decode(t, &msgs)
	if listed.code != http.StatusOK || len(msgs) != 1 || msgs[0].Body != "approved" {
		t.Fatalf("the run lists %d messages (%d): %s", len(msgs), listed.code, listed.body)
	}
	if got := threadsListed(t, liveSrv); len(got) != 0 {
		t.Fatalf("the live route table lists %d threads of the run's", len(got))
	}
	if got := threadsListed(t, runSrv); len(got) != 1 {
		t.Fatalf("the run's route table lists %d threads, want 1", len(got))
	}

	spec := live.Spec()
	liveRows := int64(0)
	for _, o := range spec.Objects {
		var n int64
		if err := db.Table(spec.TableFor(o)).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		liveRows += n
	}
	if liveRows != 0 {
		t.Fatalf("live rows = %d after the run's requests, want 0", liveRows)
	}
	rows, err := specstore.RowsOfRun(t.Context(), db, spec, "run-1")
	if err != nil || len(rows) == 0 {
		t.Fatalf("run-1 stored %+v, %v", rows, err)
	}
	removed, err := specstore.RemoveRun(t.Context(), db, spec, "run-1")
	if err != nil || removed == 0 {
		t.Fatalf("removed %d rows, %v", removed, err)
	}
	if got := threadsListed(t, runSrv); len(got) != 0 {
		t.Fatalf("the run lists %d threads after its removal", len(got))
	}
}

func threadsListed(t *testing.T, srv *httptest.Server) []models.ChatThread {
	t.Helper()
	listed := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads", "lead-a", nil)
	if listed.code != http.StatusOK {
		t.Fatalf("list = %d: %s", listed.code, listed.body)
	}
	var out []models.ChatThread
	listed.decode(t, &out)
	return out
}
