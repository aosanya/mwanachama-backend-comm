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

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

// testIdentity is a fixed [routes.Identity] — a stand-in for the gateway's
// real session-derived identity, the same role a caller-supplied test
// double plays for actor's HierarchyChecker.
type testIdentity struct {
	callerID string
	deviceID string
}

func (i testIdentity) CallerID(r *http.Request) string       { return i.callerID }
func (i testIdentity) CallerDeviceID(r *http.Request) string { return i.deviceID }

// noopActWriter never gets exercised by these tests (ModerationRoutes only
// covers reads) but ModerationStore's constructor requires a non-nil one.
type noopActWriter struct{}

func (noopActWriter) WriteAct(ctx context.Context, tx *sql.Tx, e mwanachamacomm.ActEntry) error {
	return nil
}

func newRouteTestDB(t *testing.T) (*gorm.DB, mwanachamacomm.TableNames) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	tables := mwanachamacomm.DefaultTableNames()
	if err := mwanachamacomm.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db, tables
}

func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return out
}

func TestChatActivityRoute(t *testing.T) {
	db, tables := newRouteTestDB(t)
	chat, err := mwanachamacomm.NewChatStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	if _, err := chat.Post(context.Background(), mwanachamacomm.ChatMessage{StructureID: "c-1", AuthorID: "a-1", Body: "hi"}); err != nil {
		t.Fatalf("seed post: %v", err)
	}

	handler := routes.ChatActivity(chat)
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/activity", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	page := decodeJSON[mwanachamacomm.ChatActivityPage](t, w)
	if page.Total != 1 || page.Messages != 1 {
		t.Fatalf("page = %+v, want Total=1 Messages=1", page)
	}
}

func TestChatActivityRouteRejectsBadLimit(t *testing.T) {
	db, tables := newRouteTestDB(t)
	chat, err := mwanachamacomm.NewChatStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/activity?limit=-1", nil)
	w := httptest.NewRecorder()
	routes.ChatActivity(chat)(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// TestGetDMThreadRefusesAnOutsiderWithNotFound is DEV-1137's own contract:
// a caller who is not on a thread's roster must get the exact same answer
// as a thread that was never minted — 404, never 403, or the status code
// itself becomes an enumeration oracle.
func TestGetDMThreadRefusesAnOutsiderWithNotFound(t *testing.T) {
	db, tables := newRouteTestDB(t)
	dm, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	th, err := dm.CreateThread(context.Background(), mwanachamacomm.DMThread{CreatedBy: "m-1"}, nil)
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}

	outsider := testIdentity{callerID: "m-stranger"}
	req := httptest.NewRequest(http.MethodGet, "/v1/dm/threads/"+th.ID, nil)
	req.SetPathValue("threadID", th.ID)
	w := httptest.NewRecorder()
	routes.GetDMThread(dm, outsider)(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("outsider status = %d, want 404: %s", w.Code, w.Body.String())
	}

	absentReq := httptest.NewRequest(http.MethodGet, "/v1/dm/threads/dm-does-not-exist", nil)
	absentReq.SetPathValue("threadID", "dm-does-not-exist")
	w2 := httptest.NewRecorder()
	routes.GetDMThread(dm, outsider)(w2, absentReq)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("absent thread status = %d, want 404: %s", w2.Code, w2.Body.String())
	}
	if w.Body.String() != w2.Body.String() {
		t.Fatalf("outsider body %q must be byte-identical to absent-thread body %q, or the status/body pair becomes an oracle", w.Body.String(), w2.Body.String())
	}
}

func TestPublishDMDeviceKeyIgnoresClaimedProvenance(t *testing.T) {
	db, tables := newRouteTestDB(t)
	dm, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	identity := testIdentity{callerID: "m-1", deviceID: "device-1"}
	req := httptest.NewRequest(http.MethodPost, "/v1/dm/device-keys", jsonBody(t, map[string]string{
		"public_key":   "pk-1",
		"published_by": "actor-someone-else",
		"device_id":    "device-someone-else",
	}))
	w := httptest.NewRecorder()
	routes.PublishDMDeviceKey(dm, identity)(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}
	out := decodeJSON[mwanachamacomm.DMDeviceKey](t, w)
	if out.PublishedBy != "m-1" || out.DeviceID != "device-1" {
		t.Fatalf("provenance = %+v, want the session's (m-1/device-1), not the claimed one", out)
	}
}

