package mwanachamacomm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (f *cover13Fixture) removal(t *testing.T, messageID, by string) models.Removal {
	t.Helper()
	rem, err := f.m.ModerationStore.CreateRemoval(context.Background(), models.Removal{
		MessageID: messageID, StructureID: "s-1", RemovedBy: by, ActorRoleClass: "steward", Reason: models.RemovalReasonAbuse,
	}, "", models.Actor{ID: by, Label: "Steward"})
	if err != nil {
		t.Fatalf("CreateRemoval: %v", err)
	}
	return rem
}

func TestCover13ReportsAreQueuedNewestFirstPerStructureAndPerMessage(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	file := func(structure, message, by string, reason models.ReportReason) models.Report {
		r, err := f.m.ModerationStore.FileReport(ctx, models.Report{
			MessageID: message, StructureID: structure, ReportedBy: by, ReporterRoleClass: "member", Reason: reason, Excerpt: "text",
		})
		if err != nil {
			t.Fatalf("FileReport: %v", err)
		}
		return r
	}
	first := file("s-1", "msg-1", "mem-1", models.ReportReasonSpam)
	second := file("s-1", "msg-1", "mem-2", models.ReportReasonAbuse)
	file("s-2", "msg-9", "mem-3", models.ReportReasonThreats)
	if first.ID == "" || first.ReportedAt.IsZero() {
		t.Fatalf("report = %+v; an id and a time are filled", first)
	}
	queue, err := f.m.ModerationStore.ListReportQueue(ctx, "s-1")
	if err != nil || len(queue) != 2 || queue[0].ID != second.ID || queue[1].ID != first.ID {
		t.Fatalf("queue = %+v, %v; want the newest report first and only this structure's", queue, err)
	}
	forMessage, err := f.m.ModerationStore.ListReportsForMessage(ctx, "msg-1")
	if err != nil || len(forMessage) != 2 {
		t.Fatalf("reports for msg-1 = %+v, %v", forMessage, err)
	}
	none, err := f.m.ModerationStore.ListReportsForMessage(ctx, "msg-none")
	if err != nil || len(none) != 0 {
		t.Fatalf("reports for an unreported message = %+v, %v", none, err)
	}
}

func TestCover13ARemovalWritesItsWithheldActInTheSameTransaction(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	rem := f.removal(t, "msg-1", "steward-1")
	if rem.ID == "" || rem.RemovedAt.IsZero() {
		t.Fatalf("removal = %+v", rem)
	}
	acts := f.acts.written()
	if len(acts) != 1 || acts[0].Kind != models.ActPostWithheld || acts[0].SubjectID != "msg-1" || acts[0].ActorID != "steward-1" {
		t.Fatalf("acts = %+v; one post_withheld on the message by the steward", acts)
	}
	if acts[0].StructureID != "s-1" || acts[0].SubjectRef != "s-1" || acts[0].ActorStructureID != "s-1" {
		t.Fatalf("with no wall and no actor structure the act sits on the removal's structure: %+v", acts[0])
	}
	if acts[0].Detail["reason"] != string(models.RemovalReasonAbuse) {
		t.Fatalf("act detail = %+v", acts[0].Detail)
	}
	got, err := f.m.ModerationStore.GetRemoval(ctx, rem.ID)
	if err != nil || got.MessageID != "msg-1" {
		t.Fatalf("GetRemoval = %+v, %v", got, err)
	}
	byMessage, err := f.m.ModerationStore.GetRemovalForMessage(ctx, "msg-1")
	if err != nil || byMessage.ID != rem.ID {
		t.Fatalf("GetRemovalForMessage = %+v, %v", byMessage, err)
	}
	if _, err := f.m.ModerationStore.GetRemoval(ctx, "rem-missing"); !errors.Is(err, models.ErrModerationNotFound) {
		t.Fatalf("GetRemoval of a missing id = %v, want ErrModerationNotFound", err)
	}
	if _, err := f.m.ModerationStore.GetRemovalForMessage(ctx, "msg-none"); !errors.Is(err, models.ErrModerationNotFound) {
		t.Fatalf("GetRemovalForMessage of a message never removed = %v, want ErrModerationNotFound", err)
	}
	f.removal(t, "msg-2", "steward-1")
	list, err := f.m.ModerationStore.ListRemovalsForStructure(ctx, "s-1")
	if err != nil || len(list) != 2 {
		t.Fatalf("removals of s-1 = %+v, %v", list, err)
	}
}

