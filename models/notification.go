package models

// Notification and NotificationPreference — one thing the product told one
// person, and the only lever in the set that can suppress it. Ported
// unchanged in business logic from mwanachama-backend-api-gateway's
// internal/domain/notification (DEV-1120), per the given decision that
// notification joins comm: "zero existing coupling to any other domain
// package, so the move is low-risk" (architecture-domain-decomposition.md).
//
// Every exported name here is prefixed Notification, matching this
// package's own convention for names introduced fresh rather than carried
// over from a single-domain package (see [ChatActivityQuery] etc.) — the
// gateway's own notification.Repository/.Notification/.Preference had no
// collision to force this, but models/ is shared across five domains now
// and a bare Settings-shaped name here would be the next collision waiting
// to happen.
//
// Four properties this type is about, none of which is a plain field:
//
//   - **A notification is a side-effect, never a composer.** Nothing here
//     takes a title or a body: a row is raised by an act taken on some
//     other screen, and the title/preview are rendered from (Event,
//     SubjectKind, SubjectID).
//   - **The two caps are uniqueness, not counters.** One reminder per
//     member per survey, one nudge per chapter per survey — enforced by
//     partial unique indexes (see gormstore), not a read-then-write.
//   - **Absence means on.** No row is written at enrollment, so the
//     suppression read is "not exists (… where muted)".
//   - **Two categories can never be muted.** `survey` and `security` are
//     refused by NotificationPreference.Validate with the reason.
//
// Validation is duplicated with the schema on purpose: this package's own
// fast-test dialect (sqlite) enforces fewer constraints than Postgres does,
// so without Validate the two backends could disagree about what a
// notification *is*.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotificationNotFound is returned when an id names no row.
var ErrNotificationNotFound = errors.New("notification: not found")

// ErrNotificationInvalid is returned by Validate. Every wrapping error below
// is an invariant the schema also states as a CHECK or partial index; the
// message names which.
var ErrNotificationInvalid = errors.New("notification: invalid row")

// ErrNotificationCapSpent is returned by NotificationRepository.Raise when
// one of the two caps is already spent — the member already holds a
// reminder for that survey, or the chapter's nudge is gone.
//
// It is its own error and not an ErrNotificationInvalid because the caller
// can act on it and draws the answer in words ("0 of 1 left").
var ErrNotificationCapSpent = errors.New("notification: that cap is already spent for this subject")

// ErrNotificationMuted is returned by Raise when the recipient has muted
// the category. Not an error the product shows anybody — the sender is
// told a count, never who received it — so the only correct handling is to
// carry on.
var ErrNotificationMuted = errors.New("notification: the recipient has muted this category")

// ErrNotificationCategoryExempt is returned by SetPreference for `survey`
// and `security`. The message is the point: a caller that never saw the
// settings screen learns the rule from the sentence itself.
var ErrNotificationCategoryExempt = errors.New("notification: `survey` and `security` always reach a member and cannot be muted")

// NotificationMaxMarkRead bounds one MarkRead call — a limit is a
// constraint, not a counter, so both backends refuse the same 201st id
// rather than one of them accepting it.
const NotificationMaxMarkRead = 200

// NotificationDefaultPage is the page size both stores fall back to when a
// caller passes limit <= 0.
const NotificationDefaultPage = 200

// Notification is one row of the notification table.
//
// There is no Title, Preview or Body field, and that absence is the point:
// nothing in the product writes a notification body; the client renders
// both from Event and the subject it names.
type Notification struct {
	ID       string
	MemberID string

	// AuthorMemberID is authorship, not delivery: the acting admin, set
	// only where a human composed the wording. Empty for every row the
	// system raises on its own.
	AuthorMemberID string

	// SeatRoleKindID and SeatChapterID are the address for a notice sent to
	// a seat rather than a person, kept beside the member it resolved to.
	// Both set or both empty — Validate refuses half of one.
	SeatRoleKindID string
	SeatChapterID  string

	Category    NotificationCategory
	Event       NotificationEvent
	SubjectKind NotificationSubjectKind
	SubjectID   string

	// ChapterID is set for the reminder and the nudge — acts taken against
	// a chapter — and empty for a My Record event, which is individually
	// earned.
	ChapterID string

	CreatedAt time.Time

	// ReadAt is the recipient's own assertion and the only column any
	// client writes. Nothing stamps it server-side. Nil is unread.
	ReadAt *time.Time
}

