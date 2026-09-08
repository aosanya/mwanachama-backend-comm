package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestChatThreadJSONFields(t *testing.T) {
	th := ChatThread{ID: "t1", StructureID: "c1", TagPath: "events/2026", CreatedAt: time.Unix(0, 0).UTC()}
	b, err := json.Marshal(th)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"id", "structure_id", "tag_path", "created_at"} {
		if _, ok := got[key]; !ok {
			t.Errorf("ChatThread missing key %q: %s", key, b)
		}
	}
}

// A room-level message (not in a thread) must not carry a thread_id key —
// the client uses its absence to render it outside any thread.
func TestChatMessageThreadIDOmittedForRoomLevelPost(t *testing.T) {
	m := ChatMessage{ID: "m1", StructureID: "c1", AuthorID: "a1", Body: "hi", CreatedAt: time.Unix(0, 0).UTC()}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := got["thread_id"]; ok {
		t.Errorf("room-level message should omit thread_id: %s", b)
	}

	m.ThreadID = "t1"
	b2, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal threaded: %v", err)
	}
	var got2 map[string]any
	if err := json.Unmarshal(b2, &got2); err != nil {
		t.Fatalf("Unmarshal threaded: %v", err)
	}
	if v, ok := got2["thread_id"]; !ok || v != "t1" {
		t.Errorf("threaded message should carry thread_id = t1, got %v (present=%v): %s", v, ok, b2)
	}
}
