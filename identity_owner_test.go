package mwanachamacomm_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/identity"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func holdOneOfEachIdentified(t *testing.T, m *mwanachamacomm.CommManager) map[string]string {
	t.Helper()
	ctx := context.Background()
	thread, err := m.MintThread(ctx, "ward-1", "/general")
	if err != nil {
		t.Fatal(err)
	}
	message, err := m.PostChatMessage(ctx, models.ChatMessage{StructureID: "ward-1", ThreadID: thread.ID, AuthorID: "a-1", Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	notice, err := m.Raise(ctx, models.Notification{
		ActorID: "a-1", Category: models.CategoryItem, Event: models.EventItemFiledForYou,
		SubjectKind: models.SubjectItem, SubjectID: "item-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ack, err := m.AcknowledgeNotification(ctx, "a-1", notice.ID)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"chat_thread": thread.ID, "chat_message": message.ID,
		"notification": notice.ID, "notification_acknowledgement": ack.ID,
	}
}

func TestStableIdentityIsTheRawCommInstanceMountObjectAndRowId(t *testing.T) {
	m, _, _ := newRulesManager(t)
	owner := m.Identity()
	for object, id := range holdOneOfEachIdentified(t, m) {
		got, err := owner.StableIdentity(context.Background(), object, id)
		if err != nil {
			t.Fatalf("StableIdentity(%s): %v", object, err)
		}
		if want := "comm/mwanachama/main/" + object + "/" + id; got != want {
			t.Errorf("StableIdentity(%s) = %q, want %q", object, got, want)
		}
	}
}

func TestResolveGivesBackTheObjectAndIdStableIdentityWasAskedFor(t *testing.T) {
	m, _, _ := newRulesManager(t)
	owner := m.Identity()
	ctx := context.Background()
	for object, id := range holdOneOfEachIdentified(t, m) {
		raw, err := owner.StableIdentity(ctx, object, id)
		if err != nil {
			t.Fatal(err)
		}
		gotObject, gotID, err := owner.Resolve(ctx, raw)
		if err != nil || gotObject != object || gotID != id {
			t.Errorf("Resolve(%q) = (%q, %q, %v), want (%q, %q)", raw, gotObject, gotID, err, object, id)
		}
	}
}

func TestStableIdentityNamesAMissAndAnObjectCommKeepsPrivate(t *testing.T) {
	m, _, _ := newRulesManager(t)
	owner := m.Identity()
	ctx := context.Background()
	if _, err := owner.StableIdentity(ctx, "notification", "missing"); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("a missing row: %v, want identity.ErrNotFound", err)
	}
	for _, private := range []string{"dm_thread", "dm_message", "dm_participant", "address", "push_subscription", "notification_preference"} {
		if _, err := owner.StableIdentity(ctx, private, "any"); !errors.Is(err, mwanachamacomm.ErrNoStableIdentity) {
			t.Errorf("%s: %v, want ErrNoStableIdentity", private, err)
		}
	}
}

func TestResolveNamesWhyARawIsNotAnswered(t *testing.T) {
	m, _, _ := newRulesManager(t)
	owner := m.Identity()
	ctx := context.Background()
	held := holdOneOfEachIdentified(t, m)
	raw, err := owner.StableIdentity(ctx, "notification", held["notification"])
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		raw  string
		want error
	}{
		"a string that does not parse":  {"not a raw", identity.ErrInvalidRaw},
		"another module's raw":          {strings.Replace(raw, "comm/", "agency/", 1), identity.ErrNotOwned},
		"another instance's raw":        {strings.Replace(raw, "/mwanachama/", "/other/", 1), identity.ErrNotOwned},
		"another mount's raw":           {strings.Replace(raw, "/main/", "/harvest/", 1), identity.ErrNotOwned},
		"an identity that is not three": {"comm/mwanachama/main/" + held["notification"], identity.ErrNotFound},
		"a private object":              {"comm/mwanachama/main/dm_thread/" + held["notification"], identity.ErrNotFound},
		"a row that was never written":  {"comm/mwanachama/main/notification/missing", identity.ErrNotFound},
	}
	for name, c := range cases {
		if _, _, err := owner.Resolve(ctx, c.raw); !errors.Is(err, c.want) {
			t.Errorf("%s: Resolve(%q) err = %v, want %v", name, c.raw, err, c.want)
		}
	}
}

func TestAnIdentityIsNotAnsweredForAThingOfAnotherObject(t *testing.T) {
	m, _, _ := newRulesManager(t)
	owner := m.Identity()
	held := holdOneOfEachIdentified(t, m)
	raw := "comm/mwanachama/main/chat_message/" + held["notification"]
	if _, _, err := owner.Resolve(context.Background(), raw); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("a notification's id under chat_message: err = %v, want identity.ErrNotFound", err)
	}
}
