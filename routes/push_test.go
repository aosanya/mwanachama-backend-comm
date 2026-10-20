package routes_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"testing"
)

func browserKeys(t *testing.T) map[string]string {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"p256dh": base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		"auth":   base64.RawURLEncoding.EncodeToString(auth),
	}
}

func TestASubscriptionBelongsToTheSessionNotTheBody(t *testing.T) {
	srv, m := newCommServer(t)
	got := callWithDevice(t, srv, http.MethodPost, "/push-subscriptions", "m-1", "phone-1", map[string]any{
		"endpoint":       "https://push.example.org/a",
		"expirationTime": nil,
		"keys":           browserKeys(t),
		"actor_id":       "m-2",
		"device_id":      "phone-9",
	})
	if got.code != http.StatusCreated {
		t.Fatalf("subscribe = %d: %s", got.code, got.body)
	}
	var out map[string]any
	got.decode(t, &out)
	if out["actor_id"] != "m-1" || out["device_id"] != "phone-1" {
		t.Fatalf("subscription = %v, want it owned by the session's caller and handset", out)
	}
	for _, secret := range []string{"auth_secret", "public_key", "keys", "AuthSecret", "PublicKey"} {
		if _, leaked := out[secret]; leaked {
			t.Fatalf("the response carries %q: %s", secret, got.body)
		}
	}

	if theirs, _ := m.ListPushSubscriptions(context.Background(), "m-2"); len(theirs) != 0 {
		t.Fatalf("a body naming m-2 subscribed m-2")
	}
}

func TestASubscriptionThatCannotBeSentToIsABadRequest(t *testing.T) {
	srv, _ := newCommServer(t)
	for name, body := range map[string]map[string]any{
		"no keys":      {"endpoint": "https://push.example.org/a"},
		"an http URL":  {"endpoint": "http://push.example.org/a", "keys": browserKeys(t)},
		"an internal":  {"endpoint": "https://169.254.169.254/latest", "keys": browserKeys(t)},
		"short keys":   {"endpoint": "https://push.example.org/a", "keys": map[string]string{"p256dh": "AAAA", "auth": "AAAA"}},
		"no endpoint":  {"keys": browserKeys(t)},
		"keys as text": {"endpoint": "https://push.example.org/a", "keys": "nope"},
	} {
		if got := call(t, srv, http.MethodPost, "/push-subscriptions", "m-1", body); got.code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400: %s", name, got.code, got.body)
		}
	}
}

func TestUnsubscribingOverTheMuxReachesOnlyTheCallersOwn(t *testing.T) {
	srv, m := newCommServer(t)
	endpoint := map[string]any{"endpoint": "https://push.example.org/a", "keys": browserKeys(t)}
	if got := call(t, srv, http.MethodPost, "/push-subscriptions", "m-1", endpoint); got.code != http.StatusCreated {
		t.Fatalf("subscribe = %d: %s", got.code, got.body)
	}

	stop := map[string]string{"endpoint": "https://push.example.org/a"}
	if got := call(t, srv, http.MethodPost, "/push-subscriptions/unsubscribe", "m-2", stop); got.code != http.StatusNoContent {
		t.Fatalf("another caller's unsubscribe = %d: %s", got.code, got.body)
	}
	if mine, _ := m.ListPushSubscriptions(context.Background(), "m-1"); len(mine) != 1 {
		t.Fatalf("another caller removed m-1's subscription")
	}
	if got := call(t, srv, http.MethodPost, "/push-subscriptions/unsubscribe", "m-1", stop); got.code != http.StatusNoContent {
		t.Fatalf("unsubscribe = %d: %s", got.code, got.body)
	}
	if mine, _ := m.ListPushSubscriptions(context.Background(), "m-1"); len(mine) != 0 {
		t.Fatalf("m-1 still holds %d subscriptions", len(mine))
	}
}
