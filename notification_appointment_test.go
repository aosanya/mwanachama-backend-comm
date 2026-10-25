package mwanachamacomm

import (
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func appointmentNotices() []models.Notification {
	return []models.Notification{
		{ActorID: "applicant-1", Category: models.CategoryCase, Event: models.EventAppointmentBooked, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
		{ActorID: "applicant-1", Category: models.CategoryCase, Event: models.EventAppointmentCancelled, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
		{ActorID: "clerk-1", Category: models.CategoryCase, Event: models.EventAppointmentCancelledByMember, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
		{ActorID: "applicant-1", Category: models.CategoryCase, Event: models.EventCaseClosed, SubjectKind: models.SubjectCase, SubjectID: "case-1"},
	}
}

func TestAppointmentAndRequestNoticesAreDeclaredByTheCivicSpec(t *testing.T) {
	civic := testSpec(t)
	for _, n := range appointmentNotices() {
		if err := Check(civic, roleNotification, n); err != nil {
			t.Errorf("civic refused %s: %v", n.Event, err)
		}
	}
}

func TestAppointmentNoticesAreNotTheSchoolsVocabulary(t *testing.T) {
	school := shippedSpecs(t)["school"]
	for _, n := range appointmentNotices() {
		probe := models.Notification{ActorID: n.ActorID, SubjectID: n.SubjectID, Event: n.Event}
		if err := Check(school, roleNotification, probe); !errors.Is(err, models.ErrNotificationInvalid) {
			t.Errorf("school accepted %+v: %v", probe, err)
		}
	}
}

func TestEachAppointmentEventPairsWithTheCaseCategory(t *testing.T) {
	for _, n := range appointmentNotices() {
		c, ok := models.NotificationCategoryOf(n.Event)
		if !ok || c != models.CategoryCase {
			t.Errorf("%s pairs with %q (%v), want case", n.Event, c, ok)
		}
		if want := n.Event == models.EventCaseClosed; models.NotificationEventUnmutable(n.Event) != want {
			t.Errorf("%s unmutable = %v, want %v", n.Event, !want, want)
		}
	}
}
