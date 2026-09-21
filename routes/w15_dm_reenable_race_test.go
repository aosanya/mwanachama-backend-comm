package routes_test

// Pins board row W15: DMStore.ReEnable's self-service path
// (dm_roster_impl.go) is a read-count-then-write TOCTOU, the same class of
// bug already filed against sibling repos as W19/W20/W21 (taskmanager),
// A14 (assetmanager) and AG34 (agency) — this is the first confirmed
// instance in mwanachama-backend-comm.
//
// ReEnable's self-service branch (memberID == by) checks
// `row.State == DMStateLeft && !hasActive(threadID)` and, only if both
// hold, calls setState(active) — two separate statements with no lock
// across the gap. models.ErrDMNotLastToLeave's own doc comment states the
// intended contract in one sentence: "only the actor who was last to leave
// can bring this thread back" — i.e. at most one departed participant may
// self-reactivate a thread nobody is currently active on. Two different
// departed participants racing the same check both observe
// hasActive()==false before either commits its own setState, so both
// writes land: two different members end the race both Active on a thread
// the exclusivity rule says should have accepted only one.
//
// If W15 is ever fixed (e.g. a transactional read-and-flip, or a
// unique/serializing guard so only the first writer's check is honoured),
// this test's assertion ("the race lands at least once in 40 tries") must
// start failing — update or delete it alongside the fix, per this repo's
// own convention for pinning tests (see w14_dm_admin_lockout_httptest_test.go).
import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

// newDMTestServerPinnedW15 is newDMTestServer but pins the underlying
// sqlite :memory: connection to exactly one connection. Without this, the
// default GORM connection pool hands concurrent goroutines separate
// anonymous in-memory databases (the identical gotcha
// mwanachama-backend-git's own CLAUDE.md documents and works around with
// `SetMaxOpenConns(1)` for its own concurrency tests), which manifests as
// spurious "no such table" errors under concurrent load and would mask
// the real race demonstrated here.
func newDMTestServerPinnedW15(t *testing.T) (*httptest.Server, mwanachamacomm.DMRepository) {
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

// TestPinsW15_ConcurrentSelfReEnableCanBothSucceed drives the race entirely
// over real HTTP: two ordinary (non-admin) participants who both left an
// otherwise-empty DM thread race a self "re-enable" call at the same
// moment. Real output demonstrating the bug (captured while authoring this
// test, 84/200 iterations of the throwaway probe this test replaces):
//
//	re-enable member-2: status=200 body=map[... state:active ...]
//	re-enable member-3: status=200 body=map[... state:active ...]
//	successes=2 (want at most 1 per exclusivity doc)
//
// i.e. both requests returned 200 and both rows ended up state=active,
// even though ErrDMNotLastToLeave's own doc comment says only the actor
// who was last to leave may bring the thread back.
func TestPinsW15_ConcurrentSelfReEnableCanBothSucceed(t *testing.T) {
	const iterations = 40
	raceLanded := 0

	for i := 0; i < iterations; i++ {
		srv, dm := newDMTestServerPinnedW15(t)
		client := srv.Client()

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

		// Everyone leaves, one at a time — no race here — so the thread
		// ends with nobody active and every participant in DMStateLeft.
		for _, m := range []string{"admin-1", "member-2", "member-3"} {
			if st, body := dmHTTPCall(t, client, http.MethodPost, base+"/leave", m, nil); st != http.StatusNoContent {
				t.Fatalf("leave %s: status=%d body=%v", m, st, body)
			}
		}

		// Race member-2 and member-3 both self-re-enabling at once.
		var wg sync.WaitGroup
		statuses := make([]int, 2)
		start := make(chan struct{})
		callers := []string{"member-2", "member-3"}
		for idx, caller := range callers {
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
		if successes > 1 {
			raceLanded++
			// Confirm the roster genuinely ended up with two active
			// participants, not just two 200s racing a single write.
			getStatus, participants := dmHTTPGetParticipants(t, client, base+"/participants", "member-2")
			if getStatus != http.StatusOK {
				t.Fatalf("participants status = %d, want 200", getStatus)
			}
			activeCount := 0
			for _, p := range participants {
				if state, _ := p["state"].(string); state == string(mwanachamacomm.DMStateActive) {
					activeCount++
				}
			}
			if activeCount < 2 {
				t.Fatalf("both re-enables returned 200 but roster shows only %d active participants: %v", activeCount, participants)
			}
			t.Logf("iteration %d: W15 reproduced — both re-enables returned 200, roster shows %d active participants: %v", i, activeCount, participants)
		}
	}

	if raceLanded == 0 {
		t.Fatalf("W15 appears fixed: 0/%d iterations landed two concurrent self-re-enables both succeeding — update or remove this pinning test", iterations)
	}
	t.Logf("W15 landed in %d/%d iterations", raceLanded, iterations)
}
