package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func newModerationStore(t *testing.T, actWriter models.ActWriter) (*mwanachamacomm.ModerationStore, *gorm.DB) {
	t.Helper()
	db, tables := newTestDB(t)
	createTestActLog(t, db)
	s, err := mwanachamacomm.NewModerationStore(db, tables, monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), actWriter)
	if err != nil {
		t.Fatalf("NewModerationStore: %v", err)
	}
	return s, db
}

func TestModerationFileReportConflict(t *testing.T) {
	s, _ := newModerationStore(t, txActWriter{})
	ctx := context.Background()
	r := models.Report{MessageID: "msg-1", ChapterID: "ward-1", ReportedBy: "m-1", Reason: models.ReportReasonAbuse, Excerpt: "..."}
	if _, err := s.FileReport(ctx, r); err != nil {
		t.Fatalf("first report: %v", err)
	}
	if _, err := s.FileReport(ctx, r); !errors.Is(err, mwanachamacomm.ErrConflict) {
		t.Fatalf("duplicate (message, reporter) report: got %v, want ErrConflict", err)
	}
}

func TestModerationCreateRemovalWritesActLogInSameTransaction(t *testing.T) {
	s, db := newModerationStore(t, txActWriter{})
	ctx := context.Background()

	rem, err := s.CreateRemoval(ctx, models.Removal{
		MessageID: "msg-1", ChapterID: "ward-1", RemovedBy: "mod-1",
		ActorRoleClass: "Ward coordinator", Reason: models.RemovalReasonAbuse,
	}, "Ward wall", models.Actor{ID: "mod-1", ChapterID: "ward-1"})
	if err != nil {
		t.Fatalf("CreateRemoval: %v", err)
	}
	if got := testActLogCount(t, db, rem.MessageID); got != 1 {
		t.Fatalf("act log rows for %q = %d, want 1", rem.MessageID, got)
	}

	// A second removal of an already-withheld post is refused before the
	// log is touched a second time.
	if _, err := s.CreateRemoval(ctx, models.Removal{
		MessageID: "msg-1", ChapterID: "ward-1", RemovedBy: "mod-1",
		ActorRoleClass: "Ward coordinator", Reason: models.RemovalReasonAbuse,
	}, "Ward wall", models.Actor{ID: "mod-1", ChapterID: "ward-1"}); !errors.Is(err, mwanachamacomm.ErrConflict) {
		t.Fatalf("second removal of the same message: got %v, want ErrConflict", err)
	}
	if got := testActLogCount(t, db, rem.MessageID); got != 1 {
		t.Fatalf("act log rows after the refused repeat = %d, want still 1", got)
	}
}

func TestModerationCreateRemovalRollsBackWhenActWriterFails(t *testing.T) {
	s, db := newModerationStore(t, txActWriter{fail: true})
	ctx := context.Background()

	if _, err := s.CreateRemoval(ctx, models.Removal{
		MessageID: "msg-2", ChapterID: "ward-1", RemovedBy: "mod-1",
		ActorRoleClass: "Ward coordinator", Reason: models.RemovalReasonAbuse,
	}, "Ward wall", models.Actor{ID: "mod-1", ChapterID: "ward-1"}); err == nil {
		t.Fatal("expected CreateRemoval to fail when the act log write fails")
	}
	if _, err := s.GetRemovalForMessage(ctx, "msg-2"); !errors.Is(err, models.ErrModerationNotFound) {
		t.Fatalf("expected the removal to have rolled back with the failed act write, got %v", err)
	}
	if got := testActLogCount(t, db, "msg-2"); got != 0 {
		t.Fatalf("act log rows after a failed write = %d, want 0", got)
	}
}

