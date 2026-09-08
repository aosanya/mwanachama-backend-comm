package mwanachamacomm

import "github.com/aosanya/mwanachama-backend-comm/models"

type (
	Address               = models.Address
	AddressSettings       = models.AddressSettings
	AddressBlock          = models.AddressBlock
	AddressMine           = models.AddressMine
	AddressListing        = models.AddressListing
	AddressDirectoryQuery = models.AddressDirectoryQuery
	AddressHours          = models.AddressHours
	AddressRepository     = models.AddressRepository
	AddressDirectory      = models.AddressDirectory
)

// AddressModeClosed is the "stops new threads, leaves old ones alone" flavour
// shared by an address's expiry, its disable switch and its hours. See
// [models.AddressModeClosed].
const AddressModeClosed = models.AddressModeClosed

// ErrAddressNotFound and ErrAddressBadSettings are aliases of their models.
// counterparts. See [models.ErrAddressNotFound], [models.ErrAddressBadSettings].
var (
	ErrAddressNotFound    = models.ErrAddressNotFound
	ErrAddressBadSettings = models.ErrAddressBadSettings
)

// AddressDerive, AddressNormalize and AddressFormat forward to the
// models. functions of the same name.
func AddressDerive(publicKey string) (string, error) { return models.AddressDerive(publicKey) }

// AddressNormalize forwards to [models.AddressNormalize].
func AddressNormalize(s string) (string, error) { return models.AddressNormalize(s) }

// AddressFormat forwards to [models.AddressFormat].
func AddressFormat(s string) string { return models.AddressFormat(s) }

// ChatMessage, ChatActivityQuery, ChatActivityPage and ChatActivityReader are
// aliases of their models. counterparts, and ChatRepository is the
// persistence boundary over ChatMessage and ChatThread.
type (
	ChatMessage        = models.ChatMessage
	ChatActivityQuery  = models.ChatActivityQuery
	ChatActivityPage   = models.ChatActivityPage
	ChatActivityReader = models.ChatActivityReader
	ChatRepository     = models.ChatRepository
)

// ErrChatNotFound is an alias of [models.ErrChatNotFound].
var ErrChatNotFound = models.ErrChatNotFound

// DMThread, DMMessage, DMParticipant, DMReaction and DMDeviceKey are the
// direct/group-message domain's shapes; DMParticipantState is a
// participant's lifecycle position; DMRepository is the persistence
// boundary over all of them. Aliases of their models. counterparts.
type (
	DMThread           = models.DMThread
	DMMessage          = models.DMMessage
	DMParticipant      = models.DMParticipant
	DMReaction         = models.DMReaction
	DMDeviceKey        = models.DMDeviceKey
	DMParticipantState = models.DMParticipantState
	DMRepository       = models.DMRepository
)

// The four DMParticipantState values. See [models.DMStateActive] et al.
const (
	DMStateInvited DMParticipantState = models.DMStateInvited
	DMStateActive  DMParticipantState = models.DMStateActive
	DMStateLeft    DMParticipantState = models.DMStateLeft
	DMStateKicked  DMParticipantState = models.DMStateKicked
)

// ErrDMNotFound, ErrDMAlreadyActive and ErrDMNotLastToLeave are aliases of
// their models. counterparts.
var (
	ErrDMNotFound       = models.ErrDMNotFound
	ErrDMAlreadyActive  = models.ErrDMAlreadyActive
	ErrDMNotLastToLeave = models.ErrDMNotLastToLeave
)

type (
	Report               = models.Report
	Removal              = models.Removal
	Dismissal            = models.Dismissal
	Dispute              = models.Dispute
	Actor                = models.Actor
	ReportReason         = models.ReportReason
	RemovalReason        = models.RemovalReason
	DisputeState         = models.DisputeState
	ModerationRepository = models.ModerationRepository
	ActKind              = models.ActKind
	ActEntry             = models.ActEntry
	ActWriter            = models.ActWriter
)

// The moderation vocabulary values a caller outside this module actually
// names. See [models.ReportReasonAbuse] et al.
const (
	ReportReasonAbuse      = models.ReportReasonAbuse
	RemovalReasonAbuse     = models.RemovalReasonAbuse
	DisputeReinstated      = models.DisputeReinstated
	DisputeUpheld          = models.DisputeUpheld
	ActPostWithheld        = models.ActPostWithheld
	ActPostRestored        = models.ActPostRestored
	ActRemovalLeftStanding = models.ActRemovalLeftStanding
	ActReportLeftStanding  = models.ActReportLeftStanding
)

// ErrModerationNotFound, ErrAlreadyDecided, ErrReviewerIsRemover and
// ErrAlreadyRemoved are aliases of their models. counterparts.
var (
	ErrModerationNotFound = models.ErrModerationNotFound
	ErrAlreadyDecided     = models.ErrAlreadyDecided
	ErrReviewerIsRemover  = models.ErrReviewerIsRemover
	ErrAlreadyRemoved     = models.ErrAlreadyRemoved
)

// Notification and NotificationPreference are the notification domain's row
// shapes; NotificationCategory is the opt-out unit; NotificationRepository
// is the persistence boundary over both rows. Aliases of their models.
// counterparts.
type (
	Notification           = models.Notification
	NotificationPreference = models.NotificationPreference
	NotificationCategory   = models.NotificationCategory
	NotificationRepository = models.NotificationRepository
)

// The notification vocabulary values and limits a caller outside this
// module actually names. See [models.CategorySurvey] et al.
const (
	CategorySurvey          = models.CategorySurvey
	CategoryResults         = models.CategoryResults
	CategorySecurity        = models.CategorySecurity
	CategoryChat            = models.CategoryChat
	EventSurveyReminder     = models.EventSurveyReminder
	EventSurveyNudge        = models.EventSurveyNudge
	EventResultsPublished   = models.EventResultsPublished
	SubjectSurvey           = models.SubjectSurvey
	NotificationDefaultPage = models.NotificationDefaultPage
	NotificationMaxMarkRead = models.NotificationMaxMarkRead
)

// ErrNotificationNotFound, ErrNotificationInvalid, ErrNotificationCapSpent,
// ErrNotificationMuted and ErrNotificationCategoryExempt are aliases of
// their models. counterparts.
var (
	ErrNotificationNotFound       = models.ErrNotificationNotFound
	ErrNotificationInvalid        = models.ErrNotificationInvalid
	ErrNotificationCapSpent       = models.ErrNotificationCapSpent
	ErrNotificationMuted          = models.ErrNotificationMuted
	ErrNotificationCategoryExempt = models.ErrNotificationCategoryExempt
)

// NotificationCategories, IsNotificationCategory, IsNotificationCategoryExempt
// and NotificationSMSChannel forward to the models. functions of the same
// name.
func NotificationCategories() []NotificationCategory { return models.NotificationCategories() }

// IsNotificationCategory forwards to [models.IsNotificationCategory].
func IsNotificationCategory(c NotificationCategory) bool { return models.IsNotificationCategory(c) }

// IsNotificationCategoryExempt forwards to [models.IsNotificationCategoryExempt].
func IsNotificationCategoryExempt(c NotificationCategory) bool {
	return models.IsNotificationCategoryExempt(c)
}

// NotificationSMSChannel forwards to [models.NotificationSMSChannel].
func NotificationSMSChannel(c NotificationCategory) bool { return models.NotificationSMSChannel(c) }
