package mwanachamacomm

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestDMStore() *DMMemoryStore {
	return NewDMMemoryStore(NewIDGen(), monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
}

func TestDMCreateSeedsRoster(t *testing.T) {
	s := newTestDMStore()
	ctx := context.Background()

	th, err := s.CreateThread(ctx, DMThread{Title: "board", CreatedBy: "m-1"}, []string{"m-2", "m-3", "m-1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if th.ID == "" {
		t.Fatal("expected minted thread id")
	}

	parts, _ := s.ListParticipants(ctx, th.ID)
	if len(parts) != 3 {
		t.Fatalf("expected 3 participants (creator once + m-2/m-3), got %d: %+v", len(parts), parts)
	}
	byMember := map[string]DMParticipant{}
	for _, p := range parts {
		byMember[p.MemberID] = p
	}
	// Creator is active admin, dedup'd out of initial list.
	if !byMember["m-1"].IsAdmin || byMember["m-1"].State != DMStateActive {
		t.Fatalf("expected creator active admin, got %+v", byMember["m-1"])
	}
	// Invited members show up as invited non-admins.
	for _, id := range []string{"m-2", "m-3"} {
		if byMember[id].State != DMStateInvited || byMember[id].IsAdmin {
			t.Fatalf("expected %s invited non-admin, got %+v", id, byMember[id])
		}
	}
}

func TestDMInviteAcceptLeaveKickPromoteReEnable(t *testing.T) {
	s := newTestDMStore()
	ctx := context.Background()

	th, _ := s.CreateThread(ctx, DMThread{CreatedBy: "admin"}, nil)

	// A non-admin cannot invite.
	if _, err := s.Invite(ctx, th.ID, "spy", "outsider"); err == nil {
		t.Fatal("expected non-admin invite to fail")
	}
	// Admin invites m-1 and m-2.
	if _, err := s.Invite(ctx, th.ID, "m-1", "admin"); err != nil {
		t.Fatalf("invite m-1: %v", err)
	}
	if _, err := s.Invite(ctx, th.ID, "m-2", "admin"); err != nil {
		t.Fatalf("invite m-2: %v", err)
	}

	// m-1 accepts, m-2 leaves without accepting.
	if _, err := s.Accept(ctx, th.ID, "m-1"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if err := s.Leave(ctx, th.ID, "m-2"); err != nil {
		t.Fatalf("leave: %v", err)
	}

	// Promote m-1 to admin, then m-1 kicks a fresh member.
	if _, err := s.Promote(ctx, th.ID, "m-1", "admin"); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if _, err := s.Invite(ctx, th.ID, "m-3", "admin"); err != nil {
		t.Fatalf("invite m-3: %v", err)
	}
	if err := s.Kick(ctx, th.ID, "m-3", "m-1"); err != nil {
		t.Fatalf("kick by new admin: %v", err)
	}

	// Re-enable m-3 as invited.
	if _, err := s.ReEnable(ctx, th.ID, "m-3", "admin"); err != nil {
		t.Fatalf("re-enable: %v", err)
	}

	states := map[string]DMParticipantState{}
	admins := map[string]bool{}
	parts, _ := s.ListParticipants(ctx, th.ID)
	for _, p := range parts {
		states[p.MemberID] = p.State
		admins[p.MemberID] = p.IsAdmin
	}
	if states["admin"] != DMStateActive || !admins["admin"] {
		t.Fatalf("expected admin still active admin, got state=%v admin=%v", states["admin"], admins["admin"])
	}
	if states["m-1"] != DMStateActive || !admins["m-1"] {
		t.Fatalf("expected m-1 promoted active admin, got state=%v admin=%v", states["m-1"], admins["m-1"])
	}
	if states["m-2"] != DMStateLeft {
		t.Fatalf("expected m-2 left, got %v", states["m-2"])
	}
	if states["m-3"] != DMStateInvited {
		t.Fatalf("expected m-3 re-enabled to invited, got %v", states["m-3"])
	}

	// Idempotent-ish edge cases.
	if err := s.Leave(ctx, th.ID, "never-in"); !errors.Is(err, ErrDMNotFound) {
		t.Fatalf("expected ErrDMNotFound for stranger leave, got %v", err)
	}
}

func TestDMListThreadsForFiltersLeftAndKicked(t *testing.T) {
	s := newTestDMStore()
	ctx := context.Background()

	t1, _ := s.CreateThread(ctx, DMThread{CreatedBy: "a"}, []string{"b"})
	t2, _ := s.CreateThread(ctx, DMThread{CreatedBy: "a"}, []string{"b"})
	// b accepts t1, leaves t2.
	_, _ = s.Accept(ctx, t1.ID, "b")
	_ = s.Leave(ctx, t2.ID, "b")

	got, err := s.ListThreadsFor(ctx, "b")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// b should still see t1 (active) but not t2 (left).
	if len(got) != 1 || got[0].ID != t1.ID {
		t.Fatalf("expected only t1 for b, got %+v", got)
	}

	// The other side ('a') still admins both.
	gotA, _ := s.ListThreadsFor(ctx, "a")
	if len(gotA) != 2 {
		t.Fatalf("expected a to see both threads, got %d", len(gotA))
	}
}

func TestDMMessagePostAndList(t *testing.T) {
	s := newTestDMStore()
	ctx := context.Background()
	th, _ := s.CreateThread(ctx, DMThread{CreatedBy: "a"}, []string{"b"})

	_, _ = s.Post(ctx, DMMessage{ThreadID: th.ID, SenderID: "a", PayloadCiphertext: "first"})
	_, _ = s.Post(ctx, DMMessage{ThreadID: th.ID, SenderID: "b", PayloadCiphertext: "second"})
	// Message in a different thread must not leak.
	_, _ = s.Post(ctx, DMMessage{ThreadID: "other", SenderID: "x", PayloadCiphertext: "leak"})

	msgs, err := s.ListMessages(ctx, th.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2, got %d", len(msgs))
	}
	if msgs[0].PayloadCiphertext != "first" || msgs[1].PayloadCiphertext != "second" {
		t.Fatalf("wrong order/content: %+v", msgs)
	}
}

func TestDMDeviceKeyPublishAndLookup(t *testing.T) {
	s := newTestDMStore()
	ctx := context.Background()

	k1, err := s.PublishDeviceKey(ctx, DMDeviceKey{MemberID: "m-1", PublicKey: "pk1"})
	if err != nil {
		t.Fatalf("publish 1: %v", err)
	}
	if k1.KeyID == "" || k1.CreatedAt.IsZero() {
		t.Fatalf("expected minted key id + created_at, got %+v", k1)
	}
	// Second key for the same member — separate entry keyed by KeyID.
	if _, err := s.PublishDeviceKey(ctx, DMDeviceKey{MemberID: "m-1", PublicKey: "pk2"}); err != nil {
		t.Fatalf("publish 2: %v", err)
	}
	// A key for someone else.
	if _, err := s.PublishDeviceKey(ctx, DMDeviceKey{MemberID: "m-2", PublicKey: "pk3"}); err != nil {
		t.Fatalf("publish 3: %v", err)
	}

	only1, _ := s.LookupDeviceKeys(ctx, []string{"m-1"})
	if len(only1) != 2 {
		t.Fatalf("expected 2 keys for m-1, got %d", len(only1))
	}
	both, _ := s.LookupDeviceKeys(ctx, []string{"m-1", "m-2"})
	if len(both) != 3 {
		t.Fatalf("expected 3 keys for m-1+m-2, got %d", len(both))
	}
	none, _ := s.LookupDeviceKeys(ctx, []string{"nobody"})
	if len(none) != 0 {
		t.Fatalf("expected empty lookup for unknown member, got %d", len(none))
	}
}

// DEV-1541 · a message past its deadline is not served.
//
// **The filter is what makes the guarantee exact.** A sweeper alone would
// mean *gone within an hour of the deadline, roughly*, and that is not what
// the setting says. Refusing the row from the first read after the deadline
// makes it true at the instant it is made; the sweep is what makes the rows
// leave.
func TestDMMessagesPastTheirDeadlineAreNotServed(t *testing.T) {
	base := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	now := base
	s := NewDMMemoryStore(NewIDGen(), func() time.Time { return now })
	ctx := context.Background()

	ttl := 3600
	th, err := s.CreateThread(ctx, DMThread{
		CreatedBy:         "m-1",
		MessageTTLSeconds: &ttl,
	}, []string{"m-2"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if th.MessageTTLSeconds == nil || *th.MessageTTLSeconds != ttl {
		t.Fatalf("the thread should have frozen the timer, got %v", th.MessageTTLSeconds)
	}

	if _, err := s.Post(ctx, DMMessage{ThreadID: th.ID, SenderID: "m-1"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if got, _ := s.ListMessages(ctx, th.ID); len(got) != 1 {
		t.Fatalf("before the deadline the message is there, got %d", len(got))
	}

	// One second past. Not "roughly an hour later, once a job runs".
	now = base.Add(time.Duration(ttl+1) * time.Second)
	if got, _ := s.ListMessages(ctx, th.ID); len(got) != 0 {
		t.Fatalf("past the deadline the message is not served, got %d", len(got))
	}
}

// A thread with no timer keeps everything — every chapter-mate thread, and
// every thread opened through an address that set none.
func TestDMThreadWithNoTimerKeepsEverything(t *testing.T) {
	base := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	now := base
	s := NewDMMemoryStore(NewIDGen(), func() time.Time { return now })
	ctx := context.Background()

	th, _ := s.CreateThread(ctx, DMThread{CreatedBy: "m-1"}, []string{"m-2"})
	if _, err := s.Post(ctx, DMMessage{ThreadID: th.ID, SenderID: "m-1"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	now = base.Add(365 * 24 * time.Hour)
	if got, _ := s.ListMessages(ctx, th.ID); len(got) != 1 {
		t.Fatalf("a thread with no timer keeps its messages, got %d", len(got))
	}
}

// BUG-20260827-007 — a second invite aimed at a member who has already
// accepted used to write `invited` over her acceptance, and the message
// gates then answered her the same 404 a stranger gets. Asking again is only
// ever a nudge for somebody still pending.
func TestDMInviteNeverDemotesAnActiveParticipant(t *testing.T) {
	s := newTestDMStore()
	ctx := context.Background()

	th, _ := s.CreateThread(ctx, DMThread{CreatedBy: "admin"}, []string{"m-1"})

	// Still pending: asking again is allowed and changes nothing.
	if _, err := s.Invite(ctx, th.ID, "m-1", "admin"); err != nil {
		t.Fatalf("re-inviting a pending member: %v", err)
	}
	if _, err := s.Accept(ctx, th.ID, "m-1"); err != nil {
		t.Fatalf("accept: %v", err)
	}

	if _, err := s.Invite(ctx, th.ID, "m-1", "admin"); !errors.Is(err, ErrDMAlreadyActive) {
		t.Fatalf("expected ErrDMAlreadyActive re-inviting an active member, got %v", err)
	}
	parts, _ := s.ListParticipants(ctx, th.ID)
	for _, p := range parts {
		if p.MemberID == "m-1" && p.State != DMStateActive {
			t.Fatalf("expected m-1 still active after the refused invite, got %v", p.State)
		}
	}
}

// BUG-20260828-001 — `Reactivate this group` is the departed member's own
// act, and gating it on isAdmin (which requires being *currently active*)
// refused the only caller it exists for. They come back active, still admin.
func TestDMReEnableSelfAfterLeavingLast(t *testing.T) {
	s := newTestDMStore()
	ctx := context.Background()

	th, _ := s.CreateThread(ctx, DMThread{CreatedBy: "solo"}, nil)
	if err := s.Leave(ctx, th.ID, "solo"); err != nil {
		t.Fatalf("leave: %v", err)
	}

	out, err := s.ReEnable(ctx, th.ID, "solo", "solo")
	if err != nil {
		t.Fatalf("re-enable by the member who left last: %v", err)
	}
	if out.State != DMStateActive {
		t.Fatalf("expected the returning member active, not %v — a reactivation is not an invitation", out.State)
	}
	if !out.IsAdmin {
		t.Fatal("expected the thread's own admin to come back an admin")
	}
}

// The other side of the same gate: leaving a thread somebody else is still
// in does not buy a way back in, and a kicked member cannot re-enable
// themselves.
func TestDMReEnableSelfRefusedWhileOthersActive(t *testing.T) {
	s := newTestDMStore()
	ctx := context.Background()

	th, _ := s.CreateThread(ctx, DMThread{CreatedBy: "admin"}, []string{"m-1", "m-2"})
	if _, err := s.Accept(ctx, th.ID, "m-1"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if err := s.Leave(ctx, th.ID, "m-1"); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if _, err := s.ReEnable(ctx, th.ID, "m-1", "m-1"); !errors.Is(err, ErrDMNotLastToLeave) {
		t.Fatalf("expected ErrDMNotLastToLeave while the admin is still active, got %v", err)
	}

	if err := s.Kick(ctx, th.ID, "m-2", "admin"); err != nil {
		t.Fatalf("kick: %v", err)
	}
	if err := s.Leave(ctx, th.ID, "admin"); err != nil {
		t.Fatalf("admin leaves: %v", err)
	}
	if _, err := s.ReEnable(ctx, th.ID, "m-2", "m-2"); !errors.Is(err, ErrDMNotLastToLeave) {
		t.Fatalf("expected a kicked member to stay out, got %v", err)
	}

	// A stranger is answered the same way every other roster read answers them.
	if _, err := s.ReEnable(ctx, th.ID, "never-in", "never-in"); !errors.Is(err, ErrDMNotFound) {
		t.Fatalf("expected ErrDMNotFound for a stranger, got %v", err)
	}
}
