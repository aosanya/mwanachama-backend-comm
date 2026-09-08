package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDMParticipantStateValues(t *testing.T) {
	cases := map[DMParticipantState]string{
		DMStateInvited: "invited",
		DMStateActive:  "active",
		DMStateLeft:    "left",
		DMStateKicked:  "kicked",
	}
	for state, want := range cases {
		if string(state) != want {
			t.Errorf("state = %q, want %q", state, want)
		}
	}
}

func TestDMThreadTitleOmittedWhenEmpty(t *testing.T) {
	th := DMThread{ID: "t1", CreatedBy: "m1", CreatedAt: time.Unix(0, 0).UTC()}
	b, err := json.Marshal(th)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := got["title"]; ok {
		t.Errorf("untitled thread should omit title: %s", b)
	}

	th.Title = "Ward planning"
	b2, err := json.Marshal(th)
	if err != nil {
		t.Fatalf("Marshal titled: %v", err)
	}
	var got2 map[string]any
	if err := json.Unmarshal(b2, &got2); err != nil {
		t.Fatalf("Unmarshal titled: %v", err)
	}
	if v, ok := got2["title"]; !ok || v != "Ward planning" {
		t.Errorf("titled thread should carry title, got %v (present=%v): %s", v, ok, b2)
	}
}

func TestDMParticipantJSONFields(t *testing.T) {
	p := DMParticipant{ThreadID: "t1", ActorID: "m1", State: DMStateActive, IsAdmin: true, UpdatedAt: time.Unix(0, 0).UTC()}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"thread_id", "actor_id", "state", "is_admin", "updated_at"} {
		if _, ok := got[key]; !ok {
			t.Errorf("DMParticipant missing key %q: %s", key, b)
		}
	}
}

// The gateway never decrypts — PayloadCiphertext and PerRecipientKeys must
// round-trip as opaque values, byte for byte.
func TestDMMessageCiphertextRoundTrips(t *testing.T) {
	m := DMMessage{
		ID:                "m1",
		ThreadID:          "t1",
		SenderID:          "s1",
		SenderDeviceKeyID: "k1",
		PayloadCiphertext: "opaque-blob",
		PerRecipientKeys:  map[string]string{"m2": "wrapped-key-for-m2"},
		CreatedAt:         time.Unix(0, 0).UTC(),
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got DMMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.PayloadCiphertext != m.PayloadCiphertext {
		t.Errorf("PayloadCiphertext = %q, want %q", got.PayloadCiphertext, m.PayloadCiphertext)
	}
	if got.PerRecipientKeys["m2"] != "wrapped-key-for-m2" {
		t.Errorf("PerRecipientKeys[m2] = %q, want %q", got.PerRecipientKeys["m2"], "wrapped-key-for-m2")
	}
}

func TestDMDeviceKeyJSONFields(t *testing.T) {
	k := DMDeviceKey{ActorID: "m1", KeyID: "k1", PublicKey: "pk", CreatedAt: time.Unix(0, 0).UTC()}
	b, err := json.Marshal(k)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"actor_id", "key_id", "public_key", "created_at"} {
		if _, ok := got[key]; !ok {
			t.Errorf("DMDeviceKey missing key %q: %s", key, b)
		}
	}
}
