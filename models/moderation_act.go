package models

// The chapter act-log rows moderation writes — DEV-1341.
//
// This package cannot import the gateway's internal/domain/custody (Go
// `internal/` visibility forbids it, and mwanachama-backend-api-gateway
// imports this module, so the reverse import would be circular). ActEntry
// and ActKind are this package's own minimal, narrowed copy of exactly the
// fields moderation ever populates on custody.ChapterActLogEntry — never
// EscalationLevel/ToChapterID, which belong to a different act
// (ActCaseEscalated) this domain never writes. ActWriter/MemoryActWriter are
// the seam the gateway plugs its own custody store into at construction
// time, so the moderation write and the act-log write can still happen in
// one transaction (see CLAUDE.md).
//
// ActKind's four string values (post_withheld, post_restored,
// removal_left_standing, report_left_standing) MUST match
// custody.ActKind's identical literals in the gateway byte for byte — the
// gateway's adapter casts by value, not by name. A guard test in the gateway
// (internal/store/postgres) asserts custody.ActClassOf classifies all four.
//
// What the rows say is read off M106 leader-act-log:
//
//   - **SubjectRef is the wall**, not the post — `Post withheld · Shinyalu
//     Ward wall`, `Report left standing · Shinyalu Ward wall`. It is
//     resolved by the handler and passed down, for the reason the log
//     stores actor_label rather than joining it: a chapter can be renamed
//     or retired and the row still has to render.
//   - **SubjectID is the message**, which is what makes a withholding and
//     its later restoration one post's story on the log rather than two
//     unrelated rows.

import (
	"context"
	"database/sql"
)

// ActKind is comm's own copy of the four act kinds moderation writes to the
// gateway's chapter act log. Values must match custody.ActKind exactly.
type ActKind string

const (
	ActPostWithheld        ActKind = "post_withheld"
	ActPostRestored        ActKind = "post_restored"
	ActRemovalLeftStanding ActKind = "removal_left_standing"
	ActReportLeftStanding  ActKind = "report_left_standing"
)

// ActEntry is the narrowed shape moderation hands to an ActWriter — every
// field custody.ChapterActLogEntry needs from a moderation act, and nothing
// custody computes itself (ID, OccurredAt, Class).
type ActEntry struct {
	ChapterID      string
	Kind           ActKind
	ActorID        string
	ActorLabel     string
	ActorChapterID string
	SubjectRef     string
	SubjectID      string
	Detail         map[string]any
}

// ActWriter writes one ActEntry to the chapter act log as part of the
// caller's own Postgres transaction. The gateway's adapter implementation
// casts ActEntry into a custody.ChapterActLogEntry and calls its own
// (unexported) insertAct — comm never sees the custody type.
type ActWriter interface {
	WriteAct(ctx context.Context, tx *sql.Tx, entry ActEntry) error
}

// MemoryActWriter is the in-memory backend's equivalent of ActWriter — no
// transaction to pass through, since the memory store has no transactions.
type MemoryActWriter interface {
	WriteAct(ctx context.Context, entry ActEntry) error
}

// Actor is who performed the moderation act, as the log will name them.
type Actor struct {
	ID        string
	Label     string
	ChapterID string
}

// WithheldAct composes the `post_withheld` entry for a removal.
//
// The reason **is** written here: a removal carries `RemovalReason` from its
// own closed enum, so the detail is the act's own vocabulary rather than a
// sentence somebody typed.
//
// ActorChapterID falls back to the removal's own chapter. Empty is not
// neutral: the gateway's act log reads an empty `actor_chapter_id` as *an
// administrator acting over the whole structure from no chapter in it*, and
// a ward moderator is not that. `operatorAt` sets it on the guarded route,
// so the fallback covers the break-glass and unguarded paths only.
func WithheldAct(rem Removal, wall string, actor Actor) ActEntry {
	e := moderationAct(ActPostWithheld, rem, wall, actor)
	e.Detail["reason"] = string(rem.Reason)
	e.Detail["actor_role_class"] = rem.ActorRoleClass
	return e
}

// DisputeOutcomeAct is the entry point the stores call: a second copy of the
// state-to-kind test is a second chance for the two backends to log the same
// appeal differently, so both go through this one function.
//
// An unexpected state is `removal_left_standing` — the conservative reading.
func DisputeOutcomeAct(rem Removal, d Dispute, wall string, actor Actor) ActEntry {
	if d.State == DisputeReinstated {
		return RestoredAct(rem, d, wall, actor)
	}
	return LeftStandingAct(rem, d, wall, actor)
}

