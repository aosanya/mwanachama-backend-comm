package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func audienceTemplate() models.Notification {
	return models.Notification{
		Category:    models.CategoryResults,
		Event:       models.EventPollResultPublished,
		SubjectKind: models.SubjectPoll,
		SubjectID:   "poll-1",
	}
}

func fixedAudience(actors ...string) mwanachamacomm.Audience {
	return func(_ context.Context, kind models.NotificationSubjectKind, id string) ([]string, error) {
		if kind != models.SubjectPoll || id != "poll-1" {
			return nil, nil
		}
		return actors, nil
	}
}

func TestEveryActorBehindTheRecordGetsTheirOwnNotification(t *testing.T) {
	m := newPushManager(t)
	ctx := context.Background()
	raisers := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		raisers = append(raisers, "raiser-"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	m.UseAudience(fixedAudience(append(raisers, raisers[0], "")...))
	out, err := m.RaiseToAudience(ctx, audienceTemplate())
	if err != nil {
		t.Fatalf("RaiseToAudience: %v", err)
	}
	if out.Resolved != 40 || len(out.Raised) != 40 || len(out.Missed) != 0 || !out.Complete() {
		t.Fatalf("resolved %d raised %d missed %d, want 40/40/0", out.Resolved, len(out.Raised), len(out.Missed))
	}
	for _, a := range raisers {
		got, err := m.NotificationStore.List(ctx, a, 0)
		if err != nil || len(got) != 1 || got[0].SubjectID != "poll-1" || got[0].Event != models.EventPollResultPublished {
			t.Fatalf("%s holds %v (err %v), want one notice about poll-1", a, got, err)
		}
	}
}

func TestAMutedActorIsReportedAsMissedNotDropped(t *testing.T) {
	m := newPushManager(t)
	ctx := context.Background()
	if _, err := m.NotificationStore.SetPreference(ctx, "quiet", models.CategoryResults, true); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	m.UseAudience(fixedAudience("loud", "quiet"))
	out, err := m.RaiseToAudience(ctx, audienceTemplate())
	if err != nil {
		t.Fatalf("RaiseToAudience: %v", err)
	}
	if out.Resolved != 2 || len(out.Raised) != 1 || len(out.Missed) != 1 || !out.Complete() {
		t.Fatalf("resolved %d raised %d missed %d, want 2/1/1", out.Resolved, len(out.Raised), len(out.Missed))
	}
	if out.Missed[0].ActorID != "quiet" || !errors.Is(out.Missed[0].Reason, models.ErrNotificationMuted) {
		t.Fatalf("missed %+v, want quiet muted", out.Missed[0])
	}
}

func TestAManagerGivenNoAudienceRefusesRatherThanRaisingNothing(t *testing.T) {
	m := newPushManager(t)
	if _, err := m.RaiseToAudience(context.Background(), audienceTemplate()); !errors.Is(err, mwanachamacomm.ErrNotificationNoAudience) {
		t.Fatalf("got %v, want ErrNotificationNoAudience", err)
	}
	m.UseAudience(fixedAudience())
	if _, err := m.RaiseToAudience(context.Background(), audienceTemplate()); !errors.Is(err, mwanachamacomm.ErrNotificationNoAudience) {
		t.Fatalf("empty set: got %v, want ErrNotificationNoAudience", err)
	}
}

func TestAnAudienceRaiseNamingOneActorOrNoRecordIsRefused(t *testing.T) {
	m := newPushManager(t)
	m.UseAudience(fixedAudience("a"))
	named := audienceTemplate()
	named.ActorID = "a"
	if _, err := m.RaiseToAudience(context.Background(), named); !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("named actor: got %v, want ErrNotificationInvalid", err)
	}
	bare := audienceTemplate()
	bare.SubjectID = ""
	if _, err := m.RaiseToAudience(context.Background(), bare); !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("no record: got %v, want ErrNotificationInvalid", err)
	}
}

func TestAnInvalidTemplateWritesNothingForAnyone(t *testing.T) {
	m := newPushManager(t)
	m.UseAudience(fixedAudience("a", "b"))
	bad := audienceTemplate()
	bad.Category = models.CategoryChat
	if _, err := m.RaiseToAudience(context.Background(), bad); !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("got %v, want ErrNotificationInvalid", err)
	}
	for _, a := range []string{"a", "b"} {
		if got, _ := m.NotificationStore.List(context.Background(), a, 0); len(got) != 0 {
			t.Fatalf("%s holds %d notices after a refused raise", a, len(got))
		}
	}
}
