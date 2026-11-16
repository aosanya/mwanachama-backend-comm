package mwanachamacomm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (f *cover13Fixture) dmThread(t *testing.T, creator string, invited ...string) models.DMThread {
	t.Helper()
	th, err := f.m.DMStore.CreateThread(context.Background(), models.DMThread{CreatedBy: creator, Title: "talk"}, invited)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	return th
}

func (f *cover13Fixture) stateOf(t *testing.T, thread, actor string) models.DMParticipantState {
	t.Helper()
	ps, err := f.m.DMStore.ListParticipants(context.Background(), thread)
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	for _, p := range ps {
		if p.ActorID == actor {
			return p.State
		}
	}
	return ""
}

func TestCover13CreatingAThreadMakesTheCreatorAnActiveAdminAndInvitesTheRest(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th := f.dmThread(t, "mem-1", "mem-1", "mem-2", "mem-3")
	if th.ID == "" || th.CreatedAt.IsZero() {
		t.Fatalf("thread = %+v; an id and a time are filled", th)
	}
	ps, err := f.m.DMStore.ListParticipants(ctx, th.ID)
	if err != nil || len(ps) != 3 {
		t.Fatalf("participants = %+v, %v; the creator listed among the initial actors is added once", ps, err)
	}
	byActor := map[string]models.DMParticipant{}
	for _, p := range ps {
		byActor[p.ActorID] = p
	}
	if p := byActor["mem-1"]; p.State != models.DMStateActive || !p.IsAdmin {
		t.Fatalf("creator = %+v, want active admin", p)
	}
	for _, id := range []string{"mem-2", "mem-3"} {
		if p := byActor[id]; p.State != models.DMStateInvited || p.IsAdmin {
			t.Fatalf("%s = %+v, want invited and not admin", id, p)
		}
	}
}

func TestCover13ThreadsAreListedForActiveAndInvitedParticipantsOnly(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	mine := f.dmThread(t, "mem-1", "mem-2")
	left := f.dmThread(t, "mem-3", "mem-2")
	f.dmThread(t, "mem-3")
	if _, err := f.m.DMStore.Accept(ctx, left.ID, "mem-2"); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if err := f.m.DMStore.Leave(ctx, left.ID, "mem-2"); err != nil {
		t.Fatalf("Leave: %v", err)
	}
	got, err := f.m.DMStore.ListThreadsFor(ctx, "mem-2")
	if err != nil || len(got) != 1 || got[0].ID != mine.ID {
		t.Fatalf("threads for mem-2 = %+v, %v; want only the one they are still in", got, err)
	}
	if _, err := f.m.DMStore.GetThread(ctx, "dmthread-missing"); !errors.Is(err, models.ErrDMNotFound) {
		t.Fatalf("GetThread of a missing id = %v, want ErrDMNotFound", err)
	}
	one, err := f.m.DMStore.GetThread(ctx, mine.ID)
	if err != nil || one.ID != mine.ID {
		t.Fatalf("GetThread = %+v, %v", one, err)
	}
}

func TestCover13OnlyAnActiveAdminInvitesAndAnActiveMemberCannotBeInvitedAgain(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th := f.dmThread(t, "mem-1", "mem-2")
	if _, err := f.m.DMStore.Invite(ctx, th.ID, "mem-4", "mem-2"); !errors.Is(err, models.ErrDMNotAdmin) {
		t.Fatalf("an invited non-admin inviting = %v, want ErrDMNotAdmin", err)
	}
	if _, err := f.m.DMStore.Invite(ctx, th.ID, "mem-4", "stranger"); !errors.Is(err, models.ErrDMNotAdmin) {
		t.Fatalf("a stranger inviting = %v, want ErrDMNotAdmin", err)
	}
	p, err := f.m.DMStore.Invite(ctx, th.ID, "mem-4", "mem-1")
	if err != nil || p.State != models.DMStateInvited {
		t.Fatalf("Invite = %+v, %v", p, err)
	}
	if _, err := f.m.DMStore.Invite(ctx, th.ID, "mem-1", "mem-1"); !errors.Is(err, models.ErrDMAlreadyActive) {
		t.Fatalf("inviting an active member = %v, want ErrDMAlreadyActive", err)
	}
	if _, err := f.m.DMStore.Accept(ctx, th.ID, "mem-4"); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if got := f.stateOf(t, th.ID, "mem-4"); got != models.DMStateActive {
		t.Fatalf("state after accept = %q", got)
	}
	if _, err := f.m.DMStore.Accept(ctx, th.ID, "nobody"); !errors.Is(err, models.ErrDMNotFound) {
		t.Fatalf("accepting without an invite = %v, want ErrDMNotFound", err)
	}
}

