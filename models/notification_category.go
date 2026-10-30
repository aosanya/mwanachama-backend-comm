package models

// NotificationCategory, NotificationEvent and NotificationSubjectKind — the
// notification vocabulary. Split out of notification.go for the same
// file-length reason the gateway's own category.go was: the closed
// vocabulary a notification is described in lives together, separate from
// the rows and their Validate.

import "sort"

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
	CategoryChat       NotificationCategory = "chat"
	CategoryMembership NotificationCategory = "membership"
	// CategorySecurity — OTP, device recovery, duplicate-ID case notices.
	// The only category with an SMS channel, and exempt from muting.
	CategorySecurity NotificationCategory = "security"
	// CategoryApproval — an agent-proposed write is waiting on the actor
	// named Accountable for it (mwanachama-backend-api-kazi's K6).
	CategoryApproval NotificationCategory = "approval"
	CategoryItem     NotificationCategory = "item"
	CategoryCase     NotificationCategory = "case"
	CategoryTask     NotificationCategory = "task"
	CategoryPost     NotificationCategory = "post"
	CategoryMarket   NotificationCategory = "market"
)

// notificationCategories is the whole vocabulary, in the order the object
// file lists it.
var notificationCategories = []NotificationCategory{
	CategorySurvey, CategoryResults, CategoryContribution,
	CategoryMerchandise, CategoryChat, CategoryMembership, CategorySecurity,
	CategoryApproval, CategoryItem, CategoryCase, CategoryTask, CategoryPost,
	CategoryMarket,
}

// NotificationCategories returns the vocabulary. A copy, so a caller
// ranging over it cannot re-point the package's own slice.
func NotificationCategories() []NotificationCategory {
	out := make([]NotificationCategory, len(notificationCategories))
	copy(out, notificationCategories)
	return out
}

// IsNotificationCategory is the one membership check that survived the
// conversion, because it decides a status code rather than a refusal:
// routes answers an unknown category 404, where the declared check in the
// store answers 400. It reads the Go constants, which since CM25 are the
// *civic* vocabulary rather than the module's — so a domain that declared
// its own category is answered 404 by this and accepted by the store. That
// is CM30, and CM21 is where it is fixed, by asking the spec.
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

func IsNotificationCategoryExempt(c NotificationCategory) bool { return notificationExempt(c) }

func NotificationSMSChannel(c NotificationCategory) bool { return c == CategorySecurity }

// NotificationEvent is the specific act a row is the by-product of. The
// title and body are rendered from this and the subject, never stored as
// text.
type NotificationEvent string

