package models

// NotificationCategory, NotificationEvent and NotificationSubjectKind — the
// notification vocabulary. Split out of notification.go for the same
// file-length reason the gateway's own category.go was: the closed
// vocabulary a notification is described in lives together, separate from
// the rows and their Validate.

// NotificationCategory is the opt-out unit. It is a stored column and not
// a kind derived from Event, because the member switches a *category* off
// and the enum grows with the product.
type NotificationCategory string

const (
	// CategorySurvey — a survey reached you, and the reminder. Exempt from
	// muting.
	CategorySurvey NotificationCategory = "survey"
	// CategoryResults — "You said, we heard".
	CategoryResults NotificationCategory = "results"
	// CategoryContribution — matched, orphaned, and a report closed
	// not-found.
	CategoryContribution NotificationCategory = "contribution"
	// CategoryMerchandise — stock recorded against your name.
	CategoryMerchandise NotificationCategory = "merchandise"
	// CategoryChat — a removal receipt.
	CategoryChat NotificationCategory = "chat"
	// CategoryMembership — the member's own registration: transfers,
	// consent versions, number changes.
	//
	// `membership`, not `identity`: a member switching off a category
	// called *identity* would switch off transfers, consent versions and
	// phone changes while the review of their ID number carried on — the
	// ID-case notices live in CategorySecurity, which is what makes them
	// exempt. A category names the object it holds.
	CategoryMembership NotificationCategory = "membership"
	// CategorySecurity — OTP, device recovery, duplicate-ID case notices.
	// The only category with an SMS channel, and exempt from muting.
	CategorySecurity NotificationCategory = "security"
)

// notificationCategories is the whole vocabulary, in the order the object
// file lists it.
var notificationCategories = []NotificationCategory{
	CategorySurvey, CategoryResults, CategoryContribution,
	CategoryMerchandise, CategoryChat, CategoryMembership, CategorySecurity,
}

// NotificationCategories returns the vocabulary. A copy, so a caller
// ranging over it cannot re-point the package's own slice.
func NotificationCategories() []NotificationCategory {
	out := make([]NotificationCategory, len(notificationCategories))
	copy(out, notificationCategories)
	return out
}

// IsNotificationCategory reports whether c is one of the seven.
func IsNotificationCategory(c NotificationCategory) bool {
	for _, k := range notificationCategories {
		if k == c {
			return true
		}
	}
	return false
}

// notificationExempt is the muting exemption pair. It is a function over
// the vocabulary rather than a field on a table, because an exemption a
// row can carry is an exemption a row can lose.
func notificationExempt(c NotificationCategory) bool {
	return c == CategorySurvey || c == CategorySecurity
}

// IsNotificationCategoryExempt reports whether a category always reaches
// the member.
func IsNotificationCategoryExempt(c NotificationCategory) bool { return notificationExempt(c) }

// NotificationSMSChannel reports whether a category also goes out over
// SMS.
//
// Not a column, and it must not become one: a channel column is a channel
// somebody can widen, and per-member SMS cost must not scale with
// engagement — so the rule is a function whose only input is the category.
func NotificationSMSChannel(c NotificationCategory) bool { return c == CategorySecurity }

// NotificationEvent is the specific act a row is the by-product of. The
// title and body are rendered from this and the subject, never stored as
// text.
type NotificationEvent string

