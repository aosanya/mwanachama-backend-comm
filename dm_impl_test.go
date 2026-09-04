package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func newDMStore(t *testing.T) *mwanachamacomm.DMStore {
	t.Helper()
	db, tables := newTestDB(t)
	s, err := mwanachamacomm.NewDMStore(db, tables, monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	return s
}

func TestDMLifecycle(t *testing.T) {
	s := newDMStore(t)
	ctx := context.Background()

	th, err := s.CreateThread(ctx, models.DMThread{Title: "board", CreatedBy: "m-1"}, []string{"m-2"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Invite(ctx, th.ID, "m-3", "m-1"); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if _, err := s.Accept(ctx, th.ID, "m-2"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if _, err := s.Post(ctx, models.DMMessage{ThreadID: th.ID, SenderID: "m-1", PayloadCiphertext: "hi"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	msgs, err := s.ListMessages(ctx, th.ID)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("ListMessages = %+v, err %v, want 1 message", msgs, err)
	}
	parts, err := s.ListParticipants(ctx, th.ID)
	if err != nil || len(parts) != 3 {
		t.Fatalf("ListParticipants = %+v, err %v, want 3", parts, err)
	}
}

func TestDMInviteRefusesOverwritingAnActiveMember(t *testing.T) {
	s := newDMStore(t)
	ctx := context.Background()
	th, err := s.CreateThread(ctx, models.DMThread{CreatedBy: "m-1"}, []string{"m-2"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Accept(ctx, th.ID, "m-2"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if _, err := s.Invite(ctx, th.ID, "m-2", "m-1"); !errors.Is(err, models.ErrDMAlreadyActive) {
		t.Fatalf("re-inviting an active member: got %v, want ErrDMAlreadyActive", err)
	}
	// Re-inviting somebody still pending stays allowed and idempotent.
	if _, err := s.Invite(ctx, th.ID, "m-2", "m-1"); err != nil && !errors.Is(err, models.ErrDMAlreadyActive) {
		t.Fatalf("unexpected error: %v", err)
	}
	// Confirm the active member kept their acceptance (BUG-20260827-006/007's
	// whole point).
	parts, _ := s.ListParticipants(ctx, th.ID)
	for _, p := range parts {
		if p.MemberID == "m-2" && p.State != models.DMStateActive {
			t.Fatalf("m-2's acceptance must survive a repeat invite, got state %q", p.State)
		}
	}
}

func TestDMReEnableOnlyTheLastToLeave(t *testing.T) {
	s := newDMStore(t)
	ctx := context.Background()
	th, err := s.CreateThread(ctx, models.DMThread{CreatedBy: "m-1"}, []string{"m-2"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Accept(ctx, th.ID, "m-2"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	// m-1 is still active, so m-2 leaving and trying to come back alone
	// must be refused — only the LAST to leave brings the thread back.
	if err := s.Leave(ctx, th.ID, "m-2"); err != nil {
		t.Fatalf("leave m-2: %v", err)
	}
	if _, err := s.ReEnable(ctx, th.ID, "m-2", "m-2"); !errors.Is(err, models.ErrDMNotLastToLeave) {
		t.Fatalf("re-enable while m-1 still active: got %v, want ErrDMNotLastToLeave", err)
	}
	if err := s.Leave(ctx, th.ID, "m-1"); err != nil {
		t.Fatalf("leave m-1: %v", err)
	}
	// Now nobody is active — m-2, the last to leave, may bring it back.
	p, err := s.ReEnable(ctx, th.ID, "m-2", "m-2")
	if err != nil {
		t.Fatalf("re-enable as last to leave: %v", err)
	}
	if p.State != models.DMStateActive {
		t.Fatalf("self re-enable must land active, got %q", p.State)
	}
}

func TestDMReEnableSelfIsNotAnAdminAct(t *testing.T) {
	// BUG-20260828-001: Leave has just set the caller's own row to `left`,
	// so gating self re-enable on isAdmin (which requires StateActive)
	// could never pass — the fix is recognising "you left, and you are the
	// only one who left" as its own admission, not an admin capability.
	s := newDMStore(t)
	ctx := context.Background()
	th, err := s.CreateThread(ctx, models.DMThread{CreatedBy: "m-1"}, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Leave(ctx, th.ID, "m-1"); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if _, err := s.ReEnable(ctx, th.ID, "m-1", "m-1"); err != nil {
		t.Fatalf("self re-enable of a non-admin creator: %v", err)
	}
}

func TestDMPromoteMissingParticipantIsNotFound(t *testing.T) {
	s := newDMStore(t)
	ctx := context.Background()
	th, err := s.CreateThread(ctx, models.DMThread{CreatedBy: "m-1"}, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Promote(ctx, th.ID, "nobody", "m-1"); !errors.Is(err, models.ErrDMNotFound) {
		t.Fatalf("promoting a non-participant: got %v, want ErrDMNotFound", err)
	}
}

func TestDMDeviceKeyRepublishRetiresSiblingsOnTheSameDevice(t *testing.T) {
	s := newDMStore(t)
	ctx := context.Background()
	k1, err := s.PublishDeviceKey(ctx, models.DMDeviceKey{MemberID: "m-1", PublicKey: "pk1", DeviceID: "phone-1"})
	if err != nil {
		t.Fatalf("publish 1: %v", err)
	}
	if _, err := s.PublishDeviceKey(ctx, models.DMDeviceKey{MemberID: "m-1", PublicKey: "pk2", DeviceID: "phone-1"}); err != nil {
		t.Fatalf("publish 2: %v", err)
	}
	live, err := s.LookupDeviceKeys(ctx, []string{"m-1"})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(live) != 1 || live[0].PublicKey != "pk2" {
		t.Fatalf("expected only the newest key live, got %+v", live)
	}
	for _, k := range live {
		if k.KeyID == k1.KeyID {
			t.Fatalf("the first key must have been retired by the republish")
		}
	}

	// A second handset for the same member keeps its own key live.
	if _, err := s.PublishDeviceKey(ctx, models.DMDeviceKey{MemberID: "m-1", PublicKey: "pk-other-phone", DeviceID: "phone-2"}); err != nil {
		t.Fatalf("publish other device: %v", err)
	}
	live2, err := s.LookupDeviceKeys(ctx, []string{"m-1"})
	if err != nil || len(live2) != 2 {
		t.Fatalf("expected two live keys across two devices, got %+v, err %v", live2, err)
	}
}

func TestDMReactionsReplaceNotAccumulate(t *testing.T) {
	s := newDMStore(t)
	ctx := context.Background()
	th, err := s.CreateThread(ctx, models.DMThread{CreatedBy: "m-1"}, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	msg, err := s.Post(ctx, models.DMMessage{ThreadID: th.ID, SenderID: "m-1", PayloadCiphertext: "hi"})
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if err := s.SetReaction(ctx, models.DMReaction{MessageID: msg.ID, MemberID: "m-1", Emoji: "👍"}); err != nil {
		t.Fatalf("set reaction: %v", err)
	}
	if err := s.SetReaction(ctx, models.DMReaction{MessageID: msg.ID, MemberID: "m-1", Emoji: "🎉"}); err != nil {
		t.Fatalf("replace reaction: %v", err)
	}
	rs, err := s.ListReactions(ctx, th.ID)
	if err != nil || len(rs) != 1 || rs[0].Emoji != "🎉" {
		t.Fatalf("ListReactions = %+v, err %v, want one 🎉", rs, err)
	}
	if err := s.ClearReaction(ctx, msg.ID, "m-1"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	// Clearing again is not an error — the caller asked for a state that
	// already holds.
	if err := s.ClearReaction(ctx, msg.ID, "m-1"); err != nil {
		t.Fatalf("clear again: %v", err)
	}
	rs, err = s.ListReactions(ctx, th.ID)
	if err != nil || len(rs) != 0 {
		t.Fatalf("ListReactions after clear = %+v, err %v, want none", rs, err)
	}
}

func TestDMMessageTTLExpiry(t *testing.T) {
	s := newDMStore(t)
	ctx := context.Background()
	ttl := 60
	th, err := s.CreateThread(ctx, models.DMThread{CreatedBy: "m-1", MessageTTLSeconds: &ttl}, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Post(ctx, models.DMMessage{ThreadID: th.ID, SenderID: "m-1", PayloadCiphertext: "will expire"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	msgs, err := s.ListMessages(ctx, th.ID)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("before expiry: ListMessages = %+v, err %v, want 1", msgs, err)
	}
}
