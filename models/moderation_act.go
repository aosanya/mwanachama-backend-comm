package models

import (
	"context"
	"database/sql"
)

type ActKind string

const (
	ActPostWithheld        ActKind = "post_withheld"
	ActPostRestored        ActKind = "post_restored"
	ActRemovalLeftStanding ActKind = "removal_left_standing"
	ActReportLeftStanding  ActKind = "report_left_standing"
)

type ActEntry struct {
	StructureID      string
	Kind             ActKind
	ActorID          string
	ActorLabel       string
	ActorStructureID string
	SubjectRef       string
	SubjectID        string
	Detail           map[string]any
}

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
	ID          string
	Label       string
	StructureID string
}

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

func RestoredAct(rem Removal, d Dispute, wall string, actor Actor) ActEntry {
	return disputeOutcomeAct(ActPostRestored, rem, d, wall, actor)
}

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
		wall = d.StructureID
	}
	if actor.StructureID == "" {
		actor.StructureID = d.StructureID
	}
	return ActEntry{
		StructureID:      d.StructureID,
		Kind:             ActReportLeftStanding,
		ActorID:          actor.ID,
		ActorLabel:       actor.Label,
		ActorStructureID: actor.StructureID,
		SubjectRef:       wall,
		SubjectID:        d.MessageID,
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
		wall = rem.StructureID
	}
	if actor.StructureID == "" {
		actor.StructureID = rem.StructureID
	}
	return ActEntry{
		StructureID:      rem.StructureID,
		Kind:             kind,
		ActorID:          actor.ID,
		ActorLabel:       actor.Label,
		ActorStructureID: actor.StructureID,
		SubjectRef:       wall,
		SubjectID:        rem.MessageID,
		Detail: map[string]any{
			"message_id": rem.MessageID,
		},
	}
}