const (
	// EventSurveyReached — "New survey reached you".
	EventSurveyReached NotificationEvent = "survey_reached"
	// EventSurveyReminder — the reminder sent to one member. Capped one
	// per member per survey.
	EventSurveyReminder NotificationEvent = "survey_reminder"
	// EventSurveyNudge — the chapter-scoped half of one reminder press.
	// Capped one per chapter per survey.
	EventSurveyNudge NotificationEvent = "survey_nudge"
	// EventResultsPublished — "You said, we heard".
	EventResultsPublished NotificationEvent = "results_published"
	// EventContributionMatched — "Contribution matched".
	EventContributionMatched NotificationEvent = "contribution_matched"
	// EventContributionOrphaned — an orphaned-contribution notice, told
	// once.
	EventContributionOrphaned NotificationEvent = "contribution_orphaned"
	// EventContributionReportRefused — "Report closed — not found". It
	// carries the treasurer's note without breaking the no-body rule: the
	// note is a column on the *report*, and the renderer quotes it from
	// the subject row.
	EventContributionReportRefused NotificationEvent = "contribution_report_refused"
	// EventMerchandiseRecorded — "Merchandise recorded · Cap · issued by
	// …".
	EventMerchandiseRecorded NotificationEvent = "merchandise_recorded"
	// EventMessageRemoved — a removal receipt, which never names the
	// moderator.
	EventMessageRemoved NotificationEvent = "message_removed"
	// EventIdentityCaseOpened — a case notice, which never names the other
	// member.
	EventIdentityCaseOpened NotificationEvent = "identity_case_opened"
	// EventIdentityCaseClosed — the same case, ended.
	EventIdentityCaseClosed NotificationEvent = "identity_case_closed"
	// EventEnrollmentKeyRotated — a rotation notice, the one notification
	// in the set with an author. Its subject is the *retired* key, never
	// the successor, because the successor's identifier may not cross
	// into a notice.
	EventEnrollmentKeyRotated NotificationEvent = "enrollment_key_rotated"
	// EventMemberTransferred — a transfer notice.
	EventMemberTransferred NotificationEvent = "member_transferred"
	// EventConsentVersionPublished — a new consent version.
	EventConsentVersionPublished NotificationEvent = "consent_version_published"
	// EventNumberChanged — a number-on-file change.
	EventNumberChanged NotificationEvent = "number_changed"
	// EventDeviceSignedOut — a security notice: a bare fact with no link,
	// no control, no person, no place.
	EventDeviceSignedOut NotificationEvent = "device_signed_out"
)

// notificationEventCategory is the whole vocabulary of events, each mapped
// to the category it is opted out of under.
//
// The map is the vocabulary, so an event with no category cannot exist:
// Validate refuses one — a value added without an arm here fails at the
// write rather than relocating the failure into a migration nobody
// re-reads.
var notificationEventCategory = map[NotificationEvent]NotificationCategory{
	EventSurveyReached:             CategorySurvey,
	EventSurveyReminder:            CategorySurvey,
	EventSurveyNudge:               CategorySurvey,
	EventResultsPublished:          CategoryResults,
	EventContributionMatched:       CategoryContribution,
	EventContributionOrphaned:      CategoryContribution,
	EventContributionReportRefused: CategoryContribution,
	EventMerchandiseRecorded:       CategoryMerchandise,
	EventMessageRemoved:            CategoryChat,
	EventIdentityCaseOpened:        CategorySecurity,
	EventIdentityCaseClosed:        CategorySecurity,
	EventEnrollmentKeyRotated:      CategoryMembership,
	EventMemberTransferred:         CategoryMembership,
	EventConsentVersionPublished:   CategoryMembership,
	EventNumberChanged:             CategoryMembership,
	EventDeviceSignedOut:           CategorySecurity,
}

// NotificationCategoryOf returns the category an event is opted out under,
// and whether the event is known at all.
//
// The caller never chooses the category: it is derived here so that an
// event cannot be raised under a category that would let a member mute it.
func NotificationCategoryOf(e NotificationEvent) (NotificationCategory, bool) {
	c, ok := notificationEventCategory[e]
	return c, ok
}

// NotificationSubjectKind names the table (SubjectKind, SubjectID)
// addresses. Not a foreign key, because it points at nine of them.
type NotificationSubjectKind string

const (
	SubjectSurvey             NotificationSubjectKind = "survey"
	SubjectContribution       NotificationSubjectKind = "contribution"
	SubjectContributionReport NotificationSubjectKind = "contribution_report"
	SubjectHandout            NotificationSubjectKind = "handout"
	SubjectMessage            NotificationSubjectKind = "message"
	SubjectIdentityCase       NotificationSubjectKind = "identity_case"
	SubjectDevice             NotificationSubjectKind = "device"
	SubjectEnrollmentKey      NotificationSubjectKind = "enrollment_key"
	SubjectMemberRegistration NotificationSubjectKind = "member_registration"
)

var notificationSubjectKinds = []NotificationSubjectKind{
	SubjectSurvey, SubjectContribution, SubjectContributionReport,
	SubjectHandout, SubjectMessage, SubjectIdentityCase, SubjectDevice,
	SubjectEnrollmentKey, SubjectMemberRegistration,
}

// IsNotificationSubjectKind reports whether k is one of the nine.
func IsNotificationSubjectKind(k NotificationSubjectKind) bool {
	for _, s := range notificationSubjectKinds {
		if s == k {
			return true
		}
	}
	return false
}
