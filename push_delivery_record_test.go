package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestEveryPushAttemptIsRecordedPerSubscriptionAndOutlivesADroppedOne(t *testing.T) {
	m := newPushManager(t)
	sender := &recordingSender{answers: map[string]error{
		"https://push.example.org/gone":  models.ErrPushSubscriptionGone,
		"https://push.example.org/flaky": errors.New("push: the push service answered 503"),
	}}
	m.UseWebPush(sender, func(error) {})
	gone := subscribe(t, m, "m-1", "https://push.example.org/gone")
	flaky := subscribe(t, m, "m-1", "https://push.example.org/flaky")
	fine := subscribe(t, m, "m-1", "https://push.example.org/fine")
	subscribe(t, m, "m-2", "https://push.example.org/other")

	n := raiseResults(t, m, "m-1")
	m.DrainPushes()

	got, err := m.PushDeliveriesOf(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("PushDeliveriesOf: %v", err)
	}
	want := map[string]models.PushDeliveryState{
		gone.ID:  models.PushDeliveryGone,
		flaky.ID: models.PushDeliveryFailed,
		fine.ID:  models.PushDeliverySent,
	}
	if len(got) != len(want) {
		t.Fatalf("%d attempts recorded (%+v), want one per subscription the recipient held", len(got), got)
	}
	for _, d := range got {
		if want[d.SubscriptionID] != d.State {
			t.Errorf("subscription %s recorded %q, want %q", d.SubscriptionID, d.State, want[d.SubscriptionID])
		}
		if d.NotificationID != n.ID || d.ActorID != "m-1" || d.At.IsZero() || d.ID == "" {
			t.Errorf("attempt %+v does not name its notification, recipient, id and time", d)
		}
	}
}

func TestANotificationNeverPushedHasNoAttempts(t *testing.T) {
	m := newPushManager(t)
	n := raiseResults(t, m, "m-1")
	got, err := m.PushDeliveriesOf(context.Background(), n.ID)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want no attempts without a sender", got, err)
	}
}
