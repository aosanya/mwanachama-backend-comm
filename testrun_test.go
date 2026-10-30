package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

type runWorld struct {
	live *mwanachamacomm.CommManager
	db   *gorm.DB
	spec *spec.Spec
	ctx  context.Context
}

func newRunWorld(t *testing.T) runWorld {
	t.Helper()
	db, s := newTestDB(t)
	m, err := mwanachamacomm.NewCommManager(db, s, monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), nopActWriter{}, "")
	if err != nil {
		t.Fatal(err)
	}
	m.UseBelonging(func(_ context.Context, actorID, structureID string) (bool, error) {
		return structureID == "g1" && (actorID == "coord" || actorID == "mem"), nil
	})
	m.UseLeading(func(_ context.Context, actorID, structureID string) (bool, error) {
		return structureID == "g1" && actorID == "coord", nil
	})
	m.UseLeads(func(context.Context, string) ([]string, error) { return []string{"coord"}, nil })
	return runWorld{live: m, db: db, spec: s, ctx: context.Background()}
}

func (w runWorld) run(t *testing.T, id string) *mwanachamacomm.CommManager {
	t.Helper()
	m, err := w.live.ForRun(id)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func (w runWorld) count(t *testing.T, table string) int64 {
	t.Helper()
	var n int64
	if err := w.db.Table(table).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func (w runWorld) liveRows(t *testing.T) int64 {
	t.Helper()
	var total int64
	for _, o := range w.spec.Objects {
		total += w.count(t, w.spec.TableFor(o))
	}
	return total
}

func (w runWorld) testRows(t *testing.T) int64 {
	t.Helper()
	var total int64
	for _, o := range w.spec.Objects {
		total += w.count(t, w.spec.TestTableFor(o))
	}
	return total
}

type playedScenario struct {
	thread       models.ChatThread
	requested    models.ChatThread
	message      models.ChatMessage
	notification models.Notification
	dm           models.DMThread
}

func playScenario(t *testing.T, w runWorld, m *mwanachamacomm.CommManager) playedScenario {
	t.Helper()
	var p playedScenario
	var err error
	if p.thread, err = m.StartChatThreadFor(w.ctx, "g1", "coord", "Budget"); err != nil {
		t.Fatal(err)
	}
	if p.message, err = m.PostChatMessageFor(w.ctx, "g1", p.thread.ID, "coord", "Budget is approved", "", ""); err != nil {
		t.Fatal(err)
	}
	if p.requested, err = m.StartChatThreadFor(w.ctx, "g1", "mem", "Help"); err != nil {
		t.Fatal(err)
	}
	if p.dm, err = m.DMStore.CreateThread(w.ctx, models.DMThread{CreatedBy: "coord"}, []string{"mem"}); err != nil {
		t.Fatal(err)
	}
	if p.notification, err = m.Raise(w.ctx, mwanachamacomm.Notification{
		ActorID: "mem", Category: mwanachamacomm.CategoryResults,
		Event: mwanachamacomm.EventResultsPublished, SubjectKind: mwanachamacomm.SubjectSurvey, SubjectID: "s-1",
	}); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAScenarioPlayedInARunLeavesNoRowInAnyLiveTable(t *testing.T) {
	w := newRunWorld(t)
	playScenario(t, w, w.run(t, "run-1"))

	if got := w.liveRows(t); got != 0 {
		t.Fatalf("live rows = %d after a run, want 0", got)
	}
	rows, err := specstore.RowsOfRun(w.ctx, w.db, w.spec, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	stored := map[string]int64{}
	for _, r := range rows {
		stored[r.Table] = r.Rows
	}
	for _, object := range []string{roleOf(t, w, "chat_thread"), roleOf(t, w, "chat_message"), roleOf(t, w, "dm_thread"), roleOf(t, w, "notification")} {
		if stored[w.spec.TestTableFor(objectOf(t, w, object))] == 0 {
			t.Fatalf("run-1 stored nothing for %s: %+v", object, rows)
		}
	}
}

func roleOf(t *testing.T, w runWorld, role string) string {
	t.Helper()
	o, ok := w.spec.ByRole(role)
	if !ok {
		t.Fatalf("no object fills %q", role)
	}
	return o.Name
}

func objectOf(t *testing.T, w runWorld, name string) spec.Object {
	t.Helper()
	for _, o := range w.spec.Objects {
		if o.Name == name {
			return o
		}
	}
	t.Fatalf("no object named %q", name)
	return spec.Object{}
}

func TestALiveScenarioLeavesTheTestCopyEmpty(t *testing.T) {
	w := newRunWorld(t)
	playScenario(t, w, w.live)

	if got := w.testRows(t); got != 0 {
		t.Fatalf("test copy holds %d rows after a live scenario, want 0", got)
	}
}

func TestARunSeesNothingTheLiveManagerWrote(t *testing.T) {
	w := newRunWorld(t)
	p := playScenario(t, w, w.live)
	run := w.run(t, "run-1")

	if threads, err := run.ChatStore.ListThreads(w.ctx, "g1"); err != nil || len(threads) != 0 {
		t.Fatalf("run sees %d live threads, %v; want none", len(threads), err)
	}
	if _, err := run.GetChatMessage(w.ctx, p.message.ID); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("run reading a live message = %v, want ErrChatNotFound", err)
	}
	if got, err := run.DMStore.ListThreadsFor(w.ctx, "coord"); err != nil || len(got) != 0 {
		t.Fatalf("run sees %d live direct threads, %v; want none", len(got), err)
	}
	if got, err := run.NotificationStore.List(w.ctx, "mem", 50); err != nil || len(got) != 0 {
		t.Fatalf("run sees %d live notifications, %v; want none", len(got), err)
	}
}

func TestTheLiveManagerSeesNothingARunWrote(t *testing.T) {
	w := newRunWorld(t)
	p := playScenario(t, w, w.run(t, "run-1"))

	if threads, err := w.live.ChatStore.ListThreads(w.ctx, "g1"); err != nil || len(threads) != 0 {
		t.Fatalf("live sees %d run threads, %v; want none", len(threads), err)
	}
	if _, err := w.live.GetChatMessage(w.ctx, p.message.ID); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("live reading a run's message = %v, want ErrChatNotFound", err)
	}
	if got, err := w.live.NotificationStore.List(w.ctx, "mem", 50); err != nil || len(got) != 0 {
		t.Fatalf("live sees %d run notifications, %v; want none", len(got), err)
	}
}

func TestTwoRunsKeepTheirScenariosApart(t *testing.T) {
	w := newRunWorld(t)
	first, second := w.run(t, "run-1"), w.run(t, "run-2")
	a := playScenario(t, w, first)
	playScenario(t, w, second)
	playScenario(t, w, second)

	firstNotes, err := first.NotificationStore.List(w.ctx, "mem", 50)
	if err != nil {
		t.Fatal(err)
	}
	secondNotes, err := second.NotificationStore.List(w.ctx, "mem", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstNotes) != 1 || len(secondNotes) != 2 {
		t.Fatalf("run-1 sees %d notifications and run-2 sees %d, want 1 and 2", len(firstNotes), len(secondNotes))
	}
	if _, err := second.GetChatMessage(w.ctx, a.message.ID); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("run-2 reading run-1's message = %v, want ErrChatNotFound", err)
	}
}

func TestADirectThreadListInARunHoldsOnlyThatRunsThreads(t *testing.T) {
	w := newRunWorld(t)
	first, second := w.run(t, "run-1"), w.run(t, "run-2")
	playScenario(t, w, first)
	playScenario(t, w, second)

	got, err := first.DMStore.ListThreadsFor(w.ctx, "coord")
	if err != nil || len(got) != 1 {
		t.Fatalf("run-1 lists %d direct threads for coord, %v; want 1", len(got), err)
	}
}

func TestRemovingARunRemovesItsRowsAndKeepsTheOthers(t *testing.T) {
	w := newRunWorld(t)
	first, second := w.run(t, "run-1"), w.run(t, "run-2")
	playScenario(t, w, first)
	playScenario(t, w, second)

	removed, err := specstore.RemoveRun(w.ctx, w.db, w.spec, "run-1")
	if err != nil || removed == 0 {
		t.Fatalf("removed %d rows, %v", removed, err)
	}
	if rows, _ := specstore.RowsOfRun(w.ctx, w.db, w.spec, "run-1"); len(rows) != 0 {
		t.Fatalf("run-1 still holds %+v", rows)
	}
	if threads, _ := second.ChatStore.ListThreads(w.ctx, "g1"); len(threads) != 2 {
		t.Fatalf("run-2 lost its threads: %d", len(threads))
	}
}

func TestARunScopedRaiseDoesNotCallTheOutboundSender(t *testing.T) {
	w := newRunWorld(t)
	sender := &recordingSender{}
	w.live.UseWebPush(sender, func(error) {})
	run := w.run(t, "run-1")
	subscribe(t, w.live, "mem", "https://push.example.org/live-mem")
	subscribe(t, run, "mem", "https://push.example.org/mem")

	n := raiseResults(t, run, "mem")
	playScenario(t, w, run)
	run.DrainPushes()
	w.live.DrainPushes()

	if got := sender.to(); len(got) != 0 {
		t.Fatalf("a run-scoped manager pushed to %v, want nothing to leave", got)
	}
	if got, err := run.PushDeliveriesOf(w.ctx, n.ID); err != nil || len(got) != 0 {
		t.Fatalf("run recorded %d push attempts, %v; want none", len(got), err)
	}
}

func TestAWebPushSenderGivenToARunManagerIsNeverUsed(t *testing.T) {
	w := newRunWorld(t)
	run := w.run(t, "run-1")
	sender := &recordingSender{}
	run.UseWebPush(sender, func(error) {})
	subscribe(t, run, "mem", "https://push.example.org/mem")

	raiseResults(t, run, "mem")
	run.DrainPushes()
	w.live.DrainPushes()

	if got := sender.to(); len(got) != 0 {
		t.Fatalf("a run-scoped manager pushed to %v, want nothing to leave", got)
	}
}

func TestTheParentStillPushesAfterARunWasTaken(t *testing.T) {
	w := newRunWorld(t)
	sender := &recordingSender{}
	w.live.UseWebPush(sender, func(error) {})
	w.run(t, "run-1")
	subscribe(t, w.live, "mem", "https://push.example.org/mem")

	raiseResults(t, w.live, "mem")
	w.live.DrainPushes()

	if got := sender.to(); len(got) != 1 {
		t.Fatalf("the live manager pushed to %v, want one endpoint", got)
	}
}

func TestAModerationActInARunIsNotWrittenToTheLiveActLog(t *testing.T) {
	w := newRunWorld(t)
	createTestActLog(t, w.db)
	live, err := mwanachamacomm.NewCommManager(w.db, w.spec, monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), txActWriter{}, "")
	if err != nil {
		t.Fatal(err)
	}
	run, err := live.ForRun("run-1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = run.CreateRemoval(w.ctx, models.Removal{
		MessageID: "msg-1", StructureID: "g1", RemovedBy: "coord", Reason: models.RemovalReasonAbuse,
	}, "wall", models.Actor{ID: "coord", StructureID: "g1"})
	if err != nil {
		t.Fatal(err)
	}

	if got := testActLogCount(t, w.db, "msg-1"); got != 0 {
		t.Fatalf("the live act log holds %d entries from a run, want 0", got)
	}
	if got := w.liveRows(t); got != 0 {
		t.Fatalf("live rows = %d, want 0", got)
	}
}

