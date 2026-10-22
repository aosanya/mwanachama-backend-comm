package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestEachVolunteerNoticeIsRaisedUnderTheCivicSpec(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	for _, n := range []mwanachamacomm.Notification{
		{ActorID: "member-1", Category: models.CategoryPost, Event: models.EventVolunteerPicked, SubjectKind: models.SubjectPost, SubjectID: "post-1"},
		{ActorID: "member-1", Category: models.CategoryPost, Event: models.EventVolunteerCallFilled, SubjectKind: models.SubjectPost, SubjectID: "post-2"},
	} {
		if _, err := s.Raise(ctx, n); err != nil {
			t.Fatalf("Raise %s: %v", n.Event, err)
		}
	}
	got, err := s.List(ctx, "member-1", 0)
	if err != nil || len(got) != 2 {
		t.Fatalf("List = %d, err %v, want 2", len(got), err)
	}
}

func TestMutingPostSilencesAVolunteerNotice(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	if _, err := s.SetPreference(ctx, "member-1", models.CategoryPost, true); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	_, err := s.Raise(ctx, mwanachamacomm.Notification{
		ActorID: "member-1", Category: models.CategoryPost, Event: models.EventVolunteerCallFilled,
		SubjectKind: models.SubjectPost, SubjectID: "post-1",
	})
	if !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
		t.Fatalf("expected ErrNotificationMuted, got %v", err)
	}
}

func TestAVolunteerPickUnderTaskIsRefused(t *testing.T) {
	s := newNotificationStore(t)
	_, err := s.Raise(context.Background(), mwanachamacomm.Notification{
		ActorID: "member-1", Category: models.CategoryTask, Event: models.EventVolunteerPicked,
		SubjectKind: models.SubjectPost, SubjectID: "post-1",
	})
	if !errors.Is(err, mwanachamacomm.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid, got %v", err)
	}
}
