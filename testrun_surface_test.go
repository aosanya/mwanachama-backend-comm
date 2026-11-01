package mwanachamacomm_test

import (
	"context"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/specstore"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func playWholeSurface(t *testing.T, w runWorld, m *mwanachamacomm.CommManager) {
	ctx := w.ctx
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	thread, err := m.StartChatThreadFor(ctx, "g1", "coord", "Budget")
	must(err)
	_, err = m.OpenChatThreadFor(ctx, "g1", thread.ID, "coord")
	must(err)
	msg, err := m.PostChatMessageFor(ctx, "g1", thread.ID, "coord", "Budget is approved", "", "")
	must(err)
	_, err = m.MarkChatThreadReadFor(ctx, "g1", thread.ID, "mem", msg.ID)
	must(err)
	_, err = m.ArchiveChatThreadFor(ctx, "g1", thread.ID, "mem")
	must(err)
	must(m.UnarchiveChatThreadFor(ctx, "g1", thread.ID, "mem"))
	requested, err := m.StartChatThreadFor(ctx, "g1", "mem", "Help")
	must(err)
	_, err = m.DeclineChatThreadFor(ctx, "g1", requested.ID, "coord")
	must(err)

	start := time.Date(2026, 1, 5, 6, 0, 0, 0, time.UTC)
	standing, err := m.CreateStandingMessageFor(ctx, "g1", "coord", thread.ID, "Monthly meeting", "FREQ=MONTHLY", nil, "Africa/Nairobi", start.Format(time.RFC3339), "")
	must(err)
	_, err = m.SendDueStandingMessages(ctx, start.Add(time.Hour))
	must(err)
	_, err = m.StopStandingMessageFor(ctx, "g1", standing.ID, "coord")
	must(err)

	dm, err := m.DMStore.CreateThread(ctx, models.DMThread{CreatedBy: "coord"}, []string{"mem"})
	must(err)
	_, err = m.DMStore.Accept(ctx, dm.ID, "mem")
	must(err)
	dmMsg, err := m.PostDMMessage(ctx, models.DMMessage{ThreadID: dm.ID, SenderID: "coord", PayloadCiphertext: "hi"})
	must(err)
	must(m.SetDMReactionFor(ctx, dmMsg.ID, "mem", "+1"))
	must(m.ClearDMReactionFor(ctx, dmMsg.ID, "mem"))
	must(m.SetDMReactionFor(ctx, dmMsg.ID, "coord", "ok"))
	_, err = m.PublishDMDeviceKeyFor(ctx, models.DMDeviceKey{ActorID: "coord", KeyID: "k1", PublicKey: "pk"}, "coord", "dev-1")
	must(err)
	_, err = m.DMStore.Invite(ctx, dm.ID, "third", "coord")
	must(err)
	_, err = m.DMStore.Promote(ctx, dm.ID, "mem", "coord")
	must(err)
	must(m.DMStore.Kick(ctx, dm.ID, "third", "coord"))
	_, err = m.ReEnableFor(ctx, dm.ID, "third", "coord")
	must(err)
	must(m.DMStore.Leave(ctx, dm.ID, "mem"))

	_, err = m.ModerationStore.FileReport(ctx, models.Report{MessageID: msg.ID, StructureID: "g1", ReportedBy: "mem", Reason: models.ReportReasonAbuse, Excerpt: "..."})
	must(err)
	removal, err := m.ModerationStore.CreateRemoval(ctx, models.Removal{MessageID: msg.ID, StructureID: "g1", RemovedBy: "coord", Reason: models.RemovalReasonAbuse}, "wall", models.Actor{ID: "coord", StructureID: "g1"})
	must(err)
	dispute, err := m.ModerationStore.CreateDispute(ctx, models.Dispute{RemovalID: removal.ID, RaisedBy: "mem", Statement: "no", ReviewStructureID: "region-1"})
	must(err)
	_, err = m.ModerationStore.DecideDispute(ctx, dispute.ID, models.DisputeReinstated, "reviewer", time.Now(), "wall", models.Actor{ID: "reviewer"})
	must(err)
	_, err = m.ModerationStore.DismissReports(ctx, models.Dismissal{MessageID: "msg-other", StructureID: "g1", DismissedBy: "coord", Reason: models.RemovalReasonAbuse}, "wall", models.Actor{ID: "coord"})
	must(err)

	_, err = m.AddressStore.Publish(ctx, models.Address{ActorID: "mem", Hash: hashOf("a1"), SaltID: 1, AddressIndex: 0})
	must(err)
	must(m.AddressStore.Block(ctx, models.AddressBlock{ActorID: "mem", Hash: hashOf("blocked")}))
	_, err = m.AddressStore.UpdateSettings(ctx, "mem", 0, models.AddressSettings{PublicAddress: "MKU4827YUT3391"})
	must(err)
	must(m.RetireMyAddress(ctx, "mem", 0))

	n, err := m.Raise(ctx, mwanachamacomm.Notification{
		ActorID: "mem", Category: mwanachamacomm.CategoryResults,
		Event: mwanachamacomm.EventResultsPublished, SubjectKind: mwanachamacomm.SubjectSurvey, SubjectID: "s-1",
	})
	must(err)
	_, err = m.SetNotificationPreferenceFor(ctx, "mem", models.CategoryItem, true)
	must(err)
	_, err = m.MarkMyNotificationsRead(ctx, "mem", []string{n.ID}, "")
	must(err)
	_, err = m.AcknowledgeNotification(ctx, "mem", n.ID)
	must(err)
	m.UseAudience(fixedAudience("mem", "coord"))
	_, err = m.RaiseToAudience(ctx, audienceTemplate())
	must(err)

	_, err = m.SubscribePushFor(ctx, "mem", "dev-1", "https://push.example.org/mem", newThrowawayBrowser(t).keys())
	must(err)
	_, err = m.RecordPushDelivery(ctx, models.PushDelivery{NotificationID: n.ID, ActorID: "mem", SubscriptionID: "sub-1", State: models.PushDeliverySent})
	must(err)
	must(m.UnsubscribePushFor(ctx, "mem", "https://push.example.org/mem"))

	m.UseStanding(func(_ context.Context, actorID, kind, id string) (models.SubjectStanding, error) {
		if kind != "poster" || id != "p1" {
			return models.StandingNone, nil
		}
		if actorID == "coord" {
			return models.StandingAnswerer, nil
		}
		return models.StandingAudience, nil
	})
	m.UseAnswerers(func(context.Context, string, string) ([]string, error) { return []string{"coord"}, nil })
	opened, err := m.OpenSubjectThreadFor(ctx, "poster", "p1", "mem", models.AudienceGroup)
	must(err)
	_, err = m.PostSubjectMessageFor(ctx, opened.ID, "mem", "Is it still open", "")
	must(err)
	_, err = m.PostSubjectMessageFor(ctx, opened.ID, "coord", "Yes", "")
	must(err)
}

