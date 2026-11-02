package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func TestTheNotificationListAnswersNamesAsAnObjectForARowRaisedWithNone(t *testing.T) {
	srv, m := newCommServer(t)
	if _, err := m.Raise(context.Background(), mwanachamacomm.Notification{
		ActorID: "m-1", Category: mwanachamacomm.CategorySurvey,
		Event:       mwanachamacomm.EventSurveyReminder,
		SubjectKind: mwanachamacomm.SubjectSurvey, SubjectID: "survey-1",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	list := call(t, srv, http.MethodGet, "/notifications", "m-1", nil)
	if list.code != http.StatusOK {
		t.Fatalf("list = %d: %s", list.code, list.body)
	}
	var rows []map[string]json.RawMessage
	list.decode(t, &rows)
	if len(rows) != 1 {
		t.Fatalf("list = %s, want one row", list.body)
	}
	if names := string(rows[0]["Names"]); names != "{}" {
		t.Errorf("Names answers %s, want {}: %s", names, list.body)
	}
}