func TestARunWithoutAnIDIsRefused(t *testing.T) {
	w := newRunWorld(t)
	if _, err := w.live.ForRun(""); !errors.Is(err, specstore.ErrRunIDEmpty) {
		t.Fatalf("ForRun(\"\") = %v, want ErrRunIDEmpty", err)
	}
}

func TestChatCountsInARunCoverOnlyThatRun(t *testing.T) {
	w := newRunWorld(t)
	first, second := w.run(t, "run-1"), w.run(t, "run-2")
	a := playScenario(t, w, first)
	playScenario(t, w, second)
	playScenario(t, w, second)
	playScenario(t, w, w.live)

	page, err := first.ChatStore.Activity(w.ctx, models.ChatActivityQuery{})
	if err != nil || page.Threads != 2 || page.Messages != 1 {
		t.Fatalf("run-1 activity = %+v, %v; want 2 threads and 1 message", page, err)
	}
	page, err = second.ChatStore.Activity(w.ctx, models.ChatActivityQuery{})
	if err != nil || page.Threads != 2 || page.Messages != 2 {
		t.Fatalf("run-2 activity = %+v, %v; want 2 threads and 2 messages", page, err)
	}
	unread, err := first.ChatStore.UnreadCounts(w.ctx, "g1", "mem")
	if err != nil || len(unread) != 1 || unread[a.thread.ID] != 1 {
		t.Fatalf("run-1 unread = %v, %v; want only its own thread with 1", unread, err)
	}
	latest, err := first.ChatStore.LatestPosts(w.ctx, "g1")
	if err != nil || len(latest) != 1 {
		t.Fatalf("run-1 latest posts = %v, %v; want only its own posted thread", latest, err)
	}
	if _, there := latest[a.thread.ID]; !there {
		t.Fatalf("run-1 latest posts = %v, want run-1's own thread among them", latest)
	}
}

