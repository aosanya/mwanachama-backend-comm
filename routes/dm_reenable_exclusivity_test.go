package routes_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

// seedEmptiedThread leaves a conversation with three departed participants
// and nobody active, which is the only state a self re-enable is allowed
// from. Every step crosses the mux.
func seedEmptiedThread(t *testing.T, srv *httptest.Server, m *mwanachamacomm.CommManager) string {
	t.Helper()
	threadID := seedThread(t, m, "admin-1")
	base := "/dm/threads/" + threadID

	for _, who := range []string{"member-2", "member-3"} {
		if got := call(t, srv, http.MethodPost, base+"/invite", "admin-1",
			map[string]string{"actor_id": who}); got.code != http.StatusCreated {
			t.Fatalf("invite %s = %d: %s", who, got.code, got.body)
		}
		if got := call(t, srv, http.MethodPost, base+"/accept", who, nil); got.code != http.StatusOK {
			t.Fatalf("accept %s = %d: %s", who, got.code, got.body)
		}
	}
	if got := call(t, srv, http.MethodPost, base+"/promote", "admin-1",
		map[string]string{"actor_id": "member-2"}); got.code != http.StatusOK {
		t.Fatalf("promote = %d: %s", got.code, got.body)
	}
	for _, who := range []string{"admin-1", "member-3", "member-2"} {
		if got := call(t, srv, http.MethodPost, base+"/leave", who, nil); got.code != http.StatusNoContent {
			t.Fatalf("leave %s = %d: %s", who, got.code, got.body)
		}
	}
	return base
}

func activeCount(t *testing.T, srv *httptest.Server, base, caller string) int {
	t.Helper()
	got := call(t, srv, http.MethodGet, base+"/participants", caller, nil)
	if got.code != http.StatusOK {
		t.Fatalf("participants = %d: %s", got.code, got.body)
	}
	var roster []struct {
		State string `json:"state"`
	}
	got.decode(t, &roster)
	active := 0
	for _, p := range roster {
		if p.State == string(mwanachamacomm.DMStateActive) {
			active++
		}
	}
	return active
}

func TestSelfReEnableRefusesOnceAnotherDepartedParticipantHasClaimedTheThread(t *testing.T) {
	srv, m := newCommServer(t)
	base := seedEmptiedThread(t, srv, m)

	if got := call(t, srv, http.MethodPost, base+"/re-enable", "member-2", map[string]string{}); got.code != http.StatusOK {
		t.Fatalf("the first self re-enable = %d, want 200: %s", got.code, got.body)
	}
	if got := call(t, srv, http.MethodPost, base+"/re-enable", "member-3", map[string]string{}); got.code != http.StatusForbidden {
		t.Fatalf("the second self re-enable = %d, want 403 — only the actor who was last to leave may bring the thread back: %s",
			got.code, got.body)
	}
	if active := activeCount(t, srv, base, "member-2"); active != 1 {
		t.Fatalf("the roster shows %d active participants, want exactly 1", active)
	}
}

// Guards CM15. The fix is a single conditional UPDATE in
// claimEmptiedThread, so the invariant holds however the two requests
// interleave.
//
// This test cannot by itself prove the fix. SetMaxOpenConns(1) is needed
// because the SQLite in-memory pool otherwise hands each goroutine its own
// anonymous database, and one connection serialises the two statements,
// which closes the very window the race needed. That is exactly why the pin
// this replaces reported the bug fixed against unfixed code. The
// deterministic test above is what holds the rule; this one holds the
// concurrent shape, and a future reader should not restore an assertion
// that the race must land at least once, because under this setup it cannot.
func TestConcurrentSelfReEnableLeavesAtMostOneActive(t *testing.T) {
	const iterations = 40

	for i := 0; i < iterations; i++ {
		srv, m := newCommServer(t)
		base := seedEmptiedThread(t, srv, m)

		var wg sync.WaitGroup
		statuses := make([]int, 2)
		start := make(chan struct{})
		for idx, caller := range []string{"member-2", "member-3"} {
			wg.Add(1)
			go func(idx int, caller string) {
				defer wg.Done()
				<-start
				statuses[idx] = call(t, srv, http.MethodPost, base+"/re-enable", caller, map[string]string{}).code
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
			t.Fatalf("iteration %d: %d of 2 concurrent self re-enables succeeded (statuses %v), want exactly 1",
				i, successes, statuses)
		}
		if active := activeCount(t, srv, base, "member-2"); active != 1 {
			t.Fatalf("iteration %d: the roster shows %d active participants, want exactly 1", i, active)
		}
	}
}

// W14's two remaining shapes, which the behaviour suite does not cover: a
// sole participant may leave, and a non-admin is unaffected by the
// last-admin rule.
func TestTheLastAdminMayLeaveWhenNobodyElseIsActive(t *testing.T) {
	srv, m := newCommServer(t)

	alone := "/dm/threads/" + seedThread(t, m, "admin-1")
	if got := call(t, srv, http.MethodPost, alone+"/leave", "admin-1", nil); got.code != http.StatusNoContent {
		t.Fatalf("a sole participant leaving = %d, want 204: %s", got.code, got.body)
	}

	invitedOnly := "/dm/threads/" + seedThread(t, m, "admin-2", "member-3")
	if got := call(t, srv, http.MethodPost, invitedOnly+"/leave", "admin-2", nil); got.code != http.StatusNoContent {
		t.Fatalf("leaving while the only other member is merely invited = %d, want 204: %s", got.code, got.body)
	}
}

func TestANonAdminIsUnaffectedByTheLastAdminRule(t *testing.T) {
	srv, m := newCommServer(t)
	base := "/dm/threads/" + seedThread(t, m, "admin-1", "member-2")

	if got := call(t, srv, http.MethodPost, base+"/accept", "member-2", nil); got.code != http.StatusOK {
		t.Fatalf("accept = %d: %s", got.code, got.body)
	}
	if got := call(t, srv, http.MethodPost, base+"/leave", "member-2", nil); got.code != http.StatusNoContent {
		t.Fatalf("a non-admin leaving = %d, want 204: %s", got.code, got.body)
	}
	if active := activeCount(t, srv, base, "admin-1"); active != 1 {
		t.Fatalf("the roster shows %d active participants, want just the admin", active)
	}
}
