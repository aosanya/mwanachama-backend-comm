package routes_test

// Upgrades W14's pin from a direct-DMStore call to a real
// httptest.NewServer + routes.DMRoutes(...) mux, driven with a real
// http.Client over real HTTP — this repo's own routes/doc.go documents a
// preference for real-mux tests over calling DMStore/ChatStore/
// ModerationStore directly, and w14_dm_admin_lockout_test.go (still kept,
// unmodified) only ever called the store's methods in Go, never through
// the HTTP boundary a real caller (the gateway, ultimately a Flutter
// client) actually goes through. This file reproduces the exact same two
// lockout shapes — self-kick, and an ordinary admin Leave stranding a
// remaining participant — but every roster mutation and every attempted
// recovery is a real POST against a real *http.Server, with the caller's
// identity carried the only way a stateless HTTP boundary can carry it
// between separate requests: a header, read by a per-request
// routes.Identity implementation (headerIdentity, below) exactly the way a
// mounting gateway's own session-derived Identity would read a cookie.
//
// If W14 is ever fixed, every "expect failure" assertion below should
// start failing loudly (a 2xx where 4xx was expected) — update this file
// alongside the fix rather than deleting it, the same instruction the
// original pin carries.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

// headerIdentity reads the caller id from a request header — the
// real-HTTP-boundary equivalent of routes_test.go's fixed testIdentity,
// needed here because a single shared *httptest.Server answers requests
// from more than one simulated caller (admin-1, member-2, ...) across the
// lifetime of one test, and identity can only travel between separate real
// HTTP requests via something the request itself carries.
type headerIdentity struct{}

const testCallerHeader = "X-Test-Caller"

func (headerIdentity) CallerID(r *http.Request) string       { return r.Header.Get(testCallerHeader) }
func (headerIdentity) CallerDeviceID(r *http.Request) string { return "" }

// newDMTestServer builds a real *httptest.Server serving exactly
// routes.DMRoutes over a real *gorm.DB-backed DMStore, mirroring how a
// mounting gateway would register the same route slice on its own mux.
func newDMTestServer(t *testing.T) (*httptest.Server, mwanachamacomm.DMRepository) {
	t.Helper()
	db, tables := newRouteTestDB(t)
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

// dmHTTPCall makes one real HTTP call against srv as caller, returning the
// response's status code and decoded body (as a map, forgivingly — a 204
// No Content response has no body to decode and returns a nil map).
func dmHTTPCall(t *testing.T, client *http.Client, method, url, caller string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = strings.NewReader(string(b))
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("NewRequest %s %s: %v", method, url, err)
	}
	if caller != "" {
		req.Header.Set(testCallerHeader, caller)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return resp.StatusCode, nil
	}
	var out map[string]any
	raw, _ := io.ReadAll(resp.Body)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode response %q: %v", raw, err)
		}
	}
	return resp.StatusCode, out
}

// TestHTTP_W14_SoleAdminSelfKickThenLockedOut reproduces W14's narrowest
// form entirely over real HTTP: the thread's sole admin kicks themselves
// (an operation Kick has no self-kick guard against), then neither
// Promote nor ReEnable — the only two roster writes that could put an
// admin back — can recover, because both require an already-active admin
// caller and there is none left.
func TestHTTP_W14_SoleAdminSelfKickThenLockedOut(t *testing.T) {
	srv, dm := newDMTestServer(t)
	client := srv.Client()

	th, err := dm.CreateThread(context.Background(), mwanachamacomm.DMThread{CreatedBy: "admin-1"}, nil)
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	base := srv.URL + "/v1/dm/threads/" + th.ID

	// The sole admin kicks themselves. Real bug: Kick never checks
	// by == actorID, so this succeeds (204) instead of being refused.
	status, body := dmHTTPCall(t, client, http.MethodPost, base+"/kick", "admin-1", map[string]string{"actor_id": "admin-1"})
	if status != http.StatusNoContent {
		t.Fatalf("W14 appears fixed over HTTP: self-kick was refused (status=%d body=%v) — update this test to assert that refusal instead", status, body)
	}

	// Recovery attempt 1: self-promote. There is no active admin left to
	// authorize it (the caller is the very admin who just got kicked), so
	// this must fail.
	promoteStatus, promoteBody := dmHTTPCall(t, client, http.MethodPost, base+"/promote", "admin-1", map[string]string{"actor_id": "admin-1"})

	// Recovery attempt 2: self re-enable. ReEnable's self path only
	// accepts a caller in DMStateLeft who was the very last participant of
	// any kind — a self-*kicked* sole admin is in DMStateKicked, not Left,
	// so this must also fail.
	reenableStatus, reenableBody := dmHTTPCall(t, client, http.MethodPost, base+"/re-enable", "admin-1", map[string]string{"actor_id": "admin-1"})

	if promoteStatus < 400 || reenableStatus < 400 {
		t.Fatalf("W14 appears fixed over HTTP: a recovery path now succeeds (promote status=%d body=%v; re-enable status=%d body=%v) — update this test",
			promoteStatus, promoteBody, reenableStatus, reenableBody)
	}
	t.Logf("W14 reproduced over real HTTP: self-kick succeeded (204), then both recovery paths were refused (promote=%d %v, re-enable=%d %v)",
		promoteStatus, promoteBody, reenableStatus, reenableBody)
}