func TestCover13ADisputeNeedsARemovalAndStartsOpen(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	if _, err := f.m.ModerationStore.CreateDispute(ctx, models.Dispute{RemovalID: "rem-missing", RaisedBy: "mem-1", Statement: "unfair", ReviewStructureID: "s-r"}); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("a dispute of a missing removal = %v, want ErrInvalidReference", err)
	}
	rem := f.removal(t, "msg-1", "steward-1")
	d, err := f.m.ModerationStore.CreateDispute(ctx, models.Dispute{
		RemovalID: rem.ID, RaisedBy: "mem-1", Statement: "unfair", ReviewStructureID: "s-r",
		State: models.DisputeUpheld, DecidedBy: "someone",
	})
	if err != nil {
		t.Fatalf("CreateDispute: %v", err)
	}
	if d.State != models.DisputeOpen || d.DecidedBy != "" || d.DecidedAt != nil || d.HeldSince.IsZero() || d.RaisedAt.IsZero() {
		t.Fatalf("dispute = %+v; a new dispute is open and undecided whatever the caller sent", d)
	}
	got, err := f.m.ModerationStore.GetDispute(ctx, d.ID)
	if err != nil || got.RemovalID != rem.ID {
		t.Fatalf("GetDispute = %+v, %v", got, err)
	}
	byRemoval, err := f.m.ModerationStore.GetDisputeForRemoval(ctx, rem.ID)
	if err != nil || byRemoval.ID != d.ID {
		t.Fatalf("GetDisputeForRemoval = %+v, %v", byRemoval, err)
	}
	if _, err := f.m.ModerationStore.GetDispute(ctx, "disp-missing"); !errors.Is(err, models.ErrModerationNotFound) {
		t.Fatalf("GetDispute of a missing id = %v, want ErrModerationNotFound", err)
	}
	if _, err := f.m.ModerationStore.GetDisputeForRemoval(ctx, "rem-other"); !errors.Is(err, models.ErrModerationNotFound) {
		t.Fatalf("GetDisputeForRemoval of an undisputed removal = %v, want ErrModerationNotFound", err)
	}
}

func TestCover13DecidingADisputeRestoresOrUpholdsAndWritesTheAct(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	rem := f.removal(t, "msg-1", "steward-1")
	d, err := f.m.ModerationStore.CreateDispute(ctx, models.Dispute{RemovalID: rem.ID, RaisedBy: "mem-1", Statement: "unfair", ReviewStructureID: "s-r"})
	if err != nil {
		t.Fatalf("CreateDispute: %v", err)
	}
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	if _, err := f.m.ModerationStore.DecideDispute(ctx, d.ID, models.DisputeReinstated, "steward-1", at, "", models.Actor{ID: "steward-1"}); !errors.Is(err, models.ErrReviewerIsRemover) {
		t.Fatalf("the remover deciding their own removal = %v, want ErrReviewerIsRemover", err)
	}
	if got, _ := f.m.ModerationStore.GetDispute(ctx, d.ID); got.State != models.DisputeOpen {
		t.Fatalf("a refused decision leaves the dispute open, got %q", got.State)
	}
	decided, err := f.m.ModerationStore.DecideDispute(ctx, d.ID, models.DisputeReinstated, "reviewer-1", at, "s-r", models.Actor{ID: "reviewer-1", Label: "Reviewer"})
	if err != nil {
		t.Fatalf("DecideDispute: %v", err)
	}
	if decided.State != models.DisputeReinstated || decided.DecidedBy != "reviewer-1" || decided.DecidedAt == nil || !decided.DecidedAt.Equal(at) {
		t.Fatalf("decided = %+v", decided)
	}
	acts := f.acts.written()
	last := acts[len(acts)-1]
	if last.Kind != models.ActPostRestored || last.SubjectRef != "s-r" || last.Detail["dispute_id"] != d.ID || last.Detail["removal_id"] != rem.ID {
		t.Fatalf("the reinstating act = %+v", last)
	}
	if _, err := f.m.ModerationStore.DecideDispute(ctx, d.ID, models.DisputeUpheld, "reviewer-2", at, "", models.Actor{ID: "reviewer-2"}); !errors.Is(err, models.ErrAlreadyDecided) {
		t.Fatalf("deciding twice = %v, want ErrAlreadyDecided", err)
	}
	if _, err := f.m.ModerationStore.DecideDispute(ctx, "disp-missing", models.DisputeUpheld, "reviewer-2", at, "", models.Actor{ID: "reviewer-2"}); !errors.Is(err, models.ErrModerationNotFound) {
		t.Fatalf("deciding a missing dispute = %v, want ErrModerationNotFound", err)
	}

	other := f.removal(t, "msg-2", "steward-1")
	d2, _ := f.m.ModerationStore.CreateDispute(ctx, models.Dispute{RemovalID: other.ID, RaisedBy: "mem-2", Statement: "no", ReviewStructureID: "s-r"})
	upheld, err := f.m.ModerationStore.DecideDispute(ctx, d2.ID, models.DisputeUpheld, "reviewer-1", at, "", models.Actor{ID: "reviewer-1"})
	if err != nil || upheld.State != models.DisputeUpheld {
		t.Fatalf("upholding = %+v, %v", upheld, err)
	}
	acts = f.acts.written()
	if got := acts[len(acts)-1].Kind; got != models.ActRemovalLeftStanding {
		t.Fatalf("an upheld removal writes %q, want removal_left_standing", got)
	}
}

