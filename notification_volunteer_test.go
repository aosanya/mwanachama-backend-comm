package mwanachamacomm

import (
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func volunteerNotices() []models.Notification {
	return []models.Notification{
		{ActorID: "member-1", Category: models.CategoryPost, Event: models.EventVolunteerPicked, SubjectKind: models.SubjectPost, SubjectID: "post-1"},
		{ActorID: "member-2", Category: models.CategoryPost, Event: models.EventVolunteerCallFilled, SubjectKind: models.SubjectPost, SubjectID: "post-1"},
	}
}

func TestVolunteerNoticesAreDeclaredByTheCivicSpec(t *testing.T) {
	civic := testSpec(t)
	for _, n := range volunteerNotices() {
		if err := Check(civic, roleNotification, n); err != nil {
			t.Errorf("civic refused %s: %v", n.Event, err)
		}
	}
}

func TestVolunteerNoticesAreNotTheSchoolsVocabulary(t *testing.T) {
	school := shippedSpecs(t)["school"]
	for _, n := range volunteerNotices() {
		probe := models.Notification{ActorID: n.ActorID, SubjectID: n.SubjectID, Event: n.Event}
		if err := Check(school, roleNotification, probe); !errors.Is(err, models.ErrNotificationInvalid) {
			t.Errorf("school accepted %+v: %v", probe, err)
		}
	}
}

func TestVolunteerNoticesPairWithThePostCategory(t *testing.T) {
	for _, e := range []models.NotificationEvent{models.EventVolunteerPicked, models.EventVolunteerCallFilled} {
		c, ok := models.NotificationCategoryOf(e)
		if !ok || c != models.CategoryPost {
			t.Errorf("%s pairs with %q (%v), want post", e, c, ok)
		}
	}
}
