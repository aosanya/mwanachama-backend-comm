package mwanachamacomm

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func cover13PushKeys() models.PushKeys {
	return models.PushKeys{
		P256dh: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, models.PushPublicKeyBytes)),
		Auth:   base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{9}, models.PushAuthSecretBytes)),
	}
}

func TestCover13ASubscriptionIsKeptPerEndpointAndTheNewestOwnerWins(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	first, err := f.m.SubscribePushFor(ctx, "mem-1", "phone", "  https://push.example.com/send/abc ", cover13PushKeys())
	if err != nil {
		t.Fatalf("SubscribePushFor: %v", err)
	}
	if first.Endpoint != "https://push.example.com/send/abc" || first.ActorID != "mem-1" || first.DeviceID != "phone" {
		t.Fatalf("subscription = %+v; the endpoint is trimmed", first)
	}
	moved, err := f.m.SubscribePushFor(ctx, "mem-2", "tablet", "https://push.example.com/send/abc", cover13PushKeys())
	if err != nil || moved.ActorID != "mem-2" || moved.DeviceID != "tablet" {
		t.Fatalf("the same endpoint subscribed by someone else = %+v, %v; the endpoint belongs to the latest", moved, err)
	}
	mine, err := f.m.ListPushSubscriptions(ctx, "mem-1")
	if err != nil || len(mine) != 0 {
		t.Fatalf("mem-1 subscriptions = %+v, %v; the endpoint moved away", mine, err)
	}
	theirs, err := f.m.ListPushSubscriptions(ctx, "mem-2")
	if err != nil || len(theirs) != 1 {
		t.Fatalf("mem-2 subscriptions = %+v, %v", theirs, err)
	}
}

func TestCover13ASubscriptionWithAWrongPartIsRefusedAndNothingIsKept(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	good := cover13PushKeys()
	cases := map[string]struct {
		caller, endpoint string
		keys             models.PushKeys
	}{
		"nobody":     {"", "https://push.example.com/x", good},
		"plain http": {"mem-1", "http://push.example.com/x", good},
		"an ip host": {"mem-1", "https://10.0.0.5/x", good},
		"short key":  {"mem-1", "https://push.example.com/x", models.PushKeys{P256dh: "AAAA", Auth: good.Auth}},
		"bad secret": {"mem-1", "https://push.example.com/x", models.PushKeys{P256dh: good.P256dh, Auth: "***"}},
	}
	for name, c := range cases {
		if _, err := f.m.SubscribePushFor(ctx, c.caller, "", c.endpoint, c.keys); err == nil {
			t.Errorf("%s: a bad subscription was accepted", name)
		}
	}
	if subs, _ := f.m.ListPushSubscriptions(ctx, "mem-1"); len(subs) != 0 {
		t.Fatalf("subscriptions = %+v; the refused ones left nothing behind", subs)
	}
}

func TestCover13UnsubscribingNeedsACallerAndOnlyRemovesTheirEndpoint(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	if _, err := f.m.SubscribePushFor(ctx, "mem-1", "", "https://push.example.com/one", cover13PushKeys()); err != nil {
		t.Fatalf("SubscribePushFor: %v", err)
	}
	if _, err := f.m.SubscribePushFor(ctx, "mem-1", "", "https://push.example.com/two", cover13PushKeys()); err != nil {
		t.Fatalf("SubscribePushFor: %v", err)
	}
	if err := f.m.UnsubscribePushFor(ctx, "  ", "https://push.example.com/one"); !errors.Is(err, models.ErrPushSubscriptionInvalid) {
		t.Fatalf("unsubscribing with no caller = %v, want ErrPushSubscriptionInvalid", err)
	}
	if err := f.m.UnsubscribePushFor(ctx, "mem-2", "https://push.example.com/one"); err != nil {
		t.Fatalf("UnsubscribePushFor: %v", err)
	}
	if subs, _ := f.m.ListPushSubscriptions(ctx, "mem-1"); len(subs) != 2 {
		t.Fatalf("another person's unsubscribe removed %d of mem-1's subscriptions", 2-len(subs))
	}
	if err := f.m.UnsubscribePushFor(ctx, "mem-1", " https://push.example.com/one "); err != nil {
		t.Fatalf("UnsubscribePushFor: %v", err)
	}
	subs, _ := f.m.ListPushSubscriptions(ctx, "mem-1")
	if len(subs) != 1 || subs[0].Endpoint != "https://push.example.com/two" {
		t.Fatalf("subscriptions after unsubscribing one = %+v", subs)
	}
	if err := f.m.DropPushSubscription(ctx, subs[0].ID); err != nil {
		t.Fatalf("DropPushSubscription: %v", err)
	}
	if subs, _ = f.m.ListPushSubscriptions(ctx, "mem-1"); len(subs) != 0 {
		t.Fatalf("subscriptions after dropping by id = %+v", subs)
	}
}

