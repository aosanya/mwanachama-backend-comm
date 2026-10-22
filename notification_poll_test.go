package mwanachamacomm

import (
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func pollNotices() []models.Notification {
	return []models.Notification{
		{ActorID: "member-1", Category: models.CategorySurvey, Event: models.EventPollOpened, SubjectKind: models.SubjectPoll, SubjectID: "poll-1"},
		{ActorID: "member-1", Category: models.CategoryResults, Event: models.EventPollResultPublished, SubjectKind: models.SubjectPoll, SubjectID: "poll-1"},
	}
}

func TestPollNoticesAreDeclaredByTheCivicSpec(t *testing.T) {
	civic := testSpec(t)
	for _, n := range pollNotices() {
		if err := Check(civic, roleNotification, n); err != nil {
			t.Errorf("civic refused %s: %v", n.Event, err)
		}
	}
}

func TestPollNoticesAreNotTheSchoolsVocabulary(t *testing.T) {
	school := shippedSpecs(t)["school"]
	for _, n := range pollNotices() {
		probe := models.Notification{ActorID: n.ActorID, SubjectID: n.SubjectID, Event: n.Event}
		if err := Check(school, roleNotification, probe); !errors.Is(err, models.ErrNotificationInvalid) {
			t.Errorf("school accepted %+v: %v", probe, err)
		}
	}
}

func TestAPollOpeningIsToldAsANewFormIs(t *testing.T) {
	poll, _ := models.NotificationCategoryOf(models.EventPollOpened)
	form, _ := models.NotificationCategoryOf(models.EventSurveyReached)
	if poll != form {
		t.Fatalf("poll_opened pairs with %q, a new form with %q", poll, form)
	}
}

func TestAPollResultPairsWithTheResultsCategory(t *testing.T) {
	c, ok := models.NotificationCategoryOf(models.EventPollResultPublished)
	if !ok || c != models.CategoryResults {
		t.Fatalf("poll_result_published pairs with %q (%v), want results", c, ok)
	}
}
