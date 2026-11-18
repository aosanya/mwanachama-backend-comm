package routes_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func handOverBody(recipient string) map[string]any {
	return map[string]any{
		"recipient_id": recipient,
		"category":     "report",
		"event":        "report_handed_over",
		"subject_kind": "report",
		"subject_id":   "report-1",
		"names":        map[string]string{"report": "Annual report"},
	}
}

func TestAHandOverRaisesOneNotificationForTheNamedPersonOnly(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseRaisable(func(e models.NotificationEvent) bool { return e == models.EventReportHandedOver })
	out := call(t, srv, http.MethodPost, "/notifications", "officer-1", handOverBody("councillor-1"))
	if out.code != http.StatusOK && out.code != http.StatusCreated {
		t.Fatalf("raise = %d: %s", out.code, out.body)
	}
	list := call(t, srv, http.MethodGet, "/notifications", "councillor-1", nil)
	var rows []map[string]json.RawMessage
	list.decode(t, &rows)
	if len(rows) != 1 || string(rows[0]["Event"]) != `"report_handed_over"` {
		t.Fatalf("recipient list = %s", list.body)
	}
	mine := call(t, srv, http.MethodGet, "/notifications", "officer-1", nil)
	var none []map[string]json.RawMessage
	mine.decode(t, &none)
	if len(none) != 0 {
		t.Fatalf("raiser list = %s, want empty", mine.body)
	}
}

func TestAHandOverOfAnEventTheHostDoesNotNameIsRefusedWith403(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseRaisable(func(e models.NotificationEvent) bool { return e == models.EventReportHandedOver })
	body := handOverBody("councillor-1")
	body["category"] = "security"
	body["event"] = "device_signed_out"
	body["subject_kind"] = "device"
	out := call(t, srv, http.MethodPost, "/notifications", "officer-1", body)
	if out.code != http.StatusForbidden {
		t.Fatalf("raise = %d: %s, want 403", out.code, out.body)
	}
}

func TestACallerCannotNameThemselvesAsSomeoneElseWhenHandingOver(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseRaisable(func(e models.NotificationEvent) bool { return e == models.EventReportHandedOver })
	body := handOverBody("councillor-1")
	body["caller_id"] = "mayor-1"
	call(t, srv, http.MethodPost, "/notifications", "officer-1", body)
	list := call(t, srv, http.MethodGet, "/notifications", "councillor-1", nil)
	var rows []map[string]json.RawMessage
	list.decode(t, &rows)
	if len(rows) != 1 {
		t.Fatalf("recipient list = %s", list.body)
	}
}
