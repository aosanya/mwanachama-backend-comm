package mwanachamacomm_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	comm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func pushKeys() models.PushKeys {
	enc := base64.RawURLEncoding
	return models.PushKeys{
		P256dh: enc.EncodeToString(append([]byte{4}, make([]byte, 64)...)),
		Auth:   enc.EncodeToString(make([]byte, 16)),
	}
}

func TestReleasingAnActorStopsTheirPushes(t *testing.T) {
	ctx := context.Background()
	m := gatedManager(t, nil)
	for actor, endpoint := range map[string]string{"mem-1": "https://push.example.org/a", "mem-2": "https://push.example.org/b"} {
		if _, err := m.SubscribePushFor(ctx, actor, "dev-1", endpoint, pushKeys()); err != nil {
			t.Fatalf("SubscribePushFor %s: %v", actor, err)
		}
	}
	if err := m.ReleaseActor(ctx, "mem-1"); err != nil {
		t.Fatalf("ReleaseActor: %v", err)
	}
	if subs, _ := m.ListPushSubscriptions(ctx, "mem-1"); len(subs) != 0 {
		t.Fatalf("released actor keeps %d subscriptions", len(subs))
	}
	if subs, _ := m.ListPushSubscriptions(ctx, "mem-2"); len(subs) != 1 {
		t.Fatalf("other actor has %d subscriptions, want 1", len(subs))
	}
}

func TestReleasingNobodyIsRefused(t *testing.T) {
	if err := gatedManager(t, nil).ReleaseActor(context.Background(), ""); !errors.Is(err, comm.ErrPushSubscriptionInvalid) {
		t.Fatalf("ReleaseActor empty = %v, want ErrPushSubscriptionInvalid", err)
	}
}

func TestWithdrawalHidesTheActorsMessagesWithoutDeletingThem(t *testing.T) {
	ctx := context.Background()
	m := gatedManager(t, nil)
	ids := map[string]string{}
	for _, author := range []string{"mem-1", "mem-2"} {
		posted, err := m.PostChatMessage(ctx, models.ChatMessage{StructureID: "s-1", ThreadID: "t-1", AuthorID: author, Body: "hello from " + author})
		if err != nil {
			t.Fatalf("PostChatMessage %s: %v", author, err)
		}
		ids[author] = posted.ID
	}
	if err := m.HideActorMessages(ctx, "mem-1"); err != nil {
		t.Fatalf("HideActorMessages: %v", err)
	}
	shown, err := m.ListChatMessages(ctx, "s-1", "t-1")
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(shown) != 1 || shown[0].AuthorID != "mem-2" {
		t.Fatalf("listed messages = %+v, want only mem-2's", shown)
	}
	kept, err := m.GetChatMessage(ctx, ids["mem-1"])
	if err != nil || kept.WithdrawnAt == "" || kept.Body == "" {
		t.Fatalf("withdrawn message = %+v (%v), want it kept and marked", kept, err)
	}
}

func TestHidingMessagesOfNobodyIsRefused(t *testing.T) {
	if err := gatedManager(t, nil).HideActorMessages(context.Background(), ""); !errors.Is(err, models.ErrChatInvalid) {
		t.Fatalf("HideActorMessages empty = %v, want ErrChatInvalid", err)
	}
}
