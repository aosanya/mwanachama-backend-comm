package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func chatActivityStatusFor(t *testing.T, rawQuery string) int {
	t.Helper()
	for _, route := range RoutesFor(nil, Mount{}) {
		if route.Action != "comm.chat.activity.read" {
			continue
		}
		req := httptest.NewRequest(http.MethodGet, "/chat/activity?"+rawQuery, nil)
		rec := httptest.NewRecorder()
		route.Handler(rec, req)
		return rec.Code
	}
	t.Fatal("no route for comm.chat.activity.read")
	return 0
}

func TestChatActivityRefusesAWindowThatCannotBeAPage(t *testing.T) {
	for _, rawQuery := range []string{"limit=0", "limit=-1", "offset=-1", "limit=1&offset=-3"} {
		if got := chatActivityStatusFor(t, rawQuery); got != http.StatusBadRequest {
			t.Errorf("GET /chat/activity?%s answered %d, want 400", rawQuery, got)
		}
	}
}
