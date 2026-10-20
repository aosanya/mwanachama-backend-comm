package mwanachamacomm_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

type throwawayBrowser struct {
	key        *ecdh.PrivateKey
	auth       []byte
	PublicKey  string
	AuthSecret string
}

func newThrowawayBrowser(t *testing.T) throwawayBrowser {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	return throwawayBrowser{
		key: key, auth: auth,
		PublicKey:  base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		AuthSecret: base64.RawURLEncoding.EncodeToString(auth),
	}
}

func (b throwawayBrowser) keys() models.PushKeys {
	return models.PushKeys{P256dh: b.PublicKey, Auth: b.AuthSecret}
}

type nopActWriter struct{}

func (nopActWriter) WriteAct(context.Context, *sql.Tx, models.ActEntry) error { return nil }

func newPushManager(t *testing.T) *mwanachamacomm.CommManager {
	t.Helper()
	db, s := newTestDB(t)
	m, err := mwanachamacomm.NewCommManager(db, s, monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), nopActWriter{}, "")
	if err != nil {
		t.Fatalf("NewCommManager: %v", err)
	}
	return m
}

type sent struct {
	endpoint string
	payload  []byte
}

type recordingSender struct {
	mu      sync.Mutex
	sent    []sent
	answers map[string]error
}

func (r *recordingSender) Send(_ context.Context, sub models.PushSubscription, payload []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, sent{endpoint: sub.Endpoint, payload: payload})
	return r.answers[sub.Endpoint]
}

func (r *recordingSender) to() map[string][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string][]byte{}
	for _, s := range r.sent {
		out[s.endpoint] = s.payload
	}
	return out
}

func raiseResults(t *testing.T, m *mwanachamacomm.CommManager, actorID string) models.Notification {
	t.Helper()
	n, err := m.Raise(context.Background(), mwanachamacomm.Notification{
		ActorID: actorID, Category: mwanachamacomm.CategoryResults,
		Event: mwanachamacomm.EventResultsPublished, SubjectKind: mwanachamacomm.SubjectSurvey, SubjectID: "s-1",
	})
	if err != nil {
		t.Fatalf("Raise: %v", err)
	}
	return n
}

func subscribe(t *testing.T, m *mwanachamacomm.CommManager, actorID, endpoint string) models.PushSubscription {
	t.Helper()
	sub, err := m.SubscribePushFor(context.Background(), actorID, "", endpoint, newThrowawayBrowser(t).keys())
	if err != nil {
		t.Fatalf("SubscribePushFor: %v", err)
	}
	return sub
}

func TestASubscriptionIsStoredPerEndpointAndMovesWithItsBrowser(t *testing.T) {
	m := newPushManager(t)
	ctx := context.Background()

	first := subscribe(t, m, "m-1", "https://push.example.org/a")
	again := subscribe(t, m, "m-1", "https://push.example.org/a")
	if again.ID != first.ID {
		t.Fatalf("re-subscribing the same endpoint minted a second row: %s then %s", first.ID, again.ID)
	}
	subscribe(t, m, "m-1", "https://push.example.org/b")

	mine, err := m.ListPushSubscriptions(ctx, "m-1")
	if err != nil || len(mine) != 2 {
		t.Fatalf("m-1 holds %d subscriptions (%v), want 2", len(mine), err)
	}

	subscribe(t, m, "m-2", "https://push.example.org/a")
	mine, _ = m.ListPushSubscriptions(ctx, "m-1")
	theirs, _ := m.ListPushSubscriptions(ctx, "m-2")
	if len(mine) != 1 || len(theirs) != 1 || theirs[0].Endpoint != "https://push.example.org/a" {
		t.Fatalf("after m-2 subscribed m-1's browser: m-1 %d, m-2 %v; want the endpoint to move", len(mine), theirs)
	}
}

func TestUnsubscribingRemovesOnlyTheCallersOwn(t *testing.T) {
	m := newPushManager(t)
	ctx := context.Background()
	subscribe(t, m, "m-1", "https://push.example.org/a")

	if err := m.UnsubscribePushFor(ctx, "m-2", "https://push.example.org/a"); err != nil {
		t.Fatalf("unsubscribing somebody else's endpoint: %v", err)
	}
	if mine, _ := m.ListPushSubscriptions(ctx, "m-1"); len(mine) != 1 {
		t.Fatalf("another caller removed m-1's subscription")
	}
	if err := m.UnsubscribePushFor(ctx, "m-1", "https://push.example.org/a"); err != nil {
		t.Fatalf("UnsubscribePushFor: %v", err)
	}
	if mine, _ := m.ListPushSubscriptions(ctx, "m-1"); len(mine) != 0 {
		t.Fatalf("m-1 still holds %d subscriptions", len(mine))
	}
	if err := m.UnsubscribePushFor(ctx, "m-1", "https://push.example.org/a"); err != nil {
		t.Fatalf("a repeated unsubscribe: %v", err)
	}
}

