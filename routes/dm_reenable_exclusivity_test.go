package routes_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

func dmHTTPGetParticipants(t *testing.T, client *http.Client, url, caller string) (int, []map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set(testCallerHeader, caller)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	var out []map[string]any
	raw, _ := io.ReadAll(resp.Body)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode response %q: %v", raw, err)
		}
	}
	return resp.StatusCode, out
}

func newDMExclusivityServer(t *testing.T) (*httptest.Server, mwanachamacomm.DMRepository) {
	t.Helper()
	db, tables := newRouteTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB(): %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	dm, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	mux := http.NewServeMux()
	for _, rt := range routes.DMRoutes(dm, headerIdentity{}) {
		mux.HandleFunc(rt.Pattern(""), rt.Handler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, dm
}

func seedEmptiedDMThread(t *testing.T, client *http.Client, srv *httptest.Server, dm mwanachamacomm.DMRepository) string {
	t.Helper()
	th, err := dm.CreateThread(context.Background(), mwanachamacomm.DMThread{CreatedBy: "admin-1"}, nil)
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	base := srv.URL + "/v1/dm/threads/" + th.ID

	for _, m := range []string{"member-2", "member-3"} {
		if st, body := dmHTTPCall(t, client, http.MethodPost, base+"/invite", "admin-1", map[string]string{"actor_id": m}); st != http.StatusCreated {
			t.Fatalf("invite %s: status=%d body=%v", m, st, body)
		}
		if st, body := dmHTTPCall(t, client, http.MethodPost, base+"/accept", m, nil); st != http.StatusOK {
			t.Fatalf("accept %s: status=%d body=%v", m, st, body)
		}
	}
	if st, body := dmHTTPCall(t, client, http.MethodPost, base+"/promote", "admin-1", map[string]string{"actor_id": "member-2"}); st != http.StatusOK {
		t.Fatalf("promote member-2: status=%d body=%v", st, body)
	}
	for _, m := range []string{"admin-1", "member-3", "member-2"} {
		if st, body := dmHTTPCall(t, client, http.MethodPost, base+"/leave", m, nil); st != http.StatusNoContent {
			t.Fatalf("leave %s: status=%d body=%v", m, st, body)
		}
	}
	return base
}

func activeParticipants(t *testing.T, client *http.Client, base, caller string) []map[string]any {
	t.Helper()
	st, participants := dmHTTPGetParticipants(t, client, base+"/participants", caller)
	if st != http.StatusOK {
		t.Fatalf("participants status = %d, want 200", st)
	}
	var active []map[string]any
	for _, p := range participants {
		if state, _ := p["state"].(string); state == string(mwanachamacomm.DMStateActive) {
			active = append(active, p)
		}
	}
	return active
}

func TestSelfReEnableRefusesOnceAnotherDepartedParticipantHasClaimedTheThread(t *testing.T) {
	srv, dm := newDMExclusivityServer(t)
	client := srv.Client()
	base := seedEmptiedDMThread(t, client, srv, dm)

	if st, body := dmHTTPCall(t, client, http.MethodPost, base+"/re-enable", "member-2", map[string]string{}); st != http.StatusOK {
		t.Fatalf("first self re-enable: status=%d body=%v, want 200", st, body)
	}
	st, body := dmHTTPCall(t, client, http.MethodPost, base+"/re-enable", "member-3", map[string]string{})
	if st != http.StatusForbidden {
		t.Fatalf("second self re-enable: status=%d body=%v, want 403 — only the actor who was last to leave may bring the thread back", st, body)
	}
	if active := activeParticipants(t, client, base, "member-2"); len(active) != 1 {
		t.Fatalf("roster shows %d active participants, want exactly 1: %v", len(active), active)
	}
}

// TestConcurrentSelfReEnableLeavesAtMostOneActive guards CM15. The fix is a
// single conditional UPDATE in claimEmptiedThread, so the invariant holds
// however the two requests interleave.
//
// This test cannot by itself prove the fix: `SetMaxOpenConns(1)` is needed
// because the SQLite in-memory pool otherwise hands each goroutine its own
// anonymous database, and one connection serializes the two statements, which
// closes the very window the race needed. That is exactly why the pin this
// replaces reported "W15 appears fixed" against unfixed code. The
// deterministic test above is what actually holds the rule; this one holds the
// concurrent shape, and a future reader should not restore an assertion that
// the race must land at least once, because under this setup it cannot.
func TestConcurrentSelfReEnableLeavesAtMostOneActive(t *testing.T) {
	const iterations = 40

	for i := 0; i < iterations; i++ {
		srv, dm := newDMExclusivityServer(t)
		client := srv.Client()
		base := seedEmptiedDMThread(t, client, srv, dm)

		var wg sync.WaitGroup
		statuses := make([]int, 2)
		start := make(chan struct{})
		for idx, caller := range []string{"member-2", "member-3"} {
			wg.Add(1)
			go func(idx int, caller string) {
				defer wg.Done()
				<-start
				st, _ := dmHTTPCall(t, client, http.MethodPost, base+"/re-enable", caller, map[string]string{})
				statuses[idx] = st
			}(idx, caller)
		}
		close(start)
		wg.Wait()

		successes := 0
		for _, s := range statuses {
			if s == http.StatusOK {
				successes++
			}
		}
		if successes != 1 {
			t.Fatalf("iteration %d: %d of 2 concurrent self re-enables succeeded (statuses %v), want exactly 1", i, successes, statuses)
		}
		if active := activeParticipants(t, client, base, "member-2"); len(active) != 1 {
			t.Fatalf("iteration %d: roster shows %d active participants, want exactly 1: %v", i, len(active), active)
		}
	}
}