func TestAStandingMessageSentInARunStaysInThatRun(t *testing.T) {
	w := newRunWorld(t)
	run := w.run(t, "run-1")
	thread, err := run.StartChatThreadFor(w.ctx, "g1", "coord", "Meetings")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 5, 6, 0, 0, 0, time.UTC)
	if _, err := run.CreateStandingMessageFor(w.ctx, "g1", "coord", thread.ID, "Monthly meeting", "FREQ=MONTHLY", nil, "Africa/Nairobi", start.Format(time.RFC3339), ""); err != nil {
		t.Fatal(err)
	}

	sent, err := run.SendDueStandingMessages(w.ctx, start.Add(time.Hour))
	if err != nil || len(sent) != 1 {
		t.Fatalf("run sent %d standing messages, %v; want 1", len(sent), err)
	}
	if got := w.liveRows(t); got != 0 {
		t.Fatalf("live rows = %d after a standing send in a run, want 0", got)
	}
	again, err := run.SendDueStandingMessages(w.ctx, start.Add(2*time.Hour))
	if err != nil || len(again) != 0 {
		t.Fatalf("the occurrence went out twice: %v, %+v", err, again)
	}
}

func TestPruningPushAttemptsInARunLeavesTheOtherRunsAlone(t *testing.T) {
	w := newRunWorld(t)
	first, second := w.run(t, "run-1"), w.run(t, "run-2")
	for _, m := range []*mwanachamacomm.CommManager{first, second} {
		if _, err := m.RecordPushDelivery(w.ctx, models.PushDelivery{
			NotificationID: "n-1", ActorID: "mem", SubscriptionID: "sub-1", State: models.PushDeliverySent,
		}); err != nil {
			t.Fatal(err)
		}
	}

	pruned, err := first.PushStore.PrunePushDeliveries(w.ctx, time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || pruned != 1 {
		t.Fatalf("pruned %d, %v; want only run-1's attempt", pruned, err)
	}
	if got, err := second.PushDeliveriesOf(w.ctx, "n-1"); err != nil || len(got) != 1 {
		t.Fatalf("run-2 holds %d attempts, %v; want 1", len(got), err)
	}
}