func TestTheWholeCommSurfacePlayedInARunLeavesNoRowInAnyLiveTable(t *testing.T) {
	w := newRunWorld(t)
	playWholeSurface(t, w, w.run(t, "run-1"))

	if got := w.liveRows(t); got != 0 {
		t.Fatalf("live rows = %d after a run played the whole surface, want 0", got)
	}
	rows, err := specstore.RowsOfRun(w.ctx, w.db, w.spec, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	stored := map[string]bool{}
	for _, r := range rows {
		stored[r.Object] = true
	}
	released := map[string]bool{"chat_archive": true, "push_subscription": true}
	for _, o := range w.spec.Objects {
		if !stored[o.Name] && !released[o.Name] {
			t.Fatalf("the run stored nothing for %s", o.Name)
		}
	}
}

func TestARunsDeletionsLeaveEveryLiveRowInPlace(t *testing.T) {
	w := newRunWorld(t)
	ctx := w.ctx
	thread, err := w.live.StartChatThreadFor(ctx, "g1", "coord", "Budget")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.live.OpenChatThreadFor(ctx, "g1", thread.ID, "coord"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.live.ArchiveChatThreadFor(ctx, "g1", thread.ID, "mem"); err != nil {
		t.Fatal(err)
	}
	endpoint := "https://push.example.org/mem"
	subscribe(t, w.live, "mem", endpoint)
	dm, err := w.live.DMStore.CreateThread(ctx, models.DMThread{CreatedBy: "coord"}, []string{"mem"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.live.DMStore.Accept(ctx, dm.ID, "mem"); err != nil {
		t.Fatal(err)
	}
	dmMsg, err := w.live.PostDMMessage(ctx, models.DMMessage{ThreadID: dm.ID, SenderID: "coord", PayloadCiphertext: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.live.SetDMReactionFor(ctx, dmMsg.ID, "mem", "+1"); err != nil {
		t.Fatal(err)
	}
	before := w.liveRows(t)
	if before == 0 {
		t.Fatal("the live scenario wrote nothing")
	}

	run := w.run(t, "run-1")
	_ = run.UnarchiveChatThreadFor(ctx, "g1", thread.ID, "mem")
	_ = run.UnsubscribePushFor(ctx, "mem", endpoint)
	_ = run.ClearDMReactionFor(ctx, dmMsg.ID, "mem")
	_ = run.DMStore.Leave(ctx, dm.ID, "mem")
	_ = run.DMStore.Kick(ctx, dm.ID, "mem", "coord")
	_ = run.DMStore.ClearReaction(ctx, dmMsg.ID, "mem")
	_ = run.PushStore.DropPushSubscription(ctx, "whatever")

	if after := w.liveRows(t); after != before {
		t.Fatalf("live rows went from %d to %d after a run deleted", before, after)
	}
	if got, err := w.live.ChatStore.ArchivedThreads(ctx, "g1", "mem"); err != nil || !got[thread.ID] {
		t.Fatalf("the live archive row was removed by a run: %v, %v", got, err)
	}
	if got, err := w.live.PushStore.ListPushSubscriptions(ctx, "mem"); err != nil || len(got) != 1 {
		t.Fatalf("the live subscription was removed by a run: %d, %v", len(got), err)
	}
	if got, err := w.live.DMStore.ListReactions(ctx, dm.ID); err != nil || len(got) != 1 {
		t.Fatalf("the live reaction was removed by a run: %d, %v", len(got), err)
	}
}

func TestTwoRunsPlayingTheWholeSurfaceKeepEveryRowApartAndRemovalTakesOnlyOne(t *testing.T) {
	w := newRunWorld(t)
	playWholeSurface(t, w, w.run(t, "run-1"))
	playWholeSurface(t, w, w.run(t, "run-2"))

	first, err := specstore.RowsOfRun(w.ctx, w.db, w.spec, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := specstore.RowsOfRun(w.ctx, w.db, w.spec, "run-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("run-1 holds %+v and run-2 holds %+v, want the same tables with the same counts", first, second)
	}
	for i := range first {
		if first[i].Table != second[i].Table || first[i].Rows != second[i].Rows {
			t.Fatalf("run-1 holds %+v and run-2 holds %+v, want the same tables with the same counts", first[i], second[i])
		}
	}
	if got := w.liveRows(t); got != 0 {
		t.Fatalf("live rows = %d, want 0", got)
	}

	removed, err := specstore.RemoveRun(w.ctx, w.db, w.spec, "run-1")
	if err != nil || removed == 0 {
		t.Fatalf("removed %d rows, %v", removed, err)
	}
	if rows, _ := specstore.RowsOfRun(w.ctx, w.db, w.spec, "run-1"); len(rows) != 0 {
		t.Fatalf("run-1 still holds %+v", rows)
	}
	left, err := specstore.RowsOfRun(w.ctx, w.db, w.spec, "run-2")
	if err != nil || len(left) != len(second) {
		t.Fatalf("run-2 holds %+v after run-1 was removed, want %+v (%v)", left, second, err)
	}
	for i := range left {
		if left[i].Rows != second[i].Rows {
			t.Fatalf("run-2 holds %+v after run-1 was removed, want %+v", left, second)
		}
	}
}

func TestOneRunUnsubscribingLeavesTheOtherRunsAndTheLiveSubscriptionInPlace(t *testing.T) {
	w := newRunWorld(t)
	first, second := w.run(t, "run-1"), w.run(t, "run-2")
	endpoint := "https://push.example.org/mem"
	subscribe(t, w.live, "mem", endpoint)
	subscribe(t, first, "mem", endpoint)
	subscribe(t, second, "mem", endpoint)

	if err := first.UnsubscribePushFor(w.ctx, "mem", endpoint); err != nil {
		t.Fatal(err)
	}

	if got, err := first.PushStore.ListPushSubscriptions(w.ctx, "mem"); err != nil || len(got) != 0 {
		t.Fatalf("run-1 still holds %d subscriptions, %v", len(got), err)
	}
	if got, err := second.PushStore.ListPushSubscriptions(w.ctx, "mem"); err != nil || len(got) != 1 {
		t.Fatalf("run-2 holds %d subscriptions after run-1 unsubscribed, %v; want 1", len(got), err)
	}
	if got, err := w.live.PushStore.ListPushSubscriptions(w.ctx, "mem"); err != nil || len(got) != 1 {
		t.Fatalf("the live manager holds %d subscriptions after run-1 unsubscribed, %v; want 1", len(got), err)
	}
}
