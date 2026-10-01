package routes_test

import (
	"context"
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func TestChatActivityReadsTheWholeQuery(t *testing.T) {
	srv, m := newCommServer(t)
	if _, err := m.PostChatMessage(context.Background(), mwanachamacomm.ChatMessage{
		StructureID: "ward-1", AuthorID: "a-1", Body: "hi",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got := call(t, srv, http.MethodGet, "/chat/activity?structure_id=ward-1", "a-1", nil)
	if got.code != http.StatusOK {
		t.Fatalf("activity = %d: %s", got.code, got.body)
	}
	var page struct {
		Rows []struct {
			StructureID string `json:"structure_id"`
			Messages    int    `json:"messages"`
		} `json:"rows"`
		Messages int `json:"messages"`
	}
	got.decode(t, &page)
	if page.Messages != 1 || len(page.Rows) != 1 || page.Rows[0].StructureID != "ward-1" {
		t.Fatalf("activity = %+v, want one ward with one message", page)
	}

	if bad := call(t, srv, http.MethodGet, "/chat/activity?limit=nope", "a-1", nil); bad.code != http.StatusBadRequest {
		t.Fatalf("a non-numeric limit = %d, want 400", bad.code)
	}
}

func TestTheRosterRulesHoldOverTheMux(t *testing.T) {
	srv, m := newCommServer(t)
	threadID := seedThread(t, m, "admin-1", "member-2")

	t.Run("a self-kick is refused", func(t *testing.T) {
		got := call(t, srv, http.MethodPost, "/dm/threads/"+threadID+"/kick", "admin-1",
			map[string]string{"actor_id": "admin-1"})
		if got.code != http.StatusConflict {
			t.Fatalf("self-kick = %d, want 409: %s", got.code, got.body)
		}
	})

	t.Run("inviting somebody already active is a conflict", func(t *testing.T) {
		if got := call(t, srv, http.MethodPost, "/dm/threads/"+threadID+"/accept", "member-2", nil); got.code != http.StatusOK {
			t.Fatalf("accept = %d: %s", got.code, got.body)
		}
		got := call(t, srv, http.MethodPost, "/dm/threads/"+threadID+"/invite", "admin-1",
			map[string]string{"actor_id": "member-2"})
		if got.code != http.StatusConflict {
			t.Fatalf("re-inviting an active member = %d, want 409: %s", got.code, got.body)
		}
	})

	t.Run("the last admin may not leave while others are active", func(t *testing.T) {
		got := call(t, srv, http.MethodPost, "/dm/threads/"+threadID+"/leave", "admin-1", nil)
		if got.code != http.StatusConflict {
			t.Fatalf("last-admin leave = %d, want 409: %s", got.code, got.body)
		}

		if p := call(t, srv, http.MethodPost, "/dm/threads/"+threadID+"/promote", "admin-1",
			map[string]string{"actor_id": "member-2"}); p.code != http.StatusOK {
			t.Fatalf("promote = %d: %s", p.code, p.body)
		}
		if l := call(t, srv, http.MethodPost, "/dm/threads/"+threadID+"/leave", "admin-1", nil); l.code != http.StatusNoContent {
			t.Fatalf("leave once a successor exists = %d, want 204: %s", l.code, l.body)
		}
	})

	t.Run("a non-admin roster write is refused", func(t *testing.T) {
		other := seedThread(t, m, "admin-9", "member-8")
		got := call(t, srv, http.MethodPost, "/dm/threads/"+other+"/invite", "member-8",
			map[string]string{"actor_id": "member-7"})
		if got.code != http.StatusForbidden {
			t.Fatalf("a non-admin inviting = %d, want 403: %s", got.code, got.body)
		}
	})

	t.Run("promoting a non-participant is not found", func(t *testing.T) {
		other := seedThread(t, m, "admin-5")
		got := call(t, srv, http.MethodPost, "/dm/threads/"+other+"/promote", "admin-5",
			map[string]string{"actor_id": "nobody"})
		if got.code != http.StatusNotFound {
			t.Fatalf("promoting a non-participant = %d, want 404: %s", got.code, got.body)
		}
	})
}

// CM31: an unexpected error must not arrive as a forbidden. Before the
// route table was declared, dmStatusFor's default arm returned 403, so a
// dropped connection on any DM route told the caller the request was
// understood and denied. Now the mapped statuses are mapped and everything
// else is the dispatcher's own 500.
func TestAnUnmappedFailureIsNotReportedAsForbidden(t *testing.T) {
	db, m := newCommTestDB(t)
	srv := newServer(t, m, testMount())

	threadID := seedThread(t, m, "admin-1")
	if err := db.Exec("drop table " + m.Spec().TableFor(mustObject(t, m, "dm_participant"))).Error; err != nil {
		t.Fatalf("drop the roster table: %v", err)
	}

	got := call(t, srv, http.MethodGet, "/dm/threads/"+threadID+"/participants", "admin-1", nil)
	if got.code == http.StatusForbidden {
		t.Fatalf("a missing table answered 403, which tells the caller they were denied rather than that it broke")
	}
	if got.code != http.StatusInternalServerError {
		t.Fatalf("a missing table = %d, want 500", got.code)
	}
	if msg := got.message(t); msg != "internal error" {
		t.Fatalf("the body is %q; a failure nobody mapped is redacted, never echoed", msg)
	}
}

func TestMyAddressesAndRetire(t *testing.T) {
	srv, m := newCommServer(t)
	if _, err := m.Publish(context.Background(), mwanachamacomm.Address{
		ActorID: "m-1", Hash: []byte("hash:a1"), AddressIndex: 0,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got := call(t, srv, http.MethodGet, "/dm/addresses", "m-1", nil)
	if got.code != http.StatusOK {
		t.Fatalf("list = %d: %s", got.code, got.body)
	}
	var mine []struct {
		Index int `json:"address_index"`
	}
	got.decode(t, &mine)
	if len(mine) != 1 || mine[0].Index != 0 {
		t.Fatalf("addresses = %+v, want one at index 0", mine)
	}

	if r := call(t, srv, http.MethodPost, "/dm/addresses/0/retire", "m-1", nil); r.code != http.StatusNoContent {
		t.Fatalf("retire = %d, want 204: %s", r.code, r.body)
	}
	if again := call(t, srv, http.MethodPost, "/dm/addresses/0/retire", "m-1", nil); again.code != http.StatusNotFound {
		t.Fatalf("retiring twice = %d, want 404", again.code)
	}
	if bad := call(t, srv, http.MethodPost, "/dm/addresses/nope/retire", "m-1", nil); bad.code != http.StatusBadRequest {
		t.Fatalf("a non-numeric index = %d, want 400", bad.code)
	}
	if other := call(t, srv, http.MethodPost, "/dm/addresses/0/retire", "somebody-else", nil); other.code != http.StatusNotFound {
		t.Fatalf("retiring somebody else's address = %d, want 404", other.code)
	}
}

func TestNotificationRoutesEndToEnd(t *testing.T) {
	srv, m := newCommServer(t)
	raised, err := m.Raise(context.Background(), mwanachamacomm.Notification{
		ActorID: "m-1", Category: mwanachamacomm.CategorySurvey,
		Event: mwanachamacomm.EventSurveyReminder,
		// SubjectKind and SubjectID address the thing the notice is about.
		SubjectKind: mwanachamacomm.SubjectSurvey, SubjectID: "survey-1",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	list := call(t, srv, http.MethodGet, "/notifications", "m-1", nil)
	if list.code != http.StatusOK {
		t.Fatalf("list = %d: %s", list.code, list.body)
	}

	unread := call(t, srv, http.MethodGet, "/notifications/unread", "m-1", nil)
	var count struct {
		Unread int `json:"unread"`
	}
	unread.decode(t, &count)
	if count.Unread != 1 {
		t.Fatalf("unread = %d, want 1", count.Unread)
	}

	marked := call(t, srv, http.MethodPost, "/notifications/read", "m-1",
		map[string]any{"ids": []string{raised.ID}})
	var moved struct {
		Marked int `json:"marked"`
	}
	marked.decode(t, &moved)
	if moved.Marked != 1 {
		t.Fatalf("marked = %d, want 1: %s", moved.Marked, marked.body)
	}

	if bad := call(t, srv, http.MethodPost, "/notifications/read", "m-1",
		map[string]any{"ids": []string{raised.ID}, "read_at": "not-a-time"}); bad.code != http.StatusBadRequest {
		t.Fatalf("a malformed read_at = %d, want 400: %s", bad.code, bad.body)
	}

	// Somebody else's notification is not theirs to mark.
	other := call(t, srv, http.MethodPost, "/notifications/read", "m-2",
		map[string]any{"ids": []string{raised.ID}})
	other.decode(t, &moved)
	if moved.Marked != 0 {
		t.Fatalf("another caller marked %d of somebody else's notifications", moved.Marked)
	}
}

// CM30: the category vocabulary is the mounted domain's, not a Go slice.
// An undeclared category is a not-found; a declared one that the domain
// marks exempt is a conflict with the reason.
func TestSettingAPreferenceAsksTheDeclaredVocabulary(t *testing.T) {
	srv, _ := newCommServer(t)

	if got := call(t, srv, http.MethodPut, "/notification-preferences/chat", "m-1",
		map[string]bool{"muted": true}); got.code != http.StatusOK {
		t.Fatalf("muting a declared category = %d, want 200: %s", got.code, got.body)
	}

	if got := call(t, srv, http.MethodPut, "/notification-preferences/attendance", "m-1",
		map[string]bool{"muted": true}); got.code != http.StatusNotFound {
		t.Fatalf("a category civic does not declare = %d, want 404: %s", got.code, got.body)
	}

	for _, exempt := range []string{"survey", "security"} {
		got := call(t, srv, http.MethodPut, "/notification-preferences/"+exempt, "m-1",
			map[string]bool{"muted": true})
		if got.code != http.StatusConflict {
			t.Fatalf("muting %s = %d, want 409: %s", exempt, got.code, got.body)
		}
	}

	list := call(t, srv, http.MethodGet, "/notification-preferences", "m-1", nil)
	if list.code != http.StatusOK {
		t.Fatalf("list preferences = %d: %s", list.code, list.body)
	}
}

func TestTheReportQueueIsScopedToItsStructure(t *testing.T) {
	srv, m := newCommServer(t)
	if _, err := m.FileReport(context.Background(), mwanachamacomm.Report{
		MessageID: "msg-1", StructureID: "ward-1", ReportedBy: "m-1",
		Reason: mwanachamacomm.ReportReasonAbuse,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, c := range []struct {
		structure string
		want      int
	}{{"ward-1", 1}, {"ward-2", 0}} {
		got := call(t, srv, http.MethodGet, "/groups/"+c.structure+"/moderation/reports", "mod-1", nil)
		if got.code != http.StatusOK {
			t.Fatalf("queue = %d: %s", got.code, got.body)
		}
		var out []struct {
			MessageID string `json:"message_id"`
		}
		got.decode(t, &out)
		if len(out) != c.want {
			t.Errorf("%s queue has %d reports, want %d", c.structure, len(out), c.want)
		}
	}

	byMessage := call(t, srv, http.MethodGet, "/groups/ward-1/moderation/messages/msg-1/reports", "mod-1", nil)
	if byMessage.code != http.StatusOK {
		t.Fatalf("reports for a message = %d: %s", byMessage.code, byMessage.body)
	}
}

func TestDeviceKeyLookupReturnsOnlyLiveKeys(t *testing.T) {
	srv, _ := newCommServer(t)

	first := callWithDevice(t, srv, http.MethodPost, "/dm/device-keys", "m-1", "phone-1",
		map[string]any{"public_key": "pk1"})
	if first.code != http.StatusCreated {
		t.Fatalf("publish = %d: %s", first.code, first.body)
	}
	second := callWithDevice(t, srv, http.MethodPost, "/dm/device-keys", "m-1", "phone-1",
		map[string]any{"public_key": "pk2"})
	if second.code != http.StatusCreated {
		t.Fatalf("republish = %d: %s", second.code, second.body)
	}

	got := call(t, srv, http.MethodPost, "/dm/device-keys/lookup", "m-9",
		map[string]any{"actor_ids": []string{"m-1"}})
	if got.code != http.StatusOK {
		t.Fatalf("lookup = %d: %s", got.code, got.body)
	}
	var keys []struct {
		PublicKey string `json:"public_key"`
	}
	got.decode(t, &keys)
	if len(keys) != 1 || keys[0].PublicKey != "pk2" {
		t.Fatalf("lookup returned %+v, want only the newest key for the handset", keys)
	}
}
