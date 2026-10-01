package routes_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/spec"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

type noopActWriter struct{}

func (noopActWriter) WriteAct(ctx context.Context, tx *sql.Tx, e mwanachamacomm.ActEntry) error {
	return nil
}

// A mounting process carries the session on the context, so these tests do
// what one does: read the two session facts off a header into the context,
// and give the mount a Caller and a Device that read them back out. Every
// assertion below therefore crosses a real mux with a real caller, which is
// the one thing this package's old fixtures did not do — both of the bugs
// in this repo's CLAUDE.md escaped because the handlers were called in Go.
type ctxKey string

const (
	callerKey ctxKey = "caller"
	deviceKey ctxKey = "device"

	callerHeader = "X-Test-Caller"
	deviceHeader = "X-Test-Device"
)

func session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), callerKey, r.Header.Get(callerHeader))
		ctx = context.WithValue(ctx, deviceKey, r.Header.Get(deviceHeader))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func fromCtx(key ctxKey) dispatch.Caller {
	return func(ctx context.Context) string {
		v, _ := ctx.Value(key).(string)
		return v
	}
}

func testMount() routes.Mount {
	return routes.Mount{Caller: fromCtx(callerKey), Device: fromCtx(deviceKey)}
}

func newCommTestDB(t *testing.T) (*gorm.DB, *mwanachamacomm.CommManager) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB(): %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	s, err := mwanachamacomm.SpecFor("mwanachama")
	if err != nil {
		t.Fatalf("SpecFor: %v", err)
	}
	if err := mwanachamacomm.Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	m, err := mwanachamacomm.NewCommManager(db, s, nil, noopActWriter{}, "")
	if err != nil {
		t.Fatalf("NewCommManager: %v", err)
	}
	return db, m
}

func newServer(t *testing.T, m *mwanachamacomm.CommManager, mount routes.Mount) *httptest.Server {
	t.Helper()
	built, err := routes.BuildFor(m, mount)
	if err != nil {
		t.Fatalf("BuildFor: %v", err)
	}
	mux := http.NewServeMux()
	for _, rt := range built {
		mux.Handle(rt.Pattern(""), rt.Handler)
	}
	srv := httptest.NewServer(session(mux))
	t.Cleanup(srv.Close)
	return srv
}

func newCommServer(t *testing.T) (*httptest.Server, *mwanachamacomm.CommManager) {
	t.Helper()
	_, m := newCommTestDB(t)
	return newServer(t, m, testMount()), m
}

type reply struct {
	code int
	body []byte
}

func (r reply) decode(t *testing.T, out any) {
	t.Helper()
	if err := json.Unmarshal(r.body, out); err != nil {
		t.Fatalf("decode %q: %v", r.body, err)
	}
}

func (r reply) message(t *testing.T) string {
	t.Helper()
	var out struct {
		Error string `json:"error"`
	}
	if len(r.body) == 0 {
		return ""
	}
	_ = json.Unmarshal(r.body, &out)
	return out.Error
}

func call(t *testing.T, srv *httptest.Server, method, path, caller string, body any) reply {
	t.Helper()
	return callWithDevice(t, srv, method, path, caller, "", body)
}

func callWithDevice(t *testing.T, srv *httptest.Server, method, path, caller, device string, body any) reply {
	t.Helper()
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, srv.URL+path, payload)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(callerHeader, caller)
	if device != "" {
		req.Header.Set(deviceHeader, device)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return reply{code: res.StatusCode, body: raw}
}

func TestEveryDeclaredAddressIsReachable(t *testing.T) {
	built, err := routes.Shape()
	if err != nil {
		t.Fatalf("Shape: %v", err)
	}
	if len(built) != 25 {
		t.Fatalf("the table declares %d addresses, want comm's 25", len(built))
	}
	for _, rt := range built {
		if rt.Action == "" {
			t.Errorf("%s %s carries no gating action, so a mount could not gate it", rt.Method, rt.Path)
		}
	}
}

// Nothing in comm is anonymous: every address reads or writes somebody's
// own conversations, addresses or notifications. The allowlist names what
// is public, so an operation added later and not named arrives gated.
func TestNothingIsAnonymous(t *testing.T) {
	if got := routes.Table.AnonymousActions(); len(got) != 0 {
		t.Fatalf("AnonymousActions = %v, want none", got)
	}
	_, m := newCommTestDB(t)
	if public := routes.PublicRoutes(m); len(public) != 0 {
		t.Fatalf("%d routes answer without a caller", len(public))
	}
}

func TestEverySentinelTheSpecMapsIsSupplied(t *testing.T) {
	missing, err := routes.Table.UnmappedSentinels()
	if err != nil {
		t.Fatalf("UnmappedSentinels: %v", err)
	}
	if len(missing) > 0 {
		t.Fatalf("the spec maps %v to a status and no sentinel was supplied", missing)
	}
	unknown, err := routes.Table.UnknownAnonymousActions()
	if err != nil {
		t.Fatalf("UnknownAnonymousActions: %v", err)
	}
	if len(unknown) > 0 {
		t.Fatalf("anonymous actions no operation declares: %v", unknown)
	}
}

func TestAnAuthorizerRefusalGatesEveryAddress(t *testing.T) {
	_, m := newCommTestDB(t)
	refuse := func(ctx context.Context, action string) error { return dispatch.ErrForbidden }
	srv := newServer(t, m, routes.Mount{
		Caller: fromCtx(callerKey), Device: fromCtx(deviceKey), Authorize: refuse,
	})

	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/dm/threads"},
		{http.MethodGet, "/notifications"},
		{http.MethodGet, "/chat/activity"},
		{http.MethodGet, "/dm/addresses"},
		{http.MethodGet, "/groups/ward-1/moderation/reports"},
	} {
		if got := call(t, srv, c.method, c.path, "m-1", nil); got.code != http.StatusForbidden {
			t.Errorf("%s %s = %d with an authorizer that refuses, want 403", c.method, c.path, got.code)
		}
	}
}

func mustObject(t *testing.T, m *mwanachamacomm.CommManager, role string) spec.Object {
	t.Helper()
	o, ok := m.Spec().ByRole(role)
	if !ok {
		t.Fatalf("role %q fills nothing", role)
	}
	return o
}
