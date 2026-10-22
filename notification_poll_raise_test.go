package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestEachPollNoticeIsRaisedUnderTheCivicSpec(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	for _, n := range []mwanachamacomm.Notification{
		{ActorID: "member-1", Category: models.CategorySurvey, Event: models.EventPollOpened, SubjectKind: models.SubjectPoll, SubjectID: "poll-1"},
		{ActorID: "member-1", Category: models.CategoryResults, Event: models.EventPollResultPublished, SubjectKind: models.SubjectPoll, SubjectID: "poll-1"},
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

func TestAPollOpeningCannotBeMuted(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	if _, err := s.SetPreference(ctx, "member-1", models.CategorySurvey, true); err == nil {
		t.Fatalf("a mute on survey was stored")
	}
	if _, err := s.Raise(ctx, mwanachamacomm.Notification{
		ActorID: "member-1", Category: models.CategorySurvey, Event: models.EventPollOpened,
		SubjectKind: models.SubjectPoll, SubjectID: "poll-1",
	}); err != nil {
		t.Fatalf("Raise poll_opened: %v", err)
	}
}

func TestMutingResultsSilencesAPollResult(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	if _, err := s.SetPreference(ctx, "member-1", models.CategoryResults, true); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	_, err := s.Raise(ctx, mwanachamacomm.Notification{
		ActorID: "member-1", Category: models.CategoryResults, Event: models.EventPollResultPublished,
		SubjectKind: models.SubjectPoll, SubjectID: "poll-1",
	})
	if !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
		t.Fatalf("expected ErrNotificationMuted, got %v", err)
	}
}

func TestAPollOpeningUnderResultsIsRefused(t *testing.T) {
	s := newNotificationStore(t)
	_, err := s.Raise(context.Background(), mwanachamacomm.Notification{
		ActorID: "member-1", Category: models.CategoryResults, Event: models.EventPollOpened,
		SubjectKind: models.SubjectPoll, SubjectID: "poll-1",
	})
	if !errors.Is(err, mwanachamacomm.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid, got %v", err)
	}
}