// Validate reports whether the row is one the schema would accept.
func (n Notification) Validate() error {
	if strings.TrimSpace(n.MemberID) == "" {
		return fmt.Errorf("%w: a notification has a recipient", ErrNotificationInvalid)
	}
	if !IsNotificationCategory(n.Category) {
		return fmt.Errorf("%w: %q is not a category", ErrNotificationInvalid, n.Category)
	}
	c, ok := NotificationCategoryOf(n.Event)
	if !ok {
		return fmt.Errorf("%w: %q is not an event this product raises", ErrNotificationInvalid, n.Event)
	}
	// The category is derived, so a mismatch is a caller that decided
	// one — which is the road to an event raised under a mutable category.
	if c != n.Category {
		return fmt.Errorf("%w: event %q is category %q, not %q", ErrNotificationInvalid, n.Event, c, n.Category)
	}
	if !IsNotificationSubjectKind(n.SubjectKind) {
		return fmt.Errorf("%w: %q is not a subject kind", ErrNotificationInvalid, n.SubjectKind)
	}
	if strings.TrimSpace(n.SubjectID) == "" {
		return fmt.Errorf("%w: (subject_kind, subject_id) is the address and needs both halves", ErrNotificationInvalid)
	}
	if (n.SeatRoleKindID == "") != (n.SeatChapterID == "") {
		return fmt.Errorf("%w: a seat address is both halves or neither", ErrNotificationInvalid)
	}
	// The two capped events are scoped, and a row that carries no scope
	// cannot be capped by an index over that scope.
	if n.Event == EventSurveyNudge && strings.TrimSpace(n.ChapterID) == "" {
		return fmt.Errorf("%w: a nudge is capped per chapter, so it names one", ErrNotificationInvalid)
	}
	return nil
}

// NotificationPreference is one row of notification_preference — one
// member, one category, and whether they turned it off.
//
// Absence is on: a member with no rows receives every category, so nothing
// writes a row at enrollment and a preference not in the store is not an
// error.
type NotificationPreference struct {
	ID        string
	MemberID  string
	Category  NotificationCategory
	Muted     bool
	ChangedAt time.Time
}

// Validate reports whether the row is one the schema would accept,
// including the exempt rule, which is an authorization rule spelled in
// values rather than a vocabulary check.
func (p NotificationPreference) Validate() error {
	if strings.TrimSpace(p.MemberID) == "" {
		return fmt.Errorf("%w: a preference has a member", ErrNotificationInvalid)
	}
	if !IsNotificationCategory(p.Category) {
		return fmt.Errorf("%w: %q is not a category", ErrNotificationInvalid, p.Category)
	}
	if notificationExempt(p.Category) {
		return ErrNotificationCategoryExempt
	}
	return nil
}

// NotificationRepository is the persistence boundary for Notification and
// NotificationPreference.
//
// There is no Create, and Raise is not one: every writer is an act that is
// doing something else and inserting as a side-effect. There is no Delete
// (the one exception, a member's own deletion, is a cascade the schema
// carries, not a method). There is no un-read: MarkRead sets a timestamp
// and nothing clears one.
//
// No read method takes a caller id as an authorization argument — the
// handler proves the caller *is* the recipient before it calls, so an
// authorization rule is never one wrong call site away from being passed
// the wrong value. The one place a member id is an argument is where it is
// the subject of the row rather than the reader of it.
type NotificationRepository interface {
	// List returns one member's notifications, newest first. limit <= 0
	// means NotificationDefaultPage.
	List(ctx context.Context, memberID string, limit int) ([]Notification, error)

	// UnreadCount is the badge: count(*) where read_at is null, per member.
	UnreadCount(ctx context.Context, memberID string) (int, error)

	// MarkRead stamps read_at on the caller's own rows and returns how many
	// it changed.
	//
	// The timestamp is the caller's, not now(): a member reads offline and
	// the handset syncs later, so when they read it is a fact only the
	// handset holds. Ids that are not this member's are ignored rather
	// than refused, and already-read rows are left alone, so a retry is
	// idempotent. More than NotificationMaxMarkRead ids is
	// ErrNotificationInvalid.
	MarkRead(ctx context.Context, memberID string, ids []string, readAt time.Time) (int, error)

	// Raise inserts one notification as the side-effect of an act, and is
	// the only way a row is ever written.
	//
	// It applies two rules a caller must not be trusted with: the mute (if
	// the recipient has muted n.Category, nothing is written and the
	// answer is ErrNotificationMuted — the exempt pair is never muted,
	// since SetPreference cannot store a row for them) and the caps (a
	// second survey_reminder for the same (member, survey), or a second
	// survey_nudge for the same (chapter, survey), is ErrNotificationCapSpent
	// — on Postgres that is the partial unique index answering, not a
	// read-then-write, so two concurrent presses cannot both pass a check
	// and both insert).
	Raise(ctx context.Context, n Notification) (Notification, error)

	// ListPreferences returns the rows one member has actually written. It
	// does not return a row per category — absence means on, so a member
	// who never opened the settings screen gets an empty slice and every
	// category reaches them.
	ListPreferences(ctx context.Context, memberID string) ([]NotificationPreference, error)

	// SetPreference upserts on (member_id, category). It carries no clock
	// argument, so back-dating changed_at is unsayable rather than
	// refused, and it takes the member from the caller's session rather
	// than as a forgeable field. `survey` and `security` are
	// ErrNotificationCategoryExempt, with the reason as the message.
	SetPreference(ctx context.Context, memberID string, category NotificationCategory, muted bool) (NotificationPreference, error)

	// IsMuted is the send-time read Raise makes, exported so a caller that
	// will eventually raise can ask the same question without writing.
	// "not exists (… where muted)": absent is on.
	IsMuted(ctx context.Context, memberID string, category NotificationCategory) (bool, error)
}