func TestCover13PushDeliveriesAreRecordedPerNotificationAndPrunedAfterNinetyDays(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	sub, err := f.m.SubscribePushFor(ctx, "mem-1", "", "https://push.example.com/one", cover13PushKeys())
	if err != nil {
		t.Fatalf("SubscribePushFor: %v", err)
	}
	record := func(notification string, state models.PushDeliveryState) models.PushDelivery {
		d, err := f.m.RecordPushDelivery(ctx, models.PushDelivery{NotificationID: notification, ActorID: "mem-1", SubscriptionID: sub.ID, State: state})
		if err != nil {
			t.Fatalf("RecordPushDelivery: %v", err)
		}
		return d
	}
	sent := record("n-1", models.PushDeliverySent)
	record("n-1", models.PushDeliveryGone)
	record("n-2", models.PushDeliveryFailed)
	if sent.ID == "" || sent.At.IsZero() {
		t.Fatalf("delivery = %+v; an id and a time are filled", sent)
	}
	got, err := f.m.PushDeliveriesOf(ctx, "n-1")
	if err != nil || len(got) != 2 || got[0].State != models.PushDeliverySent || got[1].State != models.PushDeliveryGone {
		t.Fatalf("deliveries of n-1 = %+v, %v; oldest first", got, err)
	}

	n, err := f.m.PrunePushDeliveries(ctx)
	if err != nil || n != 0 {
		t.Fatalf("pruning recent deliveries removed %d (%v), want none", n, err)
	}
	f.clock.at = f.clock.at.Add(PushDeliveryRetention + time.Hour)
	n, err = f.m.PrunePushDeliveries(ctx)
	if err != nil || n != 3 {
		t.Fatalf("pruning after the retention removed %d (%v), want all 3", n, err)
	}
	if got, _ := f.m.PushDeliveriesOf(ctx, "n-1"); len(got) != 0 {
		t.Fatalf("deliveries after pruning = %+v", got)
	}
}

func TestCover13UseWebPushDoesNothingOnATestRunAndCanBeSwitchedOff(t *testing.T) {
	f := cover13Setup(t)
	f.m.UseWebPush(nil, nil)
	if f.m.push != nil || f.m.NotificationStore.onRaised != nil {
		t.Fatal("no sender means no delivery and no hook")
	}
	f.m.DrainPushes()

	f.m.UseWebPush(cover13Sender{}, func(error) {})
	if f.m.push == nil || f.m.NotificationStore.onRaised == nil {
		t.Fatal("a sender installs the delivery and the hook")
	}
	f.m.DrainPushes()
	f.m.UseWebPush(nil, nil)
	if f.m.push != nil || f.m.NotificationStore.onRaised != nil {
		t.Fatal("removing the sender switches delivery off again")
	}

	f.m.runID = "run-1"
	f.m.UseWebPush(cover13Sender{}, nil)
	if f.m.push != nil {
		t.Fatal("a test run never pushes to a real device")
	}
}

type cover13Sender struct{}

func (cover13Sender) Send(context.Context, models.PushSubscription, []byte) error { return nil }