func TestDMInviteAndAcceptRoutes(t *testing.T) {
	db, tables := newRouteTestDB(t)
	dm, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	th, err := dm.CreateThread(context.Background(), mwanachamacomm.DMThread{CreatedBy: "m-1"}, nil)
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}

	asAdmin := testIdentity{callerID: "m-1"}
	inviteReq := httptest.NewRequest(http.MethodPost, "/v1/dm/threads/"+th.ID+"/invite", jsonBody(t, map[string]string{"actor_id": "m-2"}))
	inviteReq.SetPathValue("threadID", th.ID)
	w := httptest.NewRecorder()
	routes.InviteDM(dm, asAdmin)(w, inviteReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("invite status = %d, want 201: %s", w.Code, w.Body.String())
	}

	asInvitee := testIdentity{callerID: "m-2"}
	acceptReq := httptest.NewRequest(http.MethodPost, "/v1/dm/threads/"+th.ID+"/accept", nil)
	acceptReq.SetPathValue("threadID", th.ID)
	w2 := httptest.NewRecorder()
	routes.AcceptDM(dm, asInvitee)(w2, acceptReq)
	if w2.Code != http.StatusOK {
		t.Fatalf("accept status = %d, want 200: %s", w2.Code, w2.Body.String())
	}
	p := decodeJSON[mwanachamacomm.DMParticipant](t, w2)
	if p.State != mwanachamacomm.DMStateActive {
		t.Fatalf("state = %q, want active", p.State)
	}
}

func TestDMInviteRouteMapsAlreadyActiveTo409(t *testing.T) {
	db, tables := newRouteTestDB(t)
	dm, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	ctx := context.Background()
	th, err := dm.CreateThread(ctx, mwanachamacomm.DMThread{CreatedBy: "m-1"}, []string{"m-2"})
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	if _, err := dm.Accept(ctx, th.ID, "m-2"); err != nil {
		t.Fatalf("seed accept: %v", err)
	}

	identity := testIdentity{callerID: "m-1"}
	req := httptest.NewRequest(http.MethodPost, "/v1/dm/threads/"+th.ID+"/invite", jsonBody(t, map[string]string{"actor_id": "m-2"}))
	req.SetPathValue("threadID", th.ID)
	w := httptest.NewRecorder()
	routes.InviteDM(dm, identity)(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", w.Code, w.Body.String())
	}
}

func TestListDMThreadsRouteAppliesForCaller(t *testing.T) {
	db, tables := newRouteTestDB(t)
	dm, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	if _, err := dm.CreateThread(context.Background(), mwanachamacomm.DMThread{CreatedBy: "m-1"}, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	identity := testIdentity{callerID: "m-1"}
	req := httptest.NewRequest(http.MethodGet, "/v1/dm/threads", nil)
	w := httptest.NewRecorder()
	routes.ListDMThreads(dm, identity)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	out := decodeJSON[[]mwanachamacomm.DMThread](t, w)
	if len(out) != 1 {
		t.Fatalf("threads = %+v, want 1", out)
	}
}

func TestListReportQueueRoute(t *testing.T) {
	db, tables := newRouteTestDB(t)
	mod, err := mwanachamacomm.NewModerationStore(db, tables, nil, noopActWriter{})
	if err != nil {
		t.Fatalf("NewModerationStore: %v", err)
	}
	if _, err := mod.FileReport(context.Background(), mwanachamacomm.Report{
		MessageID: "msg-1", StructureID: "ward-1", ReportedBy: "m-1", Reason: mwanachamacomm.ReportReasonAbuse, Excerpt: "...",
	}); err != nil {
		t.Fatalf("seed report: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/groups/ward-1/moderation/reports", nil)
	req.SetPathValue("groupID", "ward-1")
	w := httptest.NewRecorder()
	routes.ListReportQueue(mod)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	out := decodeJSON[[]mwanachamacomm.Report](t, w)
	if len(out) != 1 || out[0].MessageID != "msg-1" {
		t.Fatalf("reports = %+v, want one for msg-1", out)
	}
}

func jsonBody(t *testing.T, v any) io.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewReader(b)
}