func TestASubscriptionIsRefusedUnlessItCanBeSentTo(t *testing.T) {
	m := newPushManager(t)
	good := newThrowawayBrowser(t).keys()
	for name, c := range map[string]struct {
		actor, endpoint string
		keys            models.PushKeys
	}{
		"no subscriber":       {"", "https://push.example.org/a", good},
		"plain http":          {"m-1", "http://push.example.org/a", good},
		"not a URL":           {"m-1", "push", good},
		"an address literal":  {"m-1", "https://10.0.0.7/a", good},
		"localhost":           {"m-1", "https://localhost/a", good},
		"a bare host":         {"m-1", "https://intranet/a", good},
		"credentials in it":   {"m-1", "https://u:p@push.example.org/a", good},
		"a short public key":  {"m-1", "https://push.example.org/a", models.PushKeys{P256dh: "AAAA", Auth: good.Auth}},
		"no auth secret":      {"m-1", "https://push.example.org/a", models.PushKeys{P256dh: good.P256dh}},
		"an auth secret of 8": {"m-1", "https://push.example.org/a", models.PushKeys{P256dh: good.P256dh, Auth: "AAAAAAAAAAA"}},
	} {
		_, err := m.SubscribePushFor(context.Background(), c.actor, "", c.endpoint, c.keys)
		if !errors.Is(err, models.ErrPushSubscriptionInvalid) && !errors.Is(err, mwanachamacomm.ErrInvalidReference) {
			t.Errorf("%s: err = %v, want a refusal", name, err)
		}
	}
}

func TestARaisedNotificationIsPushedToEverySubscriptionItsRecipientHolds(t *testing.T) {
	m := newPushManager(t)
	sender := &recordingSender{}
	m.UseWebPush(sender, nil)
	subscribe(t, m, "m-1", "https://push.example.org/phone")
	subscribe(t, m, "m-1", "https://push.example.org/laptop")
	subscribe(t, m, "m-2", "https://push.example.org/other")

	n := raiseResults(t, m, "m-1")
	m.DrainPushes()

	got := sender.to()
	if len(got) != 2 || got["https://push.example.org/other"] != nil {
		t.Fatalf("pushed to %d endpoints (%v), want m-1's two", len(got), got)
	}
	var payload struct {
		ID, Category, Event string
		SubjectID           string `json:"subject_id"`
	}
	if err := json.Unmarshal(got["https://push.example.org/phone"], &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.ID != n.ID || payload.Event != string(n.Event) || payload.SubjectID != "s-1" {
		t.Fatalf("payload = %+v, want the notification it was raised for", payload)
	}
}

func TestASubscriptionThePushServiceReportsGoneIsDropped(t *testing.T) {
	m := newPushManager(t)
	var failures []error
	sender := &recordingSender{answers: map[string]error{
		"https://push.example.org/gone":  models.ErrPushSubscriptionGone,
		"https://push.example.org/flaky": errors.New("push: the push service answered 503"),
	}}
	m.UseWebPush(sender, func(err error) { failures = append(failures, err) })
	subscribe(t, m, "m-1", "https://push.example.org/gone")
	subscribe(t, m, "m-1", "https://push.example.org/flaky")
	subscribe(t, m, "m-1", "https://push.example.org/fine")

	raiseResults(t, m, "m-1")
	m.DrainPushes()

	left, _ := m.ListPushSubscriptions(context.Background(), "m-1")
	if len(left) != 2 {
		t.Fatalf("%d subscriptions left, want the gone one dropped and the other two kept", len(left))
	}
	for _, s := range left {
		if s.Endpoint == "https://push.example.org/gone" {
			t.Fatalf("the gone subscription was kept")
		}
	}
	if len(failures) != 1 {
		t.Fatalf("reported %d failures (%v), want the one transient refusal", len(failures), failures)
	}
}

func TestNothingIsPushedForAMutedCategoryOrWithoutASender(t *testing.T) {
	m := newPushManager(t)
	subscribe(t, m, "m-1", "https://push.example.org/phone")
	raiseResults(t, m, "m-1")
	m.DrainPushes()

	sender := &recordingSender{}
	m.UseWebPush(sender, nil)
	if _, err := m.SetNotificationPreferenceFor(context.Background(), "m-1", mwanachamacomm.CategoryChat, true); err != nil {
		t.Fatalf("mute: %v", err)
	}
	_, err := m.Raise(context.Background(), mwanachamacomm.Notification{
		ActorID: "m-1", Category: mwanachamacomm.CategoryChat, Event: "message_removed",
		SubjectKind: "message", SubjectID: "msg-1",
	})
	if !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
		t.Fatalf("Raise of a muted category = %v", err)
	}
	m.DrainPushes()
	if got := sender.to(); len(got) != 0 {
		t.Fatalf("a muted notification was pushed to %v", got)
	}
}
