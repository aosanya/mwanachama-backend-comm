package mwanachamacomm

import (
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func marketNotices() []models.Notification {
	return []models.Notification{
		{ActorID: "supplier-1", Category: models.CategoryMarket, Event: models.EventSupplierApproved, SubjectKind: models.SubjectPoster, SubjectID: "poster-1"},
		{ActorID: "supplier-1", Category: models.CategoryMarket, Event: models.EventAskWaiting, SubjectKind: models.SubjectAsk, SubjectID: "ask-1"},
		{ActorID: "buyer-1", Category: models.CategoryMarket, Event: models.EventAskAnswered, SubjectKind: models.SubjectAsk, SubjectID: "ask-1"},
		{ActorID: "farmer-1", Category: models.CategoryMarket, Event: models.EventRevisionAsked, SubjectKind: models.SubjectGroupOrder, SubjectID: "go-1"},
	}
}

func TestMarketNoticesAreDeclaredByTheCivicSpec(t *testing.T) {
	civic := testSpec(t)
	for _, n := range marketNotices() {
		if err := Check(civic, roleNotification, n); err != nil {
			t.Errorf("civic refused %s: %v", n.Event, err)
		}
	}
}

func TestMarketNoticesAreNotTheSchoolsVocabulary(t *testing.T) {
	school := shippedSpecs(t)["school"]
	for _, n := range marketNotices() {
		probe := models.Notification{ActorID: n.ActorID, SubjectID: n.SubjectID, Event: n.Event}
		if err := Check(school, roleNotification, probe); !errors.Is(err, models.ErrNotificationInvalid) {
			t.Errorf("school accepted %+v: %v", probe, err)
		}
	}
}

func TestMarketNoticesPairWithTheMarketCategory(t *testing.T) {
	for _, n := range marketNotices() {
		c, ok := models.NotificationCategoryOf(n.Event)
		if !ok || c != models.CategoryMarket {
			t.Errorf("%s pairs with %q (%v), want market", n.Event, c, ok)
		}
	}
}
