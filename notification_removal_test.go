package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func removalNotice(event models.NotificationEvent, subjectID string) mwanachamacomm.Notification {
	return mwanachamacomm.Notification{
		ActorID:       "m-1",
		AuthorActorID: "lead-1",
		Category:      models.CategoryMembership,
		Event:         event,
		SubjectKind:   models.SubjectGroup,
		SubjectID:     subjectID,
	}
}

func TestARemovalNoticeRaisesUnderMembershipNamingWhoRemoved(t *testing.T) {
	for _, e := range []models.NotificationEvent{models.EventGroupMemberRemoved, models.EventAgencyMemberRemoved} {
		s := newNotificationStore(t)
		n, err := s.Raise(context.Background(), removalNotice(e, "group-1"))
		if err != nil {
			t.Fatalf("Raise %s: %v", e, err)
		}
		if n.Category != models.CategoryMembership || n.AuthorActorID != "lead-1" {
			t.Fatalf("Raise %s = %+v, want membership naming lead-1", e, n)
		}
	}
}

func TestARemovalNoticeOutsideMembershipIsRefused(t *testing.T) {
	s := newNotificationStore(t)
	bad := removalNotice(models.EventGroupMemberRemoved, "group-1")
	bad.Category = models.CategoryChat
	if _, err := s.Raise(context.Background(), bad); !errors.Is(err, mwanachamacomm.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid, got %v", err)
	}
}

func TestARemovalNoticeReachesAMemberWhoMutedMembership(t *testing.T) {
	for _, e := range []models.NotificationEvent{models.EventGroupMemberRemoved, models.EventAgencyMemberRemoved} {
		s := newNotificationStore(t)
		ctx := context.Background()
		if _, err := s.SetPreference(ctx, "m-1", models.CategoryMembership, true); err != nil {
			t.Fatalf("SetPreference: %v", err)
		}
		s.UseMutable(func(context.Context, models.NotificationEvent) (bool, bool) { return true, true })
		if _, err := s.Raise(ctx, removalNotice(e, "group-1")); err != nil {
			t.Fatalf("Raise %s to a muted member: %v", e, err)
		}
		out, err := s.List(ctx, "m-1", 0)
		if err != nil || len(out) != 1 || out[0].Event != e {
			t.Fatalf("List = %+v, err %v, want the %s notice", out, err, e)
		}
	}
}
