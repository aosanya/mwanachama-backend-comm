package routes_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestANotificationsDeliveryRecordIsReadAtItsAddress(t *testing.T) {
	srv, m := newCommServer(t)
	for _, d := range []models.PushDelivery{
		{NotificationID: "n-1", ActorID: "m-1", SubscriptionID: "s-1", State: models.PushDeliverySent},
		{NotificationID: "n-1", ActorID: "m-1", SubscriptionID: "s-2", State: models.PushDeliveryGone},
		{NotificationID: "n-2", ActorID: "m-2", SubscriptionID: "s-3", State: models.PushDeliverySent},
	} {
		if _, err := m.RecordPushDelivery(context.Background(), d); err != nil {
			t.Fatalf("RecordPushDelivery: %v", err)
		}
	}

	got := call(t, srv, http.MethodGet, "/notifications/n-1/deliveries", "o-1", nil)
	if got.code != http.StatusOK {
		t.Fatalf("read = %d: %s", got.code, got.body)
	}
	var out []models.PushDelivery
	got.decode(t, &out)
	if len(out) != 2 {
		t.Fatalf("read %d attempts (%s), want the two n-1 made", len(out), got.body)
	}
	for _, d := range out {
		if d.NotificationID != "n-1" {
			t.Fatalf("attempt %+v belongs to another notification", d)
		}
	}

	none := call(t, srv, http.MethodGet, "/notifications/n-9/deliveries", "o-1", nil)
	if none.code != http.StatusOK || string(none.body) == "" {
		t.Fatalf("unknown notification = %d: %s", none.code, none.body)
	}
}