func TestCover13DismissingReportsLeavesThemStandingUnlessTheMessageIsAlreadyWithheld(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	d, err := f.m.ModerationStore.DismissReports(ctx, models.Dismissal{
		MessageID: "msg-1", StructureID: "s-1", DismissedBy: "steward-1", ActorRoleClass: "steward", Reason: models.RemovalReasonAbuse,
	}, "", models.Actor{ID: "steward-1", Label: "Steward"})
	if err != nil || d.ID == "" || d.DismissedAt.IsZero() {
		t.Fatalf("DismissReports = %+v, %v", d, err)
	}
	acts := f.acts.written()
	if len(acts) != 1 || acts[0].Kind != models.ActReportLeftStanding || acts[0].SubjectID != "msg-1" || acts[0].Detail["dismissal_id"] != d.ID {
		t.Fatalf("acts = %+v", acts)
	}
	f.removal(t, "msg-2", "steward-1")
	_, err = f.m.ModerationStore.DismissReports(ctx, models.Dismissal{
		MessageID: "msg-2", StructureID: "s-1", DismissedBy: "steward-1", ActorRoleClass: "steward", Reason: models.RemovalReasonAbuse,
	}, "", models.Actor{ID: "steward-1"})
	if !errors.Is(err, models.ErrAlreadyRemoved) {
		t.Fatalf("dismissing reports of a withheld message = %v, want ErrAlreadyRemoved", err)
	}
	if n := len(f.acts.written()); n != 2 {
		t.Fatalf("acts = %d; the refused dismissal writes none (the removal wrote one)", n)
	}
}

func TestCover13TheRoomAndReporterViewsOfModerationRowsHideTheActor(t *testing.T) {
	rem := models.Removal{ID: "r", RemovedBy: "steward-1", Reason: models.RemovalReasonAbuse}
	if got := rem.Room(); got.RemovedBy != "" || got.ID != "r" || got.Reason != models.RemovalReasonAbuse {
		t.Fatalf("Room() = %+v; only the remover is hidden", got)
	}
	if rem.RemovedBy != "steward-1" {
		t.Fatal("Room() must not change the original")
	}
	d := models.Dismissal{ID: "d", DismissedBy: "steward-1", Reason: models.RemovalReasonAbuse, Note: "n", MessageID: "m"}
	got := d.Reporter()
	if got.DismissedBy != "" || got.Reason != "" || got.Note != "" || got.ID != "d" || got.MessageID != "m" {
		t.Fatalf("Reporter() = %+v; the actor, reason and note are hidden from a reporter", got)
	}
}