// RestoredAct composes the `post_restored` entry: a dispute decided the
// member's way, so the post comes back.
//
// DisputeID is in the detail rather than in SubjectID because the subject of
// this row is the **post** — the same post `post_withheld` named — and a
// reader following one post's story through the log must not have the key
// change under them halfway. The dispute is how it came back, which is
// detail.
func RestoredAct(rem Removal, d Dispute, wall string, actor Actor) ActEntry {
	return disputeOutcomeAct(ActPostRestored, rem, d, wall, actor)
}

// LeftStandingAct composes the `removal_left_standing` entry: the dispute
// was looked at and the removal stands.
//
// **Deliberately not named after the dispute's own outcome.** `Dispute.State`
// spells this `upheld` and means by it *the removal stands* — which is the
// opposite of what a reader who has just read the member's appeal expects —
// so this composer keeps that separation rather than propagating the word.
func LeftStandingAct(rem Removal, d Dispute, wall string, actor Actor) ActEntry {
	return disputeOutcomeAct(ActRemovalLeftStanding, rem, d, wall, actor)
}

// disputeOutcomeAct is the one composer both outcomes go through, so the
// pair cannot drift into describing the same dispute differently.
func disputeOutcomeAct(kind ActKind, rem Removal, d Dispute, wall string, actor Actor) ActEntry {
	e := moderationAct(kind, rem, wall, actor)
	e.Detail["dispute_id"] = d.ID
	e.Detail["removal_id"] = rem.ID
	// The reason the post was withheld in the first place, so the outcome row
	// is legible without the reader having to find the withholding above it.
	e.Detail["reason"] = string(rem.Reason)
	return e
}

// ReportLeftStandingAct composes the `report_left_standing` entry for a
// dismissal.
//
// **Not to be confused with LeftStandingAct above**:
//
//   - `removal_left_standing` — a post was withheld, its author appealed, and
//     the reviewer left the *removal* standing. The post stays down.
//   - `report_left_standing` — a post was reported, a moderator looked, and
//     left the *post* standing. The post stays up.
//
// It does NOT go through moderationAct: that composer reads a Removal, and
// there is no removal here — the post was not withheld. The shared fields
// are assembled directly.
func ReportLeftStandingAct(d Dismissal, wall string, actor Actor) ActEntry {
	if wall == "" {
		wall = d.ChapterID
	}
	if actor.ChapterID == "" {
		actor.ChapterID = d.ChapterID
	}
	return ActEntry{
		ChapterID:      d.ChapterID,
		Kind:           ActReportLeftStanding,
		ActorID:        actor.ID,
		ActorLabel:     actor.Label,
		ActorChapterID: actor.ChapterID,
		SubjectRef:     wall,
		SubjectID:      d.MessageID,
		// Structured keys only — `note` is deliberately absent: it is
		// somebody's typed sentence, and the only thing this row must still
		// mean years from now is which post and on what ground.
		Detail: map[string]any{
			"message_id":       d.MessageID,
			"reason":           string(d.Reason),
			"actor_role_class": d.ActorRoleClass,
			"dismissal_id":     d.ID,
		},
	}
}

// moderationAct is the shape all three (withheld/restored/left-standing)
// share.
//
// **RemovedBy never reaches this row's actor**, and that is not an
// oversight: message-removal.md's whole rule is that the remover is named
// *upward and in the log*, so the log's actor is whoever performed **this**
// act. On a withholding those are the same person; on a dispute outcome they
// are two different people, and G62 refuses the case where they are not
// (ErrReviewerIsRemover).
func moderationAct(kind ActKind, rem Removal, wall string, actor Actor) ActEntry {
	if wall == "" {
		wall = rem.ChapterID
	}
	if actor.ChapterID == "" {
		actor.ChapterID = rem.ChapterID
	}
	return ActEntry{
		ChapterID:      rem.ChapterID,
		Kind:           kind,
		ActorID:        actor.ID,
		ActorLabel:     actor.Label,
		ActorChapterID: actor.ChapterID,
		SubjectRef:     wall,
		SubjectID:      rem.MessageID,
		Detail: map[string]any{
			"message_id": rem.MessageID,
		},
	}
}
