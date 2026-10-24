package mwanachamacomm_test

import (
	"context"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestPruningRemovesOnlyDeliveriesOlderThanTheCutoff(t *testing.T) {
	m := newPushManager(t)
	ctx := context.Background()
	if _, err := m.RecordPushDelivery(ctx, models.PushDelivery{NotificationID: "n-1", ActorID: "m-1", SubscriptionID: "s-1", State: models.PushDeliverySent}); err != nil {
		t.Fatal(err)
	}

	kept, err := m.PrunePushDeliveries(ctx)
	if err != nil || kept != 0 {
		t.Fatalf("PrunePushDeliveries = %d, %v; want a fresh row kept", kept, err)
	}
	if got, _ := m.PushDeliveriesOf(ctx, "n-1"); len(got) != 1 {
		t.Fatalf("%d attempts after pruning a fresh row, want 1", len(got))
	}

	removed, err := m.PushStore.PrunePushDeliveries(ctx, time.Now().Add(time.Hour))
	if err != nil || removed != 1 {
		t.Fatalf("pruning past the row = %d, %v; want 1 removed", removed, err)
	}
	if got, _ := m.PushDeliveriesOf(ctx, "n-1"); len(got) != 0 {
		t.Fatalf("%d attempts left after pruning, want none", len(got))
	}
}