const (
	EventSurveyReached                NotificationEvent = "survey_reached"
	EventSurveyReminder               NotificationEvent = "survey_reminder"
	EventSurveyNudge                  NotificationEvent = "survey_nudge"
	EventResultsPublished             NotificationEvent = "results_published"
	EventContributionMatched          NotificationEvent = "contribution_matched"
	EventContributionOrphaned         NotificationEvent = "contribution_orphaned"
	EventContributionReportRefused    NotificationEvent = "contribution_report_refused"
	EventMerchandiseRecorded          NotificationEvent = "merchandise_recorded"
	EventMessageRemoved               NotificationEvent = "message_removed"
	EventIdentityCaseOpened           NotificationEvent = "identity_case_opened"
	EventIdentityCaseClosed           NotificationEvent = "identity_case_closed"
	EventEnrollmentKeyRotated         NotificationEvent = "enrollment_key_rotated"
	EventActorTransferred             NotificationEvent = "actor_transferred"
	EventConsentVersionPublished      NotificationEvent = "consent_version_published"
	EventNumberChanged                NotificationEvent = "number_changed"
	EventDeviceSignedOut              NotificationEvent = "device_signed_out"
	EventApprovalRequested            NotificationEvent = "approval_requested"
	EventGroupJoinApproved            NotificationEvent = "group_join_approved"
	EventGroupJoinDeclined            NotificationEvent = "group_join_declined"
	EventGroupInvited                 NotificationEvent = "group_invited"
	EventJoinRequested                NotificationEvent = "join_requested"
	EventThreadReply                  NotificationEvent = "thread_reply"
	EventThreadRequested              NotificationEvent = "thread_requested"
	EventThreadDeclined               NotificationEvent = "thread_declined"
	EventRecordMessage                NotificationEvent = "record_message"
	EventRecordAccessEnded            NotificationEvent = "record_access_ended"
	EventGroupMemberRemoved           NotificationEvent = "group_member_removed"
	EventAgencyMemberRemoved          NotificationEvent = "agency_member_removed"
	EventItemAcknowledged             NotificationEvent = "item_acknowledged"
	EventItemAnswered                 NotificationEvent = "item_answered"
	EventItemStatusChanged            NotificationEvent = "item_status_changed"
	EventItemFiledForYou              NotificationEvent = "item_filed_for_you"
	EventItemsOverdue                 NotificationEvent = "items_overdue"
	EventRecordChangeApproved         NotificationEvent = "record_change_approved"
	EventRecordChangeRefused          NotificationEvent = "record_change_refused"
	EventCaseOpened                   NotificationEvent = "case_opened"
	EventCaseClosed                   NotificationEvent = "case_closed"
	EventTaskAssigned                 NotificationEvent = "task_assigned"
	EventTaskOverdue                  NotificationEvent = "task_overdue"
	EventAppointmentBooked            NotificationEvent = "appointment_booked"
	EventAppointmentCancelled         NotificationEvent = "appointment_cancelled"
	EventAppointmentCancelledByMember NotificationEvent = "appointment_cancelled_by_member"
	EventPollOpened                   NotificationEvent = "poll_opened"
	EventPollResultPublished          NotificationEvent = "poll_result_published"
	EventVolunteerPicked              NotificationEvent = "volunteer_picked"
	EventVolunteerCallFilled          NotificationEvent = "volunteer_call_filled"
	EventSupplierApproved             NotificationEvent = "supplier_approved"
	EventAskWaiting                   NotificationEvent = "ask_waiting"
	EventAskAnswered                  NotificationEvent = "ask_answered"
	EventRevisionAsked                NotificationEvent = "revision_asked"
	EventNoticePosted                 NotificationEvent = "notice_posted"
	EventQuoteSent                    NotificationEvent = "quote_sent"
	EventDraftWaiting                 NotificationEvent = "draft_waiting"
	EventCounterSent                  NotificationEvent = "counter_sent"
	EventAskClosed                    NotificationEvent = "ask_closed"
)

var notificationEventCategory = map[NotificationEvent]NotificationCategory{
	EventSurveyReached:                CategorySurvey,
	EventSurveyReminder:               CategorySurvey,
	EventSurveyNudge:                  CategorySurvey,
	EventResultsPublished:             CategoryResults,
	EventContributionMatched:          CategoryContribution,
	EventContributionOrphaned:         CategoryContribution,
	EventContributionReportRefused:    CategoryContribution,
	EventMerchandiseRecorded:          CategoryMerchandise,
	EventMessageRemoved:               CategoryChat,
	EventIdentityCaseOpened:           CategorySecurity,
	EventIdentityCaseClosed:           CategorySecurity,
	EventEnrollmentKeyRotated:         CategoryMembership,
	EventActorTransferred:             CategoryMembership,
	EventConsentVersionPublished:      CategoryMembership,
	EventNumberChanged:                CategoryMembership,
	EventDeviceSignedOut:              CategorySecurity,
	EventApprovalRequested:            CategoryApproval,
	EventGroupJoinApproved:            CategoryMembership,
	EventGroupJoinDeclined:            CategoryMembership,
	EventGroupInvited:                 CategoryMembership,
	EventJoinRequested:                CategoryMembership,
	EventThreadReply:                  CategoryChat,
	EventThreadRequested:              CategoryChat,
	EventThreadDeclined:               CategoryChat,
	EventRecordMessage:                CategoryChat,
	EventRecordAccessEnded:            CategoryChat,
	EventGroupMemberRemoved:           CategoryMembership,
	EventAgencyMemberRemoved:          CategoryMembership,
	EventItemAcknowledged:             CategoryItem,
	EventItemAnswered:                 CategoryItem,
	EventItemStatusChanged:            CategoryItem,
	EventItemFiledForYou:              CategoryItem,
	EventItemsOverdue:                 CategoryItem,
	EventRecordChangeApproved:         CategoryMembership,
	EventRecordChangeRefused:          CategoryMembership,
	EventCaseOpened:                   CategoryCase,
	EventCaseClosed:                   CategoryCase,
	EventTaskAssigned:                 CategoryTask,
	EventTaskOverdue:                  CategoryTask,
	EventAppointmentBooked:            CategoryCase,
	EventAppointmentCancelled:         CategoryCase,
	EventAppointmentCancelledByMember: CategoryCase,
	EventPollOpened:                   CategorySurvey,
	EventPollResultPublished:          CategoryResults,
	EventVolunteerPicked:              CategoryPost,
	EventVolunteerCallFilled:          CategoryPost,
	EventSupplierApproved:             CategoryMarket,
	EventAskWaiting:                   CategoryMarket,
	EventAskAnswered:                  CategoryMarket,
	EventRevisionAsked:                CategoryMarket,
	EventNoticePosted:                 CategoryMarket,
	EventQuoteSent:                    CategoryMarket,
	EventDraftWaiting:                 CategoryMarket,
	EventCounterSent:                  CategoryMarket,
	EventAskClosed:                    CategoryMarket,
}

