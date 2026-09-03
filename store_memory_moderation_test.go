package mwanachamacomm

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeActWriter is a real, functioning MemoryActWriter for tests — not a nil
// or a no-op stub. A store built without a genuine log would pass every
// assertion below while withholding posts silently (the reason
// ModerationMemoryStore's actWriter argument is required, not optional).
type fakeActWriter struct {
	mu      sync.Mutex
	entries []ActEntry
}

func newFakeActWriter() *fakeActWriter { return &fakeActWriter{} }

func (w *fakeActWriter) WriteAct(_ context.Context, e ActEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = append(w.entries, e)
	return nil
}

// testWall and testActor are what DEV-1341's act-log rows are composed from.
var (
	testWall  = "Shinyalu Ward wall"
	testActor = Actor{ID: "mod-1", Label: "M. Shikuku", ChapterID: "chapter-1"}
)

func newModerationStore() *ModerationMemoryStore {
	return NewModerationMemoryStore(NewIDGen(), SystemClock, newFakeActWriter())
}

// TestReportThenRemovalThenDisputeFlow walks the whole shape DEV-1115 names:
// report -> removal (withheld, not deleted) -> dispute raised one chapter up
// -> decide, and checks the room and the ancestor see different things about
// the same removal at every step.
func TestReportThenRemovalThenDisputeFlow(t *testing.T) {
	ctx := context.Background()
	s := newModerationStore()

	// 1. A member reports a message. The report is a request for review, not
	// a removal — it changes nothing about the message itself.
	rep, err := s.FileReport(ctx, Report{
		MessageID: "msg-1", ChapterID: "ward-1",
		ReportedBy: "member-reporter", ReporterRoleClass: "Member",
		Reason: ReportReasonAbuse, Excerpt: "the reported text",
	})
	if err != nil {
		t.Fatalf("FileReport: %v", err)
	}
	if rep.ID == "" || rep.ReportedAt.IsZero() {
		t.Fatalf("FileReport did not stamp id/reported_at: %+v", rep)
	}

	// Filing a second, distinct report on the same message must succeed and
	// both must show up in the queue and the message's own report list —
	// "repeat reports... aggregate", not overwrite.
	if _, err := s.FileReport(ctx, Report{
		MessageID: "msg-1", ChapterID: "ward-1",
		ReportedBy: "member-second", ReporterRoleClass: "Ward coordinator",
		Reason: ReportReasonThreats, Excerpt: "the reported text",
	}); err != nil {
		t.Fatalf("second FileReport: %v", err)
	}
	byMessage, err := s.ListReportsForMessage(ctx, "msg-1")
	if err != nil || len(byMessage) != 2 {
		t.Fatalf("ListReportsForMessage = %+v, err %v, want 2 reports", byMessage, err)
	}
	queue, err := s.ListReportQueue(ctx, "ward-1")
	if err != nil || len(queue) != 2 {
		t.Fatalf("ListReportQueue = %+v, err %v, want 2 reports", queue, err)
	}

	// The same member reporting the same message twice is a conflict, not a
	// second row (message-report.md's unique(message_id, reported_by)).
	if _, err := s.FileReport(ctx, Report{
		MessageID: "msg-1", ChapterID: "ward-1",
		ReportedBy: "member-reporter", ReporterRoleClass: "Member",
		Reason: ReportReasonSpam, Excerpt: "the reported text",
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("re-reporting the same message: got %v, want ErrConflict", err)
	}

	// 2. A moderator removes the message. Nothing about the report row
	// changes or is deleted — the removal is a second, independent row.
	rem, err := s.CreateRemoval(ctx, Removal{
		MessageID: "msg-1", ChapterID: "ward-1",
		RemovedBy: "member-mod", ActorRoleClass: "Ward coordinator",
		Reason: RemovalReasonAbuse,
	}, testWall, testActor)
	if err != nil {
		t.Fatalf("CreateRemoval: %v", err)
	}
	if rem.ID == "" || rem.RemovedAt.IsZero() {
		t.Fatalf("CreateRemoval did not stamp id/removed_at: %+v", rem)
	}
	// The report is still exactly there, untouched — "not a delete" holds for
	// the report as much as for the message.
	if again, err := s.ListReportsForMessage(ctx, "msg-1"); err != nil || len(again) != 2 {
		t.Fatalf("reports after removal = %+v, err %v, want the same 2 reports still present", again, err)
	}

	// A second removal of the same message is a conflict — "one removal per
	// message".
	if _, err := s.CreateRemoval(ctx, Removal{
		MessageID: "msg-1", ChapterID: "ward-1",
		RemovedBy: "member-mod-2", ActorRoleClass: "Ward organizer",
		Reason: RemovalReasonThreats,
	}, testWall, testActor); !errors.Is(err, ErrConflict) {
		t.Fatalf("double removal: got %v, want ErrConflict", err)
	}

	// The room and the ancestor are told differently about the SAME row: the
	// room-safe projection never carries RemovedBy, the full row does.
	fromStore, err := s.GetRemovalForMessage(ctx, "msg-1")
	if err != nil {
		t.Fatalf("GetRemovalForMessage: %v", err)
	}
	roomView := fromStore.Room()
	if roomView.RemovedBy != "" {
		t.Fatalf("room view leaks RemovedBy: %+v", roomView)
	}
	if roomView.ActorRoleClass != "Ward coordinator" || roomView.Reason != RemovalReasonAbuse {
		t.Fatalf("room view lost its own fields: %+v", roomView)
	}
	if fromStore.RemovedBy != "member-mod" {
		t.Fatalf("ancestor/full view lost RemovedBy: %+v", fromStore)
	}

	// 3. The author disputes the removal, one chapter up.
	dis, err := s.CreateDispute(ctx, Dispute{
		RemovalID: rem.ID, RaisedBy: "member-author",
		Statement:       "I was quoting someone else, not saying it.",
		ReviewChapterID: "constituency-1",
	})
	if err != nil {
		t.Fatalf("CreateDispute: %v", err)
	}
	if dis.State != DisputeOpen {
		t.Fatalf("a freshly raised dispute must be Open, got %q", dis.State)
	}
	if dis.ReviewChapterID != "constituency-1" {
		t.Fatalf("dispute did not land one chapter up: %+v", dis)
	}
	if dis.HeldSince.IsZero() || dis.RaisedAt.IsZero() {
		t.Fatalf("CreateDispute did not stamp held_since/raised_at: %+v", dis)
	}

	// A second dispute on the same removal is a conflict — "one dispute per
	// removal".
	if _, err := s.CreateDispute(ctx, Dispute{
		RemovalID: rem.ID, RaisedBy: "member-author", Statement: "again",
		ReviewChapterID: "constituency-1",
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("double dispute: got %v, want ErrConflict", err)
	}

	// G62: the reviewer of a disputed removal is never the actor who removed
	// it, refused at decision time.
	if _, err := s.DecideDispute(ctx, dis.ID, DisputeReinstated, "member-mod", time.Now(), testWall, testActor); !errors.Is(err, ErrReviewerIsRemover) {
		t.Fatalf("decide by the remover: got %v, want ErrReviewerIsRemover", err)
	}

	// A genuine reviewer decides it.
	decided, err := s.DecideDispute(ctx, dis.ID, DisputeReinstated, "member-reviewer", time.Now(), testWall, testActor)
	if err != nil {
		t.Fatalf("DecideDispute: %v", err)
	}
	if decided.State != DisputeReinstated {
		t.Fatalf("decided state = %q, want reinstated", decided.State)
	}
	if decided.DecidedBy != "member-reviewer" || decided.DecidedAt == nil {
		t.Fatalf("DecideDispute did not stamp decided_by/decided_at: %+v", decided)
	}

	// Deciding an already-decided dispute is refused, not silently repeated —
	// "the outcome is recorded once".
	if _, err := s.DecideDispute(ctx, dis.ID, DisputeUpheld, "member-reviewer", time.Now(), testWall, testActor); !errors.Is(err, ErrAlreadyDecided) {
		t.Fatalf("re-deciding: got %v, want ErrAlreadyDecided", err)
	}

	// The removal itself is still exactly where it was — "reinstated" is
	// modelled as the dispute's own state, never as a delete of the removal
	// row; the ancestor register keeps showing who removed it.
	stillThere, err := s.GetRemoval(ctx, rem.ID)
	if err != nil || stillThere.RemovedBy != "member-mod" {
		t.Fatalf("removal after reinstatement = %+v, err %v — the row must survive being overturned", stillThere, err)
	}

	// GetDisputeForRemoval is the "Disputed" flag on the upward register.
	flag, err := s.GetDisputeForRemoval(ctx, rem.ID)
	if err != nil || flag.ID != dis.ID {
		t.Fatalf("GetDisputeForRemoval = %+v, err %v, want %s", flag, err, dis.ID)
	}
}

// TestGetRemovalForMessageNotFoundIsARealState covers the common case: most
// messages have never been removed, and that must read back as
// ErrModerationNotFound, not a zero-valued Removal a caller could mistake for
// "removed with no reason".
func TestGetRemovalForMessageNotFoundIsARealState(t *testing.T) {
	s := newModerationStore()
	if _, err := s.GetRemovalForMessage(context.Background(), "msg-never-removed"); !errors.Is(err, ErrModerationNotFound) {
		t.Fatalf("got %v, want ErrModerationNotFound", err)
	}
}

// TestCreateDisputeRefusesAnUnknownRemoval covers the referential guard: a
// dispute naming a removal that does not exist is the caller's mistake, not
// server data corruption.
func TestCreateDisputeRefusesAnUnknownRemoval(t *testing.T) {
	s := newModerationStore()
	_, err := s.CreateDispute(context.Background(), Dispute{
		RemovalID: "removal-does-not-exist", RaisedBy: "member-author",
		Statement: "appeal", ReviewChapterID: "constituency-1",
	})
	if !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("got %v, want ErrInvalidReference", err)
	}
}

var _ ModerationRepository = (*ModerationMemoryStore)(nil)
