package routes_test

// Pins board row W14: a DM thread's sole admin can strand every remaining
// participant with no admin at all — permanently, since every roster
// mutation except a very narrow self-reenable case requires an already
// active admin to authorize it. See dm_roster_impl.go's Kick/Leave/Promote/
// ReEnable.

import (
	"context"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

// Pins W14's narrowest form: Kick has no guard against by == actorID, so
// a sole admin can kick themselves. Once fixed, this call should return an
// error (e.g. "cannot kick yourself") instead of nil.
func TestKick_PinsSoleAdminCanKickThemselves(t *testing.T) {
	db, tables := newRouteTestDB(t)
	dm, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	ctx := context.Background()

	th, err := dm.CreateThread(ctx, mwanachamacomm.DMThread{CreatedBy: "admin-1"}, nil)
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}

	if err := dm.Kick(ctx, th.ID, "admin-1", "admin-1"); err != nil {
		t.Fatalf("W14 appears fixed: self-kick is now refused (%v) — update this pin to assert that error instead", err)
	}
}

// Pins W14's general shape: once a self-kick (or an ordinary Leave — see
// TestLeave_PinsLastAdminLeavingStrandsRemainingParticipants below) removes
// the thread's only admin, NOTHING can bring an admin back — not
// self-promotion (nobody is an active admin to authorize it), not ReEnable
// (that path only accepts the caller's own row in DMStateLeft, and only
// when they were the very last participant of any kind — never applies
// once other participants remain, and never applies to DMStateKicked at
// all). Once fixed, one of these two calls should succeed.
func TestKick_PinsSelfKickedSoleAdminHasNoWayBackIn(t *testing.T) {
	db, tables := newRouteTestDB(t)
	dm, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	ctx := context.Background()

	th, err := dm.CreateThread(ctx, mwanachamacomm.DMThread{CreatedBy: "admin-1"}, nil)
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	if err := dm.Kick(ctx, th.ID, "admin-1", "admin-1"); err != nil {
		t.Fatalf("seed self-kick: %v", err)
	}

	_, reenableErr := dm.ReEnable(ctx, th.ID, "admin-1", "admin-1")
	_, promoteErr := dm.Promote(ctx, th.ID, "admin-1", "admin-1")
	if reenableErr != nil && promoteErr != nil {
		t.Logf("both recovery paths refused, as expected of the current bug: reenable=%v promote=%v", reenableErr, promoteErr)
	} else {
		t.Fatalf("W14 appears fixed: a recovery path now succeeds (reenable=%v, promote=%v) — update this pin", reenableErr, promoteErr)
	}
}

// Pins W14's broadest, most realistic form: the admin leaves *normally*
// (Leave, not Kick) while another, non-admin participant remains active.
// That remaining participant is now permanently stuck in an admin-less
// thread — they can't self-promote (Promote requires an already-active
// admin caller) and can't bring the departed admin back (ReEnable's
// by-admin path has the same requirement). Once fixed, one of these two
// calls should succeed (e.g. Leave refusing to leave the last admin
// without a successor, or some other admin-recovery path).
func TestLeave_PinsLastAdminLeavingStrandsRemainingParticipants(t *testing.T) {
	db, tables := newRouteTestDB(t)
	dm, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	ctx := context.Background()

	th, err := dm.CreateThread(ctx, mwanachamacomm.DMThread{CreatedBy: "admin-1"}, []string{"member-2"})
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	if _, err := dm.Accept(ctx, th.ID, "member-2"); err != nil {
		t.Fatalf("member-2 accept: %v", err)
	}

	if err := dm.Leave(ctx, th.ID, "admin-1"); err != nil {
		t.Fatalf("W14 appears fixed: the last admin can no longer Leave without a successor (%v) — update this pin", err)
	}

	_, promoteErr := dm.Promote(ctx, th.ID, "member-2", "member-2")
	_, reenableErr := dm.ReEnable(ctx, th.ID, "admin-1", "member-2")
	if promoteErr != nil && reenableErr != nil {
		t.Logf("both recovery paths refused, as expected of the current bug: promote=%v reenable=%v", promoteErr, reenableErr)
	} else {
		t.Fatalf("W14 appears fixed: a recovery path now succeeds (promote=%v, reenable=%v) — update this pin", promoteErr, reenableErr)
	}
}