// TestHTTP_W14_LastAdminLeavesStrandsRemainingParticipant reproduces W14's
// broadest, most realistic form over real HTTP: the sole admin leaves
// normally (not kicked) while another active participant remains on the
// thread. That participant is now permanently stuck — Promote requires an
// already-active admin caller (there is none), and ReEnable's admin path
// carries the identical requirement, so it can't bring the departed admin
// back either. Every step here — invite, accept, leave, and both failed
// recovery attempts — goes through the real mux over real HTTP.
func TestHTTP_W14_LastAdminLeavesStrandsRemainingParticipant(t *testing.T) {
	srv, dm := newDMTestServer(t)
	client := srv.Client()

	th, err := dm.CreateThread(context.Background(), mwanachamacomm.DMThread{CreatedBy: "admin-1"}, nil)
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	base := srv.URL + "/v1/dm/threads/" + th.ID

	inviteStatus, inviteBody := dmHTTPCall(t, client, http.MethodPost, base+"/invite", "admin-1", map[string]string{"actor_id": "member-2"})
	if inviteStatus != http.StatusCreated {
		t.Fatalf("invite status = %d, want 201: %v", inviteStatus, inviteBody)
	}

	acceptStatus, acceptBody := dmHTTPCall(t, client, http.MethodPost, base+"/accept", "member-2", nil)
	if acceptStatus != http.StatusOK {
		t.Fatalf("accept status = %d, want 200: %v", acceptStatus, acceptBody)
	}

	// The sole admin leaves the thread normally, ordinary member-2 still
	// active. Real bug: Leave has no last-admin guard.
	leaveStatus, leaveBody := dmHTTPCall(t, client, http.MethodPost, base+"/leave", "admin-1", nil)
	if leaveStatus != http.StatusNoContent {
		t.Fatalf("W14 appears fixed over HTTP: last-admin Leave was refused (status=%d body=%v) — update this test to assert that refusal instead", leaveStatus, leaveBody)
	}

	// Recovery attempt 1: member-2 tries to self-promote. No active admin
	// exists any more to authorize it.
	promoteStatus, promoteBody := dmHTTPCall(t, client, http.MethodPost, base+"/promote", "member-2", map[string]string{"actor_id": "member-2"})

	// Recovery attempt 2: member-2 tries to bring admin-1 back via
	// ReEnable's admin path. That path also requires an already-active
	// admin caller — member-2 isn't one — so it fails too.
	reenableStatus, reenableBody := dmHTTPCall(t, client, http.MethodPost, base+"/re-enable", "member-2", map[string]string{"actor_id": "admin-1"})

	if promoteStatus < 400 || reenableStatus < 400 {
		t.Fatalf("W14 appears fixed over HTTP: a recovery path now succeeds (promote status=%d body=%v; re-enable status=%d body=%v) — update this test",
			promoteStatus, promoteBody, reenableStatus, reenableBody)
	}
	t.Logf("W14 reproduced over real HTTP: admin-1 left (204) leaving member-2 stranded — both recovery paths were refused (promote=%d %v, re-enable=%d %v)",
		promoteStatus, promoteBody, reenableStatus, reenableBody)

	// Confirm member-2 is still on the roster (not just refused a write,
	// genuinely stuck) — the roster still has no active admin at all.
	getStatus, participants := dmHTTPGetParticipants(t, client, base+"/participants", "member-2")
	if getStatus != http.StatusOK {
		t.Fatalf("participants status = %d, want 200: %v", getStatus, participants)
	}
	hasActiveAdmin := false
	for _, p := range participants {
		isAdmin, _ := p["is_admin"].(bool)
		state, _ := p["state"].(string)
		if isAdmin && state == string(mwanachamacomm.DMStateActive) {
			hasActiveAdmin = true
		}
	}
	if hasActiveAdmin {
		t.Fatalf("expected no active admin remaining on the roster, found one: %v", participants)
	}
}

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
