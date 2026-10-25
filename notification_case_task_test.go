package mwanachamacomm

import (
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func caseOrTaskNotices() []models.Notification {
	return []models.Notification{
		{ActorID: "asker-1", Category: models.CategoryCase, Event: models.EventCaseOpened, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
		{ActorID: "asker-1", Category: models.CategoryCase, Event: models.EventCaseClosed, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
		{ActorID: "assignee-1", Category: models.CategoryTask, Event: models.EventTaskAssigned, SubjectKind: models.SubjectTask, SubjectID: "task-1"},
		{ActorID: "holder-1", Category: models.CategoryTask, Event: models.EventTaskOverdue, SubjectKind: models.SubjectTask, SubjectID: "task-1"},
	}
}

func TestCaseAndTaskNoticesAreDeclaredByTheCivicSpec(t *testing.T) {
	civic := testSpec(t)
	for _, n := range caseOrTaskNotices() {
		if err := Check(civic, roleNotification, n); err != nil {
			t.Errorf("civic refused %s: %v", n.Event, err)
		}
	}
}

func TestCaseAndTaskNoticesAreNotTheSchoolsVocabulary(t *testing.T) {
	school := shippedSpecs(t)["school"]
	for _, n := range caseOrTaskNotices() {
		for _, probe := range []models.Notification{
			{ActorID: n.ActorID, SubjectID: n.SubjectID, Category: n.Category},
			{ActorID: n.ActorID, SubjectID: n.SubjectID, Event: n.Event},
			{ActorID: n.ActorID, SubjectID: n.SubjectID, SubjectKind: n.SubjectKind},
		} {
			if err := Check(school, roleNotification, probe); !errors.Is(err, models.ErrNotificationInvalid) {
				t.Errorf("school accepted %+v: %v", probe, err)
			}
		}
	}
}

func TestEachCaseOrTaskEventPairsWithItsOwnCategory(t *testing.T) {
	for _, n := range caseOrTaskNotices() {
		c, ok := models.NotificationCategoryOf(n.Event)
		if !ok || c != n.Category {
			t.Errorf("%s pairs with %q (%v), want %q", n.Event, c, ok, n.Category)
		}
		if want := n.Event == models.EventCaseClosed; models.NotificationEventUnmutable(n.Event) != want {
			t.Errorf("%s unmutable = %v, want %v", n.Event, !want, want)
		}
	}
}
