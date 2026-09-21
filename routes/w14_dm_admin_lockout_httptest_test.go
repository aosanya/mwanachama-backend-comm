package routes_test

// Upgrades W14's pin from a direct-DMStore call to a real
// httptest.NewServer + routes.DMRoutes(...) mux, driven with a real
// http.Client over real HTTP — this repo's own routes/doc.go documents a
// preference for real-mux tests over calling DMStore/ChatStore/
// ModerationStore directly, and w14_dm_admin_lockout_test.go (still kept,
// unmodified) only ever called the store's methods in Go, never through
// the HTTP boundary a real caller (the gateway, ultimately a Flutter
// client) actually goes through. This file covers the two lockout
// shapes — self-kick, and an ordinary admin Leave stranding a
// remaining participant — but every roster mutation and every attempted
// recovery is a real POST against a real *http.Server, with the caller's
// identity carried the only way a stateless HTTP boundary can carry it
// between separate requests: a header, read by a per-request
// routes.Identity implementation (headerIdentity, below) exactly the way a
// mounting gateway's own session-derived Identity would read a cookie.
//
// Board row W14 is fixed: both shapes are now refused (409) over real HTTP.

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

// TestHTTP_W14_SelfKickRefused: the sole admin cannot kick themselves.
func TestHTTP_W14_SelfKickRefused(t *testing.T) {
	srv, dm := newDMTestServer(t)
	client := srv.Client()

	th, err := dm.CreateThread(context.Background(), mwanachamacomm.DMThread{CreatedBy: "admin-1"}, nil)
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	base := srv.URL + "/v1/dm/threads/" + th.ID

	status, body := dmHTTPCall(t, client, http.MethodPost, base+"/kick", "admin-1", map[string]string{"actor_id": "admin-1"})
	if status != http.StatusConflict {
		t.Fatalf("self-kick: status=%d body=%v, want 409", status, body)
	}
}

// TestHTTP_W14_LastAdminLeaveRefusedUntilSuccessorPromoted: the sole admin
// cannot leave while member-2 is active, but can once member-2 is promoted.
func TestHTTP_W14_LastAdminLeaveRefusedUntilSuccessorPromoted(t *testing.T) {
	srv, dm := newDMTestServer(t)
	client := srv.Client()

	th, err := dm.CreateThread(context.Background(), mwanachamacomm.DMThread{CreatedBy: "admin-1"}, nil)
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	base := srv.URL + "/v1/dm/threads/" + th.ID

	if status, body := dmHTTPCall(t, client, http.MethodPost, base+"/invite", "admin-1", map[string]string{"actor_id": "member-2"}); status != http.StatusCreated {
		t.Fatalf("invite status = %d, want 201: %v", status, body)
	}
	if status, body := dmHTTPCall(t, client, http.MethodPost, base+"/accept", "member-2", nil); status != http.StatusOK {
		t.Fatalf("accept status = %d, want 200: %v", status, body)
	}

	if status, body := dmHTTPCall(t, client, http.MethodPost, base+"/leave", "admin-1", nil); status != http.StatusConflict {
		t.Fatalf("last-admin leave: status=%d body=%v, want 409", status, body)
	}
	if status, body := dmHTTPCall(t, client, http.MethodPost, base+"/promote", "admin-1", map[string]string{"actor_id": "member-2"}); status != http.StatusOK {
		t.Fatalf("promote member-2: status=%d body=%v, want 200", status, body)
	}
	if status, body := dmHTTPCall(t, client, http.MethodPost, base+"/leave", "admin-1", nil); status != http.StatusNoContent {
		t.Fatalf("leave after promotion: status=%d body=%v, want 204", status, body)
	}
}