func TestCover13TheLastAdminCannotLeaveWhileOthersAreActive(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th := f.dmThread(t, "mem-1", "mem-2")
	if _, err := f.m.DMStore.Accept(ctx, th.ID, "mem-2"); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if err := f.m.DMStore.Leave(ctx, th.ID, "mem-1"); !errors.Is(err, models.ErrDMLastAdmin) {
		t.Fatalf("the only admin leaving = %v, want ErrDMLastAdmin", err)
	}
	if _, err := f.m.DMStore.Promote(ctx, th.ID, "mem-2", "mem-2"); !errors.Is(err, models.ErrDMNotAdmin) {
		t.Fatalf("a non-admin promoting themselves = %v, want ErrDMNotAdmin", err)
	}
	promoted, err := f.m.DMStore.Promote(ctx, th.ID, "mem-2", "mem-1")
	if err != nil || !promoted.IsAdmin {
		t.Fatalf("Promote = %+v, %v", promoted, err)
	}
	if err := f.m.DMStore.Leave(ctx, th.ID, "mem-1"); err != nil {
		t.Fatalf("leaving once a successor is admin = %v", err)
	}
	if got := f.stateOf(t, th.ID, "mem-1"); got != models.DMStateLeft {
		t.Fatalf("state after leaving = %q", got)
	}
	if _, err := f.m.DMStore.Promote(ctx, th.ID, "ghost", "mem-2"); !errors.Is(err, models.ErrDMNotFound) {
		t.Fatalf("promoting a non-participant = %v, want ErrDMNotFound", err)
	}
}

func TestCover13KickNeedsAnAdminAndNeverTheCaller(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th := f.dmThread(t, "mem-1", "mem-2", "mem-3")
	if err := f.m.DMStore.Kick(ctx, th.ID, "mem-1", "mem-1"); !errors.Is(err, models.ErrDMSelfKick) {
		t.Fatalf("self kick = %v, want ErrDMSelfKick", err)
	}
	if err := f.m.DMStore.Kick(ctx, th.ID, "mem-3", "mem-2"); !errors.Is(err, models.ErrDMNotAdmin) {
		t.Fatalf("kick by a non-admin = %v, want ErrDMNotAdmin", err)
	}
	if err := f.m.DMStore.Kick(ctx, th.ID, "mem-3", "mem-1"); err != nil {
		t.Fatalf("Kick: %v", err)
	}
	if got := f.stateOf(t, th.ID, "mem-3"); got != models.DMStateKicked {
		t.Fatalf("state after kick = %q", got)
	}
	if err := f.m.DMStore.Kick(ctx, th.ID, "ghost", "mem-1"); !errors.Is(err, models.ErrDMNotFound) {
		t.Fatalf("kicking a non-participant = %v, want ErrDMNotFound", err)
	}
}

