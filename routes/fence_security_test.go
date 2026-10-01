package routes_test

import (
	"context"
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

// seedThread opens a conversation with admin active and every other named
// actor invited. Creating one is not a declared address — it reaches a
// gateway-internal domain — so it is seeded through the manager, and
// everything asserted afterwards crosses the mux.
func seedThread(t *testing.T, m *mwanachamacomm.CommManager, admin string, invited ...string) string {
	t.Helper()
	th, err := m.CreateThread(context.Background(),
		mwanachamacomm.DMThread{CreatedBy: admin}, invited)
	if err != nil {
		t.Fatalf("seed a conversation: %v", err)
	}
	return th.ID
}

func seedMessage(t *testing.T, m *mwanachamacomm.CommManager, threadID, sender string) string {
	t.Helper()
	msg, err := m.PostDMMessage(context.Background(), mwanachamacomm.DMMessage{
		ThreadID: threadID, SenderID: sender, PayloadCiphertext: "ct",
	})
	if err != nil {
		t.Fatalf("seed a message: %v", err)
	}
	return msg.ID
}

// An outsider must not be able to tell a conversation that exists from one
// that never did. Both answer 404 with the same message, so the status code
// cannot be used to enumerate thread ids. This is DEV-1137, and the reason
// it is pinned here is that it regressed once: the hand-written handlers
// answered 403 until a real caller went through a real gateway.
func TestARosterFenceAnswersAnOutsiderNotFoundAndNeverForbidden(t *testing.T) {
	srv, m := newCommServer(t)
	threadID := seedThread(t, m, "admin-1", "member-2")

	for _, c := range []struct{ name, method, path string }{
		{"the conversation", http.MethodGet, "/dm/threads/" + threadID},
		{"its roster", http.MethodGet, "/dm/threads/" + threadID + "/participants"},
		{"its messages", http.MethodGet, "/dm/threads/" + threadID + "/messages"},
		{"its reactions", http.MethodGet, "/dm/threads/" + threadID + "/reactions"},
	} {
		t.Run(c.name, func(t *testing.T) {
			real := call(t, srv, c.method, c.path, "outsider-9", nil)
			if real.code != http.StatusNotFound {
				t.Fatalf("an outsider reading %s = %d, want 404", c.name, real.code)
			}

			invented := call(t, srv, c.method,
				replaceID(c.path, threadID, "dm-does-not-exist"), "outsider-9", nil)
			if invented.code != http.StatusNotFound {
				t.Fatalf("a thread that never existed = %d, want 404", invented.code)
			}
			if real.message(t) != invented.message(t) {
				t.Fatalf("an existing thread answers %q and an invented one %q; the difference is an enumeration oracle",
					real.message(t), invented.message(t))
			}
		})
	}
}

// The narrower fence: being on the roster is not enough to read messages,
// only being active is. An invited-but-not-accepted caller gets the same
// 404 as an outsider.
func TestOnlyAnActiveParticipantReadsMessagesAndReactions(t *testing.T) {
	srv, m := newCommServer(t)
	threadID := seedThread(t, m, "admin-1", "member-2")

	for _, path := range []string{
		"/dm/threads/" + threadID + "/messages",
		"/dm/threads/" + threadID + "/reactions",
	} {
		if got := call(t, srv, http.MethodGet, path, "member-2", nil); got.code != http.StatusNotFound {
			t.Errorf("an invited caller reading %s = %d, want 404 until they accept", path, got.code)
		}
	}

	// The same caller, once active, reads both.
	if got := call(t, srv, http.MethodPost, "/dm/threads/"+threadID+"/accept", "member-2", nil); got.code != http.StatusOK {
		t.Fatalf("accept = %d, want 200: %s", got.code, got.body)
	}
	for _, path := range []string{
		"/dm/threads/" + threadID + "/messages",
		"/dm/threads/" + threadID + "/reactions",
	} {
		if got := call(t, srv, http.MethodGet, path, "member-2", nil); got.code != http.StatusOK {
			t.Errorf("an active caller reading %s = %d, want 200: %s", path, got.code, got.body)
		}
	}

	// An invited caller may still read the conversation and its roster,
	// which is the wider fence and a different rule.
	for _, path := range []string{
		"/dm/threads/" + threadID,
		"/dm/threads/" + threadID + "/participants",
	} {
		if got := call(t, srv, http.MethodGet, path, "admin-1", nil); got.code != http.StatusOK {
			t.Errorf("an admin reading %s = %d, want 200: %s", path, got.code, got.body)
		}
	}
}

func TestAReactionIsFencedThroughItsOwnMessagesThread(t *testing.T) {
	srv, m := newCommServer(t)
	threadID := seedThread(t, m, "admin-1", "member-2")
	messageID := seedMessage(t, m, threadID, "admin-1")

	for _, c := range []struct{ method, caller string }{
		{http.MethodPut, "outsider-9"},
		{http.MethodDelete, "outsider-9"},
		{http.MethodPut, "member-2"},
		{http.MethodDelete, "member-2"},
	} {
		var body any
		if c.method == http.MethodPut {
			body = map[string]string{"emoji": "👍"}
		}
		got := call(t, srv, c.method, "/dm/messages/"+messageID+"/reaction", c.caller, body)
		if got.code != http.StatusNotFound {
			t.Errorf("%s as %s = %d, want 404", c.method, c.caller, got.code)
		}
	}

	if got := call(t, srv, http.MethodPut, "/dm/messages/"+messageID+"/reaction", "admin-1",
		map[string]string{"emoji": "👍"}); got.code != http.StatusNoContent {
		t.Fatalf("an active participant reacting = %d, want 204: %s", got.code, got.body)
	}
	if got := call(t, srv, http.MethodDelete, "/dm/messages/"+messageID+"/reaction", "admin-1", nil); got.code != http.StatusNoContent {
		t.Fatalf("clearing own reaction = %d, want 204: %s", got.code, got.body)
	}
}

func TestAReactionMustBeShortAndPresent(t *testing.T) {
	srv, m := newCommServer(t)
	threadID := seedThread(t, m, "admin-1")
	messageID := seedMessage(t, m, threadID, "admin-1")

	for _, c := range []struct{ name, emoji string }{
		{"absent", ""},
		{"a sentence", "this is not an emoji, it is a whole opinion"},
	} {
		got := call(t, srv, http.MethodPut, "/dm/messages/"+messageID+"/reaction", "admin-1",
			map[string]string{"emoji": c.emoji})
		if got.code != http.StatusBadRequest {
			t.Errorf("%s reaction = %d, want 400: %s", c.name, got.code, got.body)
		}
	}
}

// MyAddressIndex is resolved per reader. The two owner fields and the two
// raw indexes are json:"-", so this projection is the only thing standing
// between them and somebody not party to the pair.
func TestAThreadCarriesOnlyTheReadersOwnAddressIndex(t *testing.T) {
	srv, m := newCommServer(t)

	mine := 3
	theirs := 7
	th, err := m.CreateThread(context.Background(), mwanachamacomm.DMThread{
		CreatedBy:             "admin-1",
		OpenedViaAddressOwner: "admin-1",
		OpenedViaAddressIndex: &mine,
		OpenedViaAddressHash:  []byte("hash:opened"),
		SentFromAddressOwner:  "member-2",
		SentFromAddressIndex:  &theirs,
	}, []string{"member-2"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, c := range []struct {
		caller string
		want   int
	}{
		{"admin-1", mine},
		{"member-2", theirs},
	} {
		got := call(t, srv, http.MethodGet, "/dm/threads/"+th.ID, c.caller, nil)
		if got.code != http.StatusOK {
			t.Fatalf("%s reading the thread = %d: %s", c.caller, got.code, got.body)
		}
		var out struct {
			MyAddressIndex    *int `json:"my_address_index"`
			OpenedViaAddress  bool `json:"opened_via_address"`
			OpenedViaIndexRaw *int `json:"opened_via_address_index"`
			SentFromIndexRaw  *int `json:"sent_from_address_index"`
		}
		got.decode(t, &out)
		if out.MyAddressIndex == nil || *out.MyAddressIndex != c.want {
			t.Errorf("%s sees my_address_index %v, want %d", c.caller, out.MyAddressIndex, c.want)
		}
		if out.OpenedViaIndexRaw != nil || out.SentFromIndexRaw != nil {
			t.Errorf("%s sees a raw address index, which is never serialised: %s", c.caller, got.body)
		}
		if !out.OpenedViaAddress {
			t.Errorf("%s sees opened_via_address false, want the derived true", c.caller)
		}
	}
}

// The owner, the publisher and the handset are session facts. A body
// claiming any of them is overwritten rather than refused — DEV-1265.
func TestPublishingADeviceKeyIgnoresClaimedProvenance(t *testing.T) {
	srv, _ := newCommServer(t)

	req := map[string]any{
		"key_id":       "k-1",
		"public_key":   "pk",
		"published_by": "somebody-else",
		"device_id":    "not-my-handset",
	}
	raw := callWithDevice(t, srv, http.MethodPost, "/dm/device-keys", "actor-1", "handset-9", req)
	if raw.code != http.StatusCreated {
		t.Fatalf("publish = %d, want 201: %s", raw.code, raw.body)
	}

	var out struct {
		ActorID     string `json:"actor_id"`
		PublishedBy string `json:"published_by"`
		DeviceID    string `json:"device_id"`
	}
	raw.decode(t, &out)
	if out.ActorID != "actor-1" || out.PublishedBy != "actor-1" {
		t.Errorf("owner/publisher = %q/%q, want the session's actor-1", out.ActorID, out.PublishedBy)
	}
	if out.DeviceID != "handset-9" {
		t.Errorf("device = %q, want the session's handset-9", out.DeviceID)
	}
}

func replaceID(path, from, to string) string {
	out := make([]byte, 0, len(path))
	i := 0
	for i < len(path) {
		if i+len(from) <= len(path) && path[i:i+len(from)] == from {
			out = append(out, to...)
			i += len(from)
			continue
		}
		out = append(out, path[i])
		i++
	}
	return string(out)
}
