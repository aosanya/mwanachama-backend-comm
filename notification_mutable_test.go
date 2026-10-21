package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func raiseResultsTo(ctx context.Context, s *mwanachamacomm.NotificationStore) error {
	_, err := s.Raise(ctx, mwanachamacomm.Notification{
		ActorID:     "m-1",
		Category:    mwanachamacomm.CategoryResults,
		Event:       mwanachamacomm.EventResultsPublished,
		SubjectKind: mwanachamacomm.SubjectSurvey,
		SubjectID:   "survey-1",
	})
	return err
}

func mutedStore(t *testing.T) *mwanachamacomm.NotificationStore {
	t.Helper()
	s := newNotificationStore(t)
	if _, err := s.SetPreference(context.Background(), "m-1", mwanachamacomm.CategoryResults, true); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	return s
}

func TestAHostMarkingAnEventMutableLeavesAMutedMemberUntold(t *testing.T) {
	s := mutedStore(t)
	s.UseMutable(func(context.Context, models.NotificationEvent) (bool, bool) { return true, true })
	if err := raiseResultsTo(context.Background(), s); !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
		t.Fatalf("expected ErrNotificationMuted, got %v", err)
	}
}

func TestAHostMarkingAnEventUnmutableTellsAMutedMember(t *testing.T) {
	s := mutedStore(t)
	s.UseMutable(func(_ context.Context, e models.NotificationEvent) (bool, bool) {
		return false, e == models.EventResultsPublished
	})
	if err := raiseResultsTo(context.Background(), s); err != nil {
		t.Fatalf("expected the muted member to be told, got %v", err)
	}
}

func TestAHostThatDoesNotKnowAnEventLeavesTheMuteStanding(t *testing.T) {
	s := mutedStore(t)
	s.UseMutable(func(context.Context, models.NotificationEvent) (bool, bool) { return false, false })
	if err := raiseResultsTo(context.Background(), s); !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
		t.Fatalf("expected ErrNotificationMuted, got %v", err)
	}
}

func TestWithNoHostMutableTheMuteStandsAsBefore(t *testing.T) {
	s := mutedStore(t)
	if err := raiseResultsTo(context.Background(), s); !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
		t.Fatalf("expected ErrNotificationMuted, got %v", err)
	}
}