func TestCover13ReEnableByAnAdminInvitesAgainAndTheLastToLeaveCanClaimTheThread(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th := f.dmThread(t, "mem-1", "mem-2")
	if _, err := f.m.DMStore.Accept(ctx, th.ID, "mem-2"); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if _, err := f.m.DMStore.ReEnable(ctx, th.ID, "ghost", "mem-1"); !errors.Is(err, models.ErrDMNotFound) {
		t.Fatalf("re-enabling a non-participant = %v, want ErrDMNotFound", err)
	}
	if err := f.m.DMStore.Kick(ctx, th.ID, "mem-2", "mem-1"); err != nil {
		t.Fatalf("Kick: %v", err)
	}
	if _, err := f.m.DMStore.ReEnable(ctx, th.ID, "mem-2", "mem-2"); !errors.Is(err, models.ErrDMNotLastToLeave) {
		t.Fatalf("a kicked member re-enabling themselves = %v, want ErrDMNotLastToLeave", err)
	}
	back, err := f.m.DMStore.ReEnable(ctx, th.ID, "mem-2", "mem-1")
	if err != nil || back.State != models.DMStateInvited {
		t.Fatalf("ReEnable by the admin = %+v, %v; want invited again", back, err)
	}
	if _, err := f.m.DMStore.ReEnable(ctx, th.ID, "mem-2", "mem-3"); !errors.Is(err, models.ErrDMNotAdmin) {
		t.Fatalf("ReEnable by a stranger = %v, want ErrDMNotAdmin", err)
	}

	solo := f.dmThread(t, "mem-5")
	if err := f.m.DMStore.Leave(ctx, solo.ID, "mem-5"); err != nil {
		t.Fatalf("sole member leaving = %v", err)
	}
	claimed, err := f.m.DMStore.ReEnable(ctx, solo.ID, "mem-5", "mem-5")
	if err != nil || claimed.State != models.DMStateActive {
		t.Fatalf("the last to leave claiming the emptied thread = %+v, %v", claimed, err)
	}
}

