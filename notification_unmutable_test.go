package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func ownRequestNotices() []models.Notification {
	return []models.Notification{
		{ActorID: "m-1", Category: models.CategoryPost, Event: models.EventVolunteerPicked, SubjectKind: models.SubjectPost, SubjectID: "post-1"},
		{ActorID: "m-1", Category: models.CategoryMembership, Event: models.EventRecordChangeApproved, SubjectKind: models.SubjectRecordChange, SubjectID: "change-1"},
		{ActorID: "m-1", Category: models.CategoryMembership, Event: models.EventRecordChangeRefused, SubjectKind: models.SubjectRecordChange, SubjectID: "change-2"},
		{ActorID: "m-1", Category: models.CategoryCase, Event: models.EventCaseClosed, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
	}
}

func storeMutingEverythingTheyTouch(t *testing.T) *mwanachamacomm.NotificationStore {
	t.Helper()
	s := newNotificationStore(t)
	for _, c := range []models.NotificationCategory{models.CategoryPost, models.CategoryMembership, models.CategoryCase, models.CategoryResults} {
		if _, err := s.SetPreference(context.Background(), "m-1", c, true); err != nil {
			t.Fatalf("SetPreference %s: %v", c, err)
		}
	}
	return s
}

func TestYourOwnRequestCorrectionOrPickReachesAMemberWhoMutedItsCategory(t *testing.T) {
	s := storeMutingEverythingTheyTouch(t)
	for _, n := range ownRequestNotices() {
		if !models.NotificationEventUnmutable(n.Event) {
			t.Errorf("%s is mutable", n.Event)
		}
		if _, err := s.Raise(context.Background(), n); err != nil {
			t.Errorf("Raise %s with %s muted: %v", n.Event, n.Category, err)
		}
	}
	out, err := s.List(context.Background(), "m-1", 0)
	if err != nil || len(out) != len(ownRequestNotices()) {
		t.Fatalf("List = %d notices, err %v, want %d", len(out), err, len(ownRequestNotices()))
	}
}

func TestAPollResultAndAFilledCallStayMutable(t *testing.T) {
	s := storeMutingEverythingTheyTouch(t)
	for _, n := range []models.Notification{
		{ActorID: "m-1", Category: models.CategoryResults, Event: models.EventPollResultPublished, SubjectKind: models.SubjectPoll, SubjectID: "poll-1"},
		{ActorID: "m-1", Category: models.CategoryPost, Event: models.EventVolunteerCallFilled, SubjectKind: models.SubjectPost, SubjectID: "post-1"},
	} {
		if models.NotificationEventUnmutable(n.Event) {
			t.Errorf("%s is unmutable", n.Event)
		}
		if _, err := s.Raise(context.Background(), n); !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
			t.Errorf("Raise %s with %s muted: expected ErrNotificationMuted, got %v", n.Event, n.Category, err)
		}
	}
}
