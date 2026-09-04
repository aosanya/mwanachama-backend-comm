package mwanachamacomm

import (
	"encoding/json"
	"testing"
)

func TestReasonAndStateValues(t *testing.T) {
	if ReportReasonThreats != "threats" || ReportReasonAbuse != "abuse" ||
		ReportReasonFalseClaim != "false_claim" || ReportReasonSpam != "spam" {
		t.Fatalf("ReportReason drifted: %q %q %q %q",
			ReportReasonThreats, ReportReasonAbuse, ReportReasonFalseClaim, ReportReasonSpam)
	}
	if RemovalReasonThreats != "threats" || RemovalReasonAbuse != "abuse" || RemovalReasonFalseClaim != "false_claim" {
		t.Fatalf("RemovalReason drifted: %q %q %q", RemovalReasonThreats, RemovalReasonAbuse, RemovalReasonFalseClaim)
	}
	if DisputeOpen != "open" || DisputeReinstated != "reinstated" || DisputeUpheld != "upheld" {
		t.Fatalf("DisputeState drifted: %q %q %q", DisputeOpen, DisputeReinstated, DisputeUpheld)
	}
}

// TestRoomProjectionStripsRemovedBy is G47's anonymity as a type-level
// guarantee: whatever the caller does with a Removal after calling Room(),
// the actor's identity is gone from the value, not merely hidden by a
// forgetful renderer.
func TestRoomProjectionStripsRemovedBy(t *testing.T) {
	full := Removal{
		ID: "removal-1", MessageID: "msg-1", ChapterID: "chapter-1",
		RemovedBy: "member-mod", ActorRoleClass: "Ward coordinator",
		Reason: RemovalReasonAbuse,
	}
	room := full.Room()
	if room.RemovedBy != "" {
		t.Fatalf("Room() must strip RemovedBy, got %q", room.RemovedBy)
	}
	if room.ActorRoleClass != full.ActorRoleClass || room.Reason != full.Reason {
		t.Fatalf("Room() must keep the room-safe fields: got %+v", room)
	}
	// The original value is untouched — Room() must not mutate the receiver.
	if full.RemovedBy != "member-mod" {
		t.Fatalf("Room() must not mutate its receiver, got %q", full.RemovedBy)
	}

	b, err := json.Marshal(room)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := got["removed_by"]; ok {
		t.Errorf("room-safe removal must omit removed_by on the wire: %s", b)
	}
}

// TestDisputeDecidedAtOmittedWhileOpen mirrors consent's
// TestPublishedAtPointerNilByDefault: an undecided dispute must not carry a
// zero-valued decided_at on the wire.
func TestDisputeDecidedAtOmittedWhileOpen(t *testing.T) {
	d := Dispute{ID: "dispute-1", State: DisputeOpen}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := got["decided_at"]; ok {
		t.Errorf("an open dispute must omit decided_at: %s", b)
	}
	if _, ok := got["decided_by"]; ok {
		t.Errorf("an open dispute must omit decided_by: %s", b)
	}
}
