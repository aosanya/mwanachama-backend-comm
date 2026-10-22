package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestEachCaseAndTaskNoticeIsRaisedUnderTheCivicSpec(t *testing.T) {
	notices := []mwanachamacomm.Notification{
		{ActorID: "asker-1", Category: models.CategoryCase, Event: models.EventCaseOpened, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
		{ActorID: "asker-1", Category: models.CategoryCase, Event: models.EventCaseClosed, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
		{ActorID: "assignee-1", Category: models.CategoryTask, Event: models.EventTaskAssigned, SubjectKind: models.SubjectTask, SubjectID: "task-1"},
		{ActorID: "assignee-1", Category: models.CategoryTask, Event: models.EventTaskOverdue, SubjectKind: models.SubjectTask, SubjectID: "task-1"},
	}
	s := newNotificationStore(t)
	ctx := context.Background()
	for _, n := range notices {
		if _, err := s.Raise(ctx, n); err != nil {
			t.Fatalf("Raise %s: %v", n.Event, err)
		}
	}
	asker, err := s.List(ctx, "asker-1", 0)
	if err != nil || len(asker) != 2 {
		t.Fatalf("asker List = %d, err %v, want 2", len(asker), err)
	}
	assignee, err := s.List(ctx, "assignee-1", 0)
	if err != nil || len(assignee) != 2 {
		t.Fatalf("assignee List = %d, err %v, want 2", len(assignee), err)
	}
}

func TestATaskNoticeUnderCaseIsRefused(t *testing.T) {
	s := newNotificationStore(t)
	_, err := s.Raise(context.Background(), mwanachamacomm.Notification{
		ActorID: "assignee-1", Category: models.CategoryCase, Event: models.EventTaskAssigned,
		SubjectKind: models.SubjectTask, SubjectID: "task-1",
	})
	if !errors.Is(err, mwanachamacomm.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid, got %v", err)
	}
}

func TestMutingCaseOrTaskSilencesItsNotices(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	if _, err := s.SetPreference(ctx, "assignee-1", models.CategoryTask, true); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	_, err := s.Raise(ctx, mwanachamacomm.Notification{
		ActorID: "assignee-1", Category: models.CategoryTask, Event: models.EventTaskOverdue,
		SubjectKind: models.SubjectTask, SubjectID: "task-1",
	})
	if !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
		t.Fatalf("expected ErrNotificationMuted, got %v", err)
	}
}
