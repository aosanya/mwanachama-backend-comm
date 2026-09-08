package models

import (
	"errors"
	"time"
)

// ErrModerationNotFound is returned when a report, removal or dispute id has
// no row.
var ErrModerationNotFound = errors.New("moderation: not found")

// ErrAlreadyDecided is returned by the decide path when the dispute is no
// longer open. removal-dispute.md · Lifecycle: "the outcome is recorded
// once".
var ErrAlreadyDecided = errors.New("moderation: dispute already decided")

var ErrReviewerIsRemover = errors.New("moderation: the reviewer of a disputed removal may not be the actor who removed it")

// ErrAlreadyRemoved is returned when a dismissal is asked for on a message
// that is already withheld. `Removed` and `Left standing` are two states of
// one post, not two rows a post may hold at once — message-report.md: "there
// is no status column on this table … `Removed` is the presence of a
// post-removal; `Left standing` is the presence of a dismissal".
var ErrAlreadyRemoved = errors.New("moderation: the message is already withheld, so its reports cannot be left standing")

type ReportReason string

const (
	ReportReasonThreats    ReportReason = "threats"
	ReportReasonAbuse      ReportReason = "abuse"
	ReportReasonFalseClaim ReportReason = "false_claim"
	ReportReasonSpam       ReportReason = "spam"
)

// RemovalReason is the class a moderator withholds a message under. One
// fewer option than ReportReason — spam is a reason to ask a moderator to
// look, not itself a ground the product lets a moderator withhold a message
// on (message-removal.md's three classes).
type RemovalReason string

const (
	RemovalReasonThreats    RemovalReason = "threats"
	RemovalReasonAbuse      RemovalReason = "abuse"
	RemovalReasonFalseClaim RemovalReason = "false_claim"
)

// DisputeState is where a removal_dispute sits. Open until a reviewer
// decides one of exactly two outcomes — M31 draws neither as primary.
type DisputeState string

const (
	DisputeOpen       DisputeState = "open"
	DisputeReinstated DisputeState = "reinstated"
	DisputeUpheld     DisputeState = "upheld"
)

type Report struct {
	ID string `json:"id"`
	// MessageID is the reported message. Many reports may name one message —
	// message-report.md: "the queue aggregates by this".
	MessageID   string `json:"message_id"`
	StructureID string `json:"structure_id"`
	// ReportedBy is shown to the moderator and never to the author (G47).
	ReportedBy string `json:"reported_by"`
	// ReporterRoleClass is a snapshot at report time — "a reporter promoted
	// next month must not re-write how last month's report reads".
	ReporterRoleClass string       `json:"reporter_role_class"`
	Reason            ReportReason `json:"reason"`
	// Note is additive, never a substitute for Reason.
	Note string `json:"note,omitempty"`
	// Excerpt is cut from the message body at report time, frozen — editing
	// the message afterwards does not change what the moderator reads.
	Excerpt    string    `json:"excerpt"`
	ReportedAt time.Time `json:"reported_at"`
}

// Removal is the act of withholding one message from a room. It is not a
// delete: the row is additive evidence next to the message, which this port
// never deletes or edits either — message-removal.md: "the row persists, the
// content is withheld".
type Removal struct {
	ID string `json:"id"`
	// MessageID is unique: one removal per message, and "Removed" is the
	// existence of this row.
	MessageID   string `json:"message_id"`
	StructureID string `json:"structure_id"`
	// RemovedBy is named upward and in the log, never in the room and never
	// to the author (message-removal.md). Room() strips it — see below.
	RemovedBy string `json:"removed_by,omitempty"`
	// ActorRoleClass is the only identity the room and the author ever see.
	// Snapshot at removal time; does not re-write itself if the actor's role
	// changes later.
	ActorRoleClass string        `json:"actor_role_class"`
	Reason         RemovalReason `json:"reason"`
	RemovedAt      time.Time     `json:"removed_at"`
}

// Room returns the room-safe projection of a removal: ActorRoleClass, Reason
// and RemovedAt, and never RemovedBy. This is this port's Go equivalent of
// the message_removal_room view — "the room sees a role, the level above
// sees a name", G47's anonymity enforced by what the projection omits, not
// by a client choosing what to render.
func (r Removal) Room() Removal {
	r.RemovedBy = ""
	return r
}

// Dismissal is a moderator looking at a message's reports and leaving the
// post standing — G79's third outcome.
//
// **Keyed on the message, not on a report**, which is message-report.md's own
// rule: "six people reporting one post do not each have their own outcome".
type Dismissal struct {
	ID string `json:"id"`
	// MessageID is unique: one dismissal per message, and "Left standing" is
	// the existence of this row.
	MessageID   string `json:"message_id"`
	StructureID string `json:"structure_id"`
	// DismissedBy is named upward and in the log, never to the reporters —
	// the same asymmetry Removal.RemovedBy carries. Reporter() strips it.
	DismissedBy string `json:"dismissed_by,omitempty"`
	// ActorRoleClass is the only identity a reporter ever sees.
	ActorRoleClass string `json:"actor_role_class"`
	// Reason is required, and it is RemovalReason rather than ReportReason on
	// purpose: the reason lives in the ancestor-readable act log, never in
	// the reporter's receipt.
	Reason RemovalReason `json:"reason"`
	// Note is additive and, like Reason, upward-only.
	Note        string    `json:"note,omitempty"`
	DismissedAt time.Time `json:"dismissed_at"`
}

// Reporter returns the projection a reporter may read: the outcome and its
// date, and neither who decided nor why.
//
// The Go equivalent of Removal.Room(), and it exists for the same reason —
// G47's asymmetry enforced by what the projection omits rather than by a
// client choosing what to render.
func (d Dismissal) Reporter() Dismissal {
	d.DismissedBy = ""
	d.Reason = ""
	d.Note = ""
	return d
}

type Dispute struct {
	ID string `json:"id"`
	// RemovalID is unique: one dispute per removal, and the flag on the
	// upward register is this row's existence.
	RemovalID string `json:"removal_id"`
	// RaisedBy is always the message's author — nobody else may dispute.
	RaisedBy string `json:"raised_by"`
	// Statement is the author's own words, free text, no class column —
	// deliberate: "it travels as written".
	Statement         string    `json:"statement"`
	ReviewStructureID string    `json:"review_structure_id"`
	HeldSince         time.Time `json:"held_since"`
	RaisedAt          time.Time `json:"raised_at"`
	// State is open until decided. reinstated is "Put it back"; upheld is
	// "Leave it removed".
	State DisputeState `json:"state"`
	// DecidedBy/DecidedAt are set together, only once, by the decide path.
	DecidedBy string     `json:"decided_by,omitempty"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
}

// ModerationRepository, the persistence boundary this domain's types are
// read and written through, is in its own file — see
// moderation_repository.go.