func TestCover13MessagesAreListedOldestFirstAndExpireWithTheThreadTTL(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	ttl := 30
	th, err := f.m.DMStore.CreateThread(ctx, models.DMThread{CreatedBy: "mem-1", MessageTTLSeconds: &ttl}, nil)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	old, err := f.m.DMStore.Post(ctx, models.DMMessage{ThreadID: th.ID, SenderID: "mem-1", PayloadCiphertext: "old"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	f.clock.at = f.clock.at.Add(time.Minute)
	fresh, err := f.m.DMStore.Post(ctx, models.DMMessage{ThreadID: th.ID, SenderID: "mem-1", PayloadCiphertext: "fresh"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	got, err := f.m.DMStore.ListMessages(ctx, th.ID)
	if err != nil || len(got) != 1 || got[0].ID != fresh.ID {
		t.Fatalf("messages = %+v, %v; the one past its 30 s TTL is gone", got, err)
	}
	back, err := f.m.DMStore.GetMessage(ctx, old.ID)
	if err != nil || back.PayloadCiphertext != "old" {
		t.Fatalf("GetMessage of an expired message still reads it by id: %+v, %v", back, err)
	}
	if _, err := f.m.DMStore.GetMessage(ctx, "dmmsg-missing"); !errors.Is(err, models.ErrDMNotFound) {
		t.Fatalf("GetMessage of a missing id = %v, want ErrDMNotFound", err)
	}
	none, err := f.m.DMStore.ListMessages(ctx, "dmthread-missing")
	if err != nil || len(none) != 0 {
		t.Fatalf("messages of a missing thread = %+v, %v", none, err)
	}
}

func TestCover13AReactionNeedsAMessageAndIsOnePerActor(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th := f.dmThread(t, "mem-1", "mem-2")
	msg, err := f.m.DMStore.Post(ctx, models.DMMessage{ThreadID: th.ID, SenderID: "mem-1", PayloadCiphertext: "hi"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if err := f.m.DMStore.SetReaction(ctx, models.DMReaction{MessageID: "dmmsg-missing", ActorID: "mem-2", Emoji: "👍"}); !errors.Is(err, models.ErrDMNotFound) {
		t.Fatalf("reacting to a missing message = %v, want ErrDMNotFound", err)
	}
	if err := f.m.DMStore.SetReaction(ctx, models.DMReaction{MessageID: msg.ID, ActorID: "mem-2", Emoji: "👍"}); err != nil {
		t.Fatalf("SetReaction: %v", err)
	}
	if err := f.m.DMStore.SetReaction(ctx, models.DMReaction{MessageID: msg.ID, ActorID: "mem-2", Emoji: "❤️"}); err != nil {
		t.Fatalf("SetReaction again: %v", err)
	}
	if err := f.m.DMStore.SetReaction(ctx, models.DMReaction{MessageID: msg.ID, ActorID: "mem-1", Emoji: "🎉"}); err != nil {
		t.Fatalf("SetReaction: %v", err)
	}
	got, err := f.m.DMStore.ListReactions(ctx, th.ID)
	if err != nil || len(got) != 2 {
		t.Fatalf("reactions = %+v, %v; one per actor", got, err)
	}
	if got[0].ActorID != "mem-1" || got[1].ActorID != "mem-2" || got[1].Emoji != "❤️" {
		t.Fatalf("reactions = %+v; the later emoji replaced the first", got)
	}
	if err := f.m.DMStore.ClearReaction(ctx, msg.ID, "mem-2"); err != nil {
		t.Fatalf("ClearReaction: %v", err)
	}
	got, _ = f.m.DMStore.ListReactions(ctx, th.ID)
	if len(got) != 1 || got[0].ActorID != "mem-1" {
		t.Fatalf("reactions after clearing = %+v", got)
	}
}

func TestCover13PublishingADeviceKeyRetiresTheHandsetsOlderKey(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	first, err := f.m.DMStore.PublishDeviceKey(ctx, models.DMDeviceKey{ActorID: "mem-1", KeyID: "k1", PublicKey: "pk1", DeviceID: "phone"})
	if err != nil || first.RetiredAt != nil {
		t.Fatalf("PublishDeviceKey = %+v, %v", first, err)
	}
	if _, err := f.m.DMStore.PublishDeviceKey(ctx, models.DMDeviceKey{ActorID: "mem-1", KeyID: "k2", PublicKey: "pk2", DeviceID: "phone"}); err != nil {
		t.Fatalf("PublishDeviceKey: %v", err)
	}
	if _, err := f.m.DMStore.PublishDeviceKey(ctx, models.DMDeviceKey{ActorID: "mem-1", KeyID: "k3", PublicKey: "pk3"}); err != nil {
		t.Fatalf("PublishDeviceKey: %v", err)
	}
	keys, err := f.m.DMStore.LookupDeviceKeys(ctx, []string{"mem-1", "mem-2"})
	if err != nil {
		t.Fatalf("LookupDeviceKeys: %v", err)
	}
	var ids []string
	for _, k := range keys {
		ids = append(ids, k.KeyID)
	}
	if len(ids) != 2 || ids[0] != "k2" || ids[1] != "k3" {
		t.Fatalf("live keys = %v; k1 was retired by the same handset's next key, a keyless-device key stays", ids)
	}
	none, err := f.m.DMStore.LookupDeviceKeys(ctx, nil)
	if err != nil || len(none) != 0 {
		t.Fatalf("lookup of nobody = %+v, %v", none, err)
	}
}

func TestCover13RetiringAHandsetsKeysCountsThemAndIgnoresNoDevice(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b"} {
		if _, err := f.m.DMStore.PublishDeviceKey(ctx, models.DMDeviceKey{ActorID: "mem-" + id, KeyID: "k-" + id, PublicKey: "pk", DeviceID: "tablet"}); err != nil {
			t.Fatalf("PublishDeviceKey: %v", err)
		}
	}
	if n, err := f.m.DMStore.RetireDeviceKeysForDevice(ctx, "", time.Time{}); err != nil || n != 0 {
		t.Fatalf("retiring no device = %d, %v", n, err)
	}
	n, err := f.m.DMStore.RetireDeviceKeysForDevice(ctx, "tablet", time.Time{})
	if err != nil || n != 2 {
		t.Fatalf("retired = %d, %v; want 2", n, err)
	}
	n, _ = f.m.DMStore.RetireDeviceKeysForDevice(ctx, "tablet", f.clock.now())
	if n != 0 {
		t.Fatalf("retiring again = %d; already retired keys are not counted", n)
	}
	keys, _ := f.m.DMStore.LookupDeviceKeys(ctx, []string{"mem-a", "mem-b"})
	if len(keys) != 0 {
		t.Fatalf("live keys after retiring the handset = %+v", keys)
	}
}