func TestModerationDismissReportsRefusesAnAlreadyWithheldMessage(t *testing.T) {
	s, _ := newModerationStore(t, txActWriter{})
	ctx := context.Background()
	if _, err := s.CreateRemoval(ctx, models.Removal{
		MessageID: "msg-1", ChapterID: "ward-1", RemovedBy: "mod-1", Reason: models.RemovalReasonAbuse,
	}, "wall", models.Actor{ID: "mod-1"}); err != nil {
		t.Fatalf("CreateRemoval: %v", err)
	}
	_, err := s.DismissReports(ctx, models.Dismissal{
		MessageID: "msg-1", ChapterID: "ward-1", DismissedBy: "mod-2", Reason: models.RemovalReasonAbuse,
	}, "wall", models.Actor{ID: "mod-2"})
	if !errors.Is(err, models.ErrAlreadyRemoved) {
		t.Fatalf("dismissing reports on a withheld message: got %v, want ErrAlreadyRemoved", err)
	}
}

func TestModerationDismissReportsIsOnePerMessage(t *testing.T) {
	s, db := newModerationStore(t, txActWriter{})
	ctx := context.Background()
	d := models.Dismissal{MessageID: "msg-1", ChapterID: "ward-1", DismissedBy: "mod-1", Reason: models.RemovalReasonAbuse}
	out, err := s.DismissReports(ctx, d, "wall", models.Actor{ID: "mod-1"})
	if err != nil {
		t.Fatalf("first dismiss: %v", err)
	}
	if got := testActLogCount(t, db, out.MessageID); got != 1 {
		t.Fatalf("act log rows = %d, want 1", got)
	}
	if _, err := s.DismissReports(ctx, d, "wall", models.Actor{ID: "mod-1"}); !errors.Is(err, mwanachamacomm.ErrConflict) {
		t.Fatalf("second dismiss of the same message: got %v, want ErrConflict", err)
	}
}

func TestModerationDecideDisputeEnforcesG62(t *testing.T) {
	s, _ := newModerationStore(t, txActWriter{})
	ctx := context.Background()
	rem, err := s.CreateRemoval(ctx, models.Removal{
		MessageID: "msg-1", ChapterID: "ward-1", RemovedBy: "mod-1", Reason: models.RemovalReasonAbuse,
	}, "wall", models.Actor{ID: "mod-1"})
	if err != nil {
		t.Fatalf("CreateRemoval: %v", err)
	}
	d, err := s.CreateDispute(ctx, models.Dispute{RemovalID: rem.ID, RaisedBy: "author-1", Statement: "wasn't abuse", ReviewChapterID: "region-1"})
	if err != nil {
		t.Fatalf("CreateDispute: %v", err)
	}
	// G62: the reviewer of a disputed removal may never be the remover.
	if _, err := s.DecideDispute(ctx, d.ID, models.DisputeReinstated, "mod-1", time.Now(), "wall", models.Actor{ID: "mod-1"}); !errors.Is(err, models.ErrReviewerIsRemover) {
		t.Fatalf("decide by the remover: got %v, want ErrReviewerIsRemover", err)
	}
	out, err := s.DecideDispute(ctx, d.ID, models.DisputeReinstated, "reviewer-1", time.Now(), "wall", models.Actor{ID: "reviewer-1"})
	if err != nil {
		t.Fatalf("decide by a different reviewer: %v", err)
	}
	if out.State != models.DisputeReinstated {
		t.Fatalf("State = %q, want reinstated", out.State)
	}
	// Single-shot: a decided dispute cannot be decided again.
	if _, err := s.DecideDispute(ctx, d.ID, models.DisputeUpheld, "reviewer-2", time.Now(), "wall", models.Actor{ID: "reviewer-2"}); !errors.Is(err, models.ErrAlreadyDecided) {
		t.Fatalf("re-deciding: got %v, want ErrAlreadyDecided", err)
	}
}

func TestModerationCreateDisputeRequiresAnExistingRemoval(t *testing.T) {
	s, _ := newModerationStore(t, txActWriter{})
	ctx := context.Background()
	if _, err := s.CreateDispute(ctx, models.Dispute{RemovalID: "no-such-removal", RaisedBy: "a", Statement: "x", ReviewChapterID: "r"}); !errors.Is(err, mwanachamacomm.ErrInvalidReference) {
		t.Fatalf("dispute against a non-existent removal: got %v, want ErrInvalidReference", err)
	}
}
