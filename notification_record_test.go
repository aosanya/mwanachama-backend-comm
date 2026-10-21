package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func recordChangeNotice(event models.NotificationEvent) mwanachamacomm.Notification {
	return mwanachamacomm.Notification{
		ActorID:       "resident-1",
		AuthorActorID: "clerk-1",
		Category:      models.CategoryMembership,
		Event:         event,
		SubjectKind:   models.SubjectRecordChange,
		SubjectID:     "change-1",
	}
}

func TestEachRecordChangeDecisionReachesTheResidentUnderMembership(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	for _, e := range []models.NotificationEvent{models.EventRecordChangeApproved, models.EventRecordChangeRefused} {
		if _, err := s.Raise(ctx, recordChangeNotice(e)); err != nil {
			t.Fatalf("Raise %s: %v", e, err)
		}
	}
	out, err := s.List(ctx, "resident-1", 0)
	if err != nil || len(out) != 2 {
		t.Fatalf("List = %d notices, err %v, want 2", len(out), err)
	}
	for _, n := range out {
		if n.SubjectKind != models.SubjectRecordChange || n.SubjectID != "change-1" {
			t.Fatalf("notice names %s %s, want the change request it decides", n.SubjectKind, n.SubjectID)
		}
	}
}

func TestARecordChangeNoticeOutsideMembershipIsRefused(t *testing.T) {
	s := newNotificationStore(t)
	bad := recordChangeNotice(models.EventRecordChangeRefused)
	bad.Category = models.CategoryItem
	if _, err := s.Raise(context.Background(), bad); !errors.Is(err, mwanachamacomm.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid, got %v", err)
	}
}
