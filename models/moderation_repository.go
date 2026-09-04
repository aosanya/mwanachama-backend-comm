package models

// ModerationRepository's own file, split from moderation.go's domain types
// — mirrors this module's original moderation.go/moderation_repository.go
// split ([[file-length-limit]]), which this refactor otherwise merged.

import (
	"context"
	"time"
)

// ModerationRepository is the persistence boundary for the moderation
// domain.
//
// Chapter-scoping (who may file/read a report, who may remove a message, who
// may raise/decide a dispute) is enforced in the gateway's internal/api/http
// via operatorAt/hasCapabilityAt/requireChapterMember, the same invariant
// every other domain in this port follows — this interface carries no
// capability check of its own. The one authorization rule that IS enforced
// here rather than at the HTTP layer is G62 (DecideDispute refuses reviewer
// == remover): it depends on this store's own Removal row, not on a chapter
// or a capability, so it belongs beside the write it guards.
type ModerationRepository interface {
	// FileReport inserts a message_report row. Returns ErrConflict when
	// (MessageID, ReportedBy) already exists — message-report.md's
	// unique(message_id, reported_by), reported back as a plain "already
	// reported" rather than a 500.
	FileReport(ctx context.Context, r Report) (Report, error)

	// ListReportsForMessage returns every report against one message, newest
	// first — M28's "one row opened: every report against the message by
	// name, class and time".
	ListReportsForMessage(ctx context.Context, messageID string) ([]Report, error)

	// ListReportQueue returns every report filed against a message in
	// chapterID, newest first. Not pre-aggregated by message — see the
	// package doc's scope note.
	ListReportQueue(ctx context.Context, chapterID string) ([]Report, error)

	// CreateRemoval inserts a message_removal row **and writes the chapter's
	// `post_withheld` act-log row in the same transaction** — DEV-1341 (see
	// ActWriter). Returns ErrConflict when MessageID already has one —
	// "one removal per message" — before either write, so a repeat writes no
	// second log row.
	//
	// wall is what M106 renders as the subject — `Shinyalu Ward wall` — and is
	// resolved by the handler, because the store has no chapter names.
	CreateRemoval(ctx context.Context, rem Removal, wall string, actor Actor) (Removal, error)

	// DismissReports inserts a message_report_dismissal row — G79's third
	// outcome — **and writes the chapter's `report_left_standing` act-log row
	// in the same transaction** (DEV-1351, closing DEV-1341's fourth kind).
	//
	// Keyed on the message, not on a report: one dismissal ends every report
	// against that message, which is message-report.md's "six people reporting
	// one post do not each have their own outcome".
	//
	// Two refusals, both before the log is touched, so only a dismissal that
	// happened is logged:
	//   - ErrConflict when MessageID already has a dismissal — one per
	//     message, the same shape CreateRemoval's is.
	//   - ErrAlreadyRemoved when the message is already withheld.
	//
	// wall is resolved by the handler and is what M106 renders as the subject
	// — `Report left standing · Shinyalu Ward wall`.
	DismissReports(ctx context.Context, d Dismissal, wall string, actor Actor) (Dismissal, error)

	// GetDismissalForMessage returns the dismissal naming messageID, if one
	// exists. ErrModerationNotFound when the message's reports have never
	// been left standing — a real state (most messages), not a failure, the
	// same way GetRemovalForMessage's is.
	GetDismissalForMessage(ctx context.Context, messageID string) (Dismissal, error)

	// GetRemoval returns one removal by id. ErrModerationNotFound if unknown.
	GetRemoval(ctx context.Context, id string) (Removal, error)

	// GetRemovalForMessage returns the removal naming messageID, if one
	// exists. ErrModerationNotFound when the message has never been removed —
	// a real state (most messages), not a failure.
	GetRemovalForMessage(ctx context.Context, messageID string) (Removal, error)

	// ListRemovalsForChapter returns every removal at chapterID, newest
	// first.
	ListRemovalsForChapter(ctx context.Context, chapterID string) ([]Removal, error)

	// CreateDispute inserts a removal_dispute row in the Open state. Returns
	// ErrConflict when RemovalID already has one — "one dispute per
	// removal", refused as "already disputed" rather than a raw
	// unique-violation.
	CreateDispute(ctx context.Context, d Dispute) (Dispute, error)

	// GetDispute returns one dispute by id. ErrModerationNotFound if unknown.
	GetDispute(ctx context.Context, id string) (Dispute, error)

	// GetDisputeForRemoval returns the dispute naming removalID, if one
	// exists. ErrModerationNotFound when the removal has never been disputed
	// — the "Disputed" flag on the upward register is this row's existence.
	GetDisputeForRemoval(ctx context.Context, removalID string) (Dispute, error)

	// DecideDispute moves a dispute from Open to outcome (Reinstated or
	// Upheld), stamping DecidedBy/DecidedAt=now.
	//
	// Refuses with ErrAlreadyDecided when the dispute is not Open, and with
	// ErrReviewerIsRemover when decidedBy equals the disputed removal's
	// RemovedBy (G62) — both checked here rather than left to the caller,
	// because both depend on rows only this store already has open.
	//
	// It also writes the outcome's act-log row in the same transaction —
	// `post_restored` or `removal_left_standing`, chosen by
	// DisputeOutcomeAct from the state just written (DEV-1341). Both
	// refusals above return before the log is touched, so only a decision
	// that happened is logged.
	DecideDispute(ctx context.Context, id string, outcome DisputeState, decidedBy string, now time.Time, wall string, actor Actor) (Dispute, error)
}
