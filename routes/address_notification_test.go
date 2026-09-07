package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

func TestListMyAddressesRoute(t *testing.T) {
	db, tables := newRouteTestDB(t)
	addrs, err := mwanachamacomm.NewAddressStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewAddressStore: %v", err)
	}
	if _, err := addrs.Publish(context.Background(), mwanachamacomm.Address{MemberID: "m-1", Hash: []byte("h1"), Index: 0}); err != nil {
		t.Fatalf("seed publish: %v", err)
	}

	identity := testIdentity{callerID: "m-1"}
	req := httptest.NewRequest(http.MethodGet, "/v1/dm/addresses", nil)
	w := httptest.NewRecorder()
	routes.ListMyAddresses(addrs, identity)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	out := decodeJSON[[]mwanachamacomm.AddressMine](t, w)
	if len(out) != 1 || out[0].Index != 0 {
		t.Fatalf("addresses = %+v, want one row at index 0", out)
	}
}

func TestRetireAddressRoute(t *testing.T) {
	db, tables := newRouteTestDB(t)
	addrs, err := mwanachamacomm.NewAddressStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewAddressStore: %v", err)
	}
	if _, err := addrs.Publish(context.Background(), mwanachamacomm.Address{MemberID: "m-1", Hash: []byte("h1"), Index: 0}); err != nil {
		t.Fatalf("seed publish: %v", err)
	}

	identity := testIdentity{callerID: "m-1"}
	req := httptest.NewRequest(http.MethodPost, "/v1/dm/addresses/0/retire", nil)
	req.SetPathValue("index", "0")
	w := httptest.NewRecorder()
	routes.RetireAddress(addrs, identity)(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", w.Code, w.Body.String())
	}

	// A second retire of the same index is now a miss.
	req2 := httptest.NewRequest(http.MethodPost, "/v1/dm/addresses/0/retire", nil)
	req2.SetPathValue("index", "0")
	w2 := httptest.NewRecorder()
	routes.RetireAddress(addrs, identity)(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("second retire status = %d, want 404: %s", w2.Code, w2.Body.String())
	}
}

func TestRetireAddressRouteRejectsBadIndex(t *testing.T) {
	db, tables := newRouteTestDB(t)
	addrs, err := mwanachamacomm.NewAddressStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewAddressStore: %v", err)
	}
	identity := testIdentity{callerID: "m-1"}
	req := httptest.NewRequest(http.MethodPost, "/v1/dm/addresses/nope/retire", nil)
	req.SetPathValue("index", "nope")
	w := httptest.NewRecorder()
	routes.RetireAddress(addrs, identity)(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestNotificationRoutesEndToEnd(t *testing.T) {
	db, tables := newRouteTestDB(t)
	notif, err := mwanachamacomm.NewNotificationStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewNotificationStore: %v", err)
	}
	if _, err := notif.Raise(context.Background(), mwanachamacomm.Notification{
		MemberID:    "m-1",
		Category:    mwanachamacomm.CategoryResults,
		Event:       mwanachamacomm.EventResultsPublished,
		SubjectKind: mwanachamacomm.SubjectSurvey,
		SubjectID:   "survey-1",
	}); err != nil {
		t.Fatalf("seed raise: %v", err)
	}

	identity := testIdentity{callerID: "m-1"}

	// List: one row, badge 1.
	listReq := httptest.NewRequest(http.MethodGet, "/v1/notifications", nil)
	w := httptest.NewRecorder()
	routes.ListNotifications(notif, identity)(w, listReq)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var listOut struct {
		Notifications []json.RawMessage `json:"notifications"`
		Unread        int               `json:"unread"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listOut); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listOut.Notifications) != 1 || listOut.Unread != 1 {
		t.Fatalf("list = %+v, want 1 row and unread=1", listOut)
	}

	var one struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listOut.Notifications[0], &one); err != nil {
		t.Fatalf("decode row: %v", err)
	}

	// Mark it read.
	markReq := httptest.NewRequest(http.MethodPost, "/v1/notifications/read", jsonBody(t, map[string]any{"ids": []string{one.ID}}))
	w2 := httptest.NewRecorder()
	routes.MarkNotificationsRead(notif, identity)(w2, markReq)
	if w2.Code != http.StatusOK {
		t.Fatalf("mark-read status = %d, want 200: %s", w2.Code, w2.Body.String())
	}

	unreadReq := httptest.NewRequest(http.MethodGet, "/v1/notifications/unread", nil)
	w3 := httptest.NewRecorder()
	routes.NotificationUnreadCount(notif, identity)(w3, unreadReq)
	var badge struct {
		Unread int `json:"unread"`
	}
	if err := json.NewDecoder(w3.Body).Decode(&badge); err != nil {
		t.Fatalf("decode badge: %v", err)
	}
	if badge.Unread != 0 {
		t.Fatalf("unread after mark-read = %d, want 0", badge.Unread)
	}
}

func TestSetNotificationPreferenceRoute(t *testing.T) {
	db, tables := newRouteTestDB(t)
	notif, err := mwanachamacomm.NewNotificationStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewNotificationStore: %v", err)
	}
	identity := testIdentity{callerID: "m-1"}

	// Muting chat succeeds.
	req := httptest.NewRequest(http.MethodPut, "/v1/notification-preferences/chat", jsonBody(t, map[string]any{"muted": true}))
	req.SetPathValue("category", "chat")
	w := httptest.NewRecorder()
	routes.SetNotificationPreference(notif, identity)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}

	// Muting survey (exempt) is refused with 409.
	req2 := httptest.NewRequest(http.MethodPut, "/v1/notification-preferences/survey", jsonBody(t, map[string]any{"muted": true}))
	req2.SetPathValue("category", "survey")
	w2 := httptest.NewRecorder()
	routes.SetNotificationPreference(notif, identity)(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("exempt category status = %d, want 409: %s", w2.Code, w2.Body.String())
	}

	// Unknown category is a 404.
	req3 := httptest.NewRequest(http.MethodPut, "/v1/notification-preferences/nonsense", jsonBody(t, map[string]any{"muted": true}))
	req3.SetPathValue("category", "nonsense")
	w3 := httptest.NewRecorder()
	routes.SetNotificationPreference(notif, identity)(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("unknown category status = %d, want 404: %s", w3.Code, w3.Body.String())
	}

	// listNotificationPreferences shows all seven, chat muted, others not set.
	listReq := httptest.NewRequest(http.MethodGet, "/v1/notification-preferences", nil)
	w4 := httptest.NewRecorder()
	routes.ListNotificationPreferences(notif, identity)(w4, listReq)
	var out struct {
		Preferences []struct {
			Category string `json:"category"`
			Muted    bool   `json:"muted"`
			Set      bool   `json:"set"`
			Exempt   bool   `json:"exempt"`
		} `json:"preferences"`
	}
	if err := json.NewDecoder(w4.Body).Decode(&out); err != nil {
		t.Fatalf("decode preferences: %v", err)
	}
	if len(out.Preferences) != len(mwanachamacomm.NotificationCategories()) {
		t.Fatalf("preferences = %+v, want %d rows", out.Preferences, len(mwanachamacomm.NotificationCategories()))
	}
	for _, p := range out.Preferences {
		if p.Category == "chat" && (!p.Muted || !p.Set) {
			t.Fatalf("chat preference = %+v, want muted+set", p)
		}
		if p.Category == "survey" && !p.Exempt {
			t.Fatalf("survey preference = %+v, want exempt", p)
		}
	}
}
