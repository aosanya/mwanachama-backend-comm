package models

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

var ErrNotificationCapSpent = errors.New("notification: that cap is already spent for this subject")

// ErrNotificationMuted is returned by Raise when the recipient has muted
// the category. Not an error the product shows anybody — the sender is
// told a count, never who received it — so the only correct handling is to
// carry on.
var ErrNotificationMuted = errors.New("notification: the recipient has muted this category")

// ErrNotificationCategoryExempt is returned by SetPreference for `survey`
// and `security`. The message is the point: a caller that never saw the
// settings screen learns the rule from the sentence itself.
var ErrNotificationCategoryExempt = errors.New("notification: `survey` and `security` always reach a actor and cannot be muted")

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
	ID      string
	ActorID string

	AuthorActorID string

	SeatRoleKindID  string
	SeatStructureID string

	Category    NotificationCategory
	Event       NotificationEvent
	SubjectKind NotificationSubjectKind
	SubjectID   string

	StructureID string

	CreatedAt time.Time

	// ReadAt is the recipient's own assertion and the only column any
	// client writes. Nothing stamps it server-side. Nil is unread.
	ReadAt *time.Time
}

// Validate reports whether the row is one the schema would accept.
func (n Notification) Validate() error {
	if strings.TrimSpace(n.ActorID) == "" {
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
	if (n.SeatRoleKindID == "") != (n.SeatStructureID == "") {
		return fmt.Errorf("%w: a seat address is both halves or neither", ErrNotificationInvalid)
	}
	// The two capped events are scoped, and a row that carries no scope
	// cannot be capped by an index over that scope.
	if n.Event == EventSurveyNudge && strings.TrimSpace(n.StructureID) == "" {
		return fmt.Errorf("%w: a nudge is capped per structure, so it names one", ErrNotificationInvalid)
	}
	return nil
}

type NotificationPreference struct {
	ID        string
	ActorID   string
	Category  NotificationCategory
	Muted     bool
	ChangedAt time.Time
}

// Validate reports whether the row is one the schema would accept,
// including the exempt rule, which is an authorization rule spelled in
// values rather than a vocabulary check.
func (p NotificationPreference) Validate() error {
	if strings.TrimSpace(p.ActorID) == "" {
		return fmt.Errorf("%w: a preference has a actor", ErrNotificationInvalid)
	}
	if !IsNotificationCategory(p.Category) {
		return fmt.Errorf("%w: %q is not a category", ErrNotificationInvalid, p.Category)
	}
	if notificationExempt(p.Category) {
		return ErrNotificationCategoryExempt
	}
	return nil
}

type NotificationRepository interface {
	List(ctx context.Context, actorID string, limit int) ([]Notification, error)

	UnreadCount(ctx context.Context, actorID string) (int, error)

	MarkRead(ctx context.Context, actorID string, ids []string, readAt time.Time) (int, error)

	Raise(ctx context.Context, n Notification) (Notification, error)

	ListPreferences(ctx context.Context, actorID string) ([]NotificationPreference, error)

	SetPreference(ctx context.Context, actorID string, category NotificationCategory, muted bool) (NotificationPreference, error)

	// IsMuted is the send-time read Raise makes, exported so a caller that
	// will eventually raise can ask the same question without writing.
	// "not exists (… where muted)": absent is on.
	IsMuted(ctx context.Context, actorID string, category NotificationCategory) (bool, error)
}