var notificationEventUnmutable = map[NotificationEvent]bool{
	EventGroupMemberRemoved:   true,
	EventAgencyMemberRemoved:  true,
	EventRecordAccessEnded:    true,
	EventVolunteerPicked:      true,
	EventRecordChangeApproved: true,
	EventRecordChangeRefused:  true,
	EventCaseClosed:           true,
}

func NotificationEventUnmutable(e NotificationEvent) bool { return notificationEventUnmutable[e] }

func NotificationCategoryOf(e NotificationEvent) (NotificationCategory, bool) {
	c, ok := notificationEventCategory[e]
	return c, ok
}

// NotificationEvents returns the whole vocabulary of events. Exported so the
// blueprint's declared `values` and these constants can be held to each other
// in both directions — a stored value outlives a rename, which makes a drifted
// constant a data bug rather than a compile error.
func NotificationEvents() []NotificationEvent {
	out := make([]NotificationEvent, 0, len(notificationEventCategory))
	for e := range notificationEventCategory {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
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
	SubjectActorRegistration  NotificationSubjectKind = "actor_registration"
	SubjectPendingApproval    NotificationSubjectKind = "pending_approval"
	SubjectGroup              NotificationSubjectKind = "group"
	SubjectSubjectThread      NotificationSubjectKind = "subject_thread"
	SubjectChatThread         NotificationSubjectKind = "chat_thread"
	SubjectGroupJoinRequest   NotificationSubjectKind = "group_join_request"
	SubjectGroupJoinDecision  NotificationSubjectKind = "group_join_decision"
	SubjectItem               NotificationSubjectKind = "item"
	SubjectItemDigest         NotificationSubjectKind = "item_digest"
	SubjectRecordChange       NotificationSubjectKind = "record_change"
	SubjectCase               NotificationSubjectKind = "case"
	SubjectTask               NotificationSubjectKind = "task"
	SubjectRecord             NotificationSubjectKind = "record"
	SubjectPoll               NotificationSubjectKind = "poll"
	SubjectPost               NotificationSubjectKind = "post"
	SubjectPoster             NotificationSubjectKind = "poster"
	SubjectAsk                NotificationSubjectKind = "ask"
	SubjectGroupOrder         NotificationSubjectKind = "group_order"
	SubjectNotice             NotificationSubjectKind = "notice"
	SubjectQuote              NotificationSubjectKind = "quote"
	SubjectDraft              NotificationSubjectKind = "draft"
)

var notificationSubjectKinds = []NotificationSubjectKind{
	SubjectSurvey, SubjectContribution, SubjectContributionReport,
	SubjectHandout, SubjectMessage, SubjectIdentityCase, SubjectDevice,
	SubjectEnrollmentKey, SubjectActorRegistration, SubjectPendingApproval,
	SubjectGroup, SubjectSubjectThread, SubjectChatThread,
	SubjectGroupJoinRequest, SubjectGroupJoinDecision,
	SubjectItem, SubjectItemDigest, SubjectRecordChange,
	SubjectCase, SubjectTask, SubjectRecord, SubjectPoll, SubjectPost,
	SubjectPoster, SubjectAsk, SubjectGroupOrder, SubjectNotice, SubjectQuote, SubjectDraft,
}

// NotificationSubjectKinds returns the whole vocabulary of subject kinds, for
// the same reason NotificationEvents does.
func NotificationSubjectKinds() []NotificationSubjectKind {
	out := make([]NotificationSubjectKind, len(notificationSubjectKinds))
	copy(out, notificationSubjectKinds)
	return out
}
