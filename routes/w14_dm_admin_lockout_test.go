package routes_test

// Board row W14: a DM thread's roster can no longer be left without an
// active admin — Kick refuses a self-kick and Leave refuses to let the last
// active admin go while others remain (dm_roster_impl.go).

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func newW14Thread(t *testing.T, invitees ...string) (mwanachamacomm.DMRepository, mwanachamacomm.DMThread) {
	t.Helper()
	db, tables := newRouteTestDB(t)
	dm, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	ctx := context.Background()
	th, err := dm.CreateThread(ctx, mwanachamacomm.DMThread{CreatedBy: "admin-1"}, invitees)
	if err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	for _, id := range invitees {
		if _, err := dm.Accept(ctx, th.ID, id); err != nil {
			t.Fatalf("%s accept: %v", id, err)
		}
	}
	return dm, th
}

func TestKick_RefusesSelfKick(t *testing.T) {
	dm, th := newW14Thread(t)
	err := dm.Kick(context.Background(), th.ID, "admin-1", "admin-1")
	if !errors.Is(err, mwanachamacomm.ErrDMSelfKick) {
		t.Fatalf("self-kick: got %v, want ErrDMSelfKick", err)
	}
}

func TestLeave_RefusesLastAdminWhileOthersRemain(t *testing.T) {
	dm, th := newW14Thread(t, "member-2")
	ctx := context.Background()

	if err := dm.Leave(ctx, th.ID, "admin-1"); !errors.Is(err, mwanachamacomm.ErrDMLastAdmin) {
		t.Fatalf("last admin leaving: got %v, want ErrDMLastAdmin", err)
	}
	// The thread still has its admin.
	if _, err := dm.Promote(ctx, th.ID, "member-2", "admin-1"); err != nil {
		t.Fatalf("admin-1 must still be able to promote: %v", err)
	}
	// With a successor in place the original admin may leave.
	if err := dm.Leave(ctx, th.ID, "admin-1"); err != nil {
		t.Fatalf("leave after promoting a successor: %v", err)
	}
	if _, err := dm.Invite(ctx, th.ID, "member-3", "member-2"); err != nil {
		t.Fatalf("the successor must be able to administer the thread: %v", err)
	}
}

func TestLeave_LastAdminMayLeaveWhenAloneOrNoOneElseIsActive(t *testing.T) {
	dm, th := newW14Thread(t)
	if err := dm.Leave(context.Background(), th.ID, "admin-1"); err != nil {
		t.Fatalf("the only active participant leaving: %v", err)
	}
}

func TestLeave_NonAdminIsUnaffected(t *testing.T) {
	dm, th := newW14Thread(t, "member-2")
	if err := dm.Leave(context.Background(), th.ID, "member-2"); err != nil {
		t.Fatalf("a non-admin leaving: %v", err)
	}
}
