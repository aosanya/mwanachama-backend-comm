package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestEachAppointmentNoticeIsRaisedUnderTheCivicSpec(t *testing.T) {
	notices := []mwanachamacomm.Notification{
		{ActorID: "applicant-1", Category: models.CategoryCase, Event: models.EventAppointmentBooked, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
		{ActorID: "applicant-1", Category: models.CategoryCase, Event: models.EventAppointmentCancelled, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
		{ActorID: "clerk-1", Category: models.CategoryCase, Event: models.EventAppointmentCancelledByMember, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
		{ActorID: "clerk-2", Category: models.CategoryCase, Event: models.EventAppointmentCancelledByMember, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
	}
	s := newNotificationStore(t)
	ctx := context.Background()
	for _, n := range notices {
		if _, err := s.Raise(ctx, n); err != nil {
			t.Fatalf("Raise %s to %s: %v", n.Event, n.ActorID, err)
		}
	}
	for actor, want := range map[string]int{"applicant-1": 2, "clerk-1": 1, "clerk-2": 1} {
		got, err := s.List(ctx, actor, 0)
		if err != nil || len(got) != want {
			t.Fatalf("%s List = %d, err %v, want %d", actor, len(got), err, want)
		}
	}
}

func TestAnAppointmentNoticeUnderTaskIsRefused(t *testing.T) {
	s := newNotificationStore(t)
	_, err := s.Raise(context.Background(), mwanachamacomm.Notification{
		ActorID: "applicant-1", Category: models.CategoryTask, Event: models.EventAppointmentBooked,
		SubjectKind: models.SubjectCase, SubjectID: "case-1",
	})
	if !errors.Is(err, mwanachamacomm.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid, got %v", err)
	}
}

func TestMutingCaseSilencesAppointmentNotices(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	if _, err := s.SetPreference(ctx, "applicant-1", models.CategoryCase, true); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	_, err := s.Raise(ctx, mwanachamacomm.Notification{
		ActorID: "applicant-1", Category: models.CategoryCase, Event: models.EventAppointmentCancelled,
		SubjectKind: models.SubjectCase, SubjectID: "case-1",
	})
	if !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
		t.Fatalf("expected ErrNotificationMuted, got %v", err)
	}
}
