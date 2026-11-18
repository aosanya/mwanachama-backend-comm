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

const AddressModeClosed = models.AddressModeClosed

var (
	ErrAddressNotFound    = models.ErrAddressNotFound
	ErrAddressBadSettings = models.ErrAddressBadSettings
)

func AddressDerive(publicKey string) (string, error) { return models.AddressDerive(publicKey) }

func AddressNormalize(s string) (string, error) { return models.AddressNormalize(s) }

func AddressFormat(s string) string { return models.AddressFormat(s) }

type (
	ChatMessage        = models.ChatMessage
	ChatReadMarker     = models.ChatReadMarker
	ChatArchive        = models.ChatArchive
	ChatThreadView     = models.ChatThreadView
	ChatLastMessage    = models.ChatLastMessage
	ChatActivityQuery  = models.ChatActivityQuery
	ChatActivityPage   = models.ChatActivityPage
	ChatActivityReader = models.ChatActivityReader
	ChatRepository     = models.ChatRepository
)

var ErrChatNotFound = models.ErrChatNotFound

var ErrChatInvalid = models.ErrChatInvalid

var ErrChatNotDeclinable = models.ErrChatNotDeclinable

var ErrChatThreadDeclined = models.ErrChatThreadDeclined

type (
	SubjectThread        = models.SubjectThread
	SubjectThreadView    = models.SubjectThreadView
	SubjectThreadMessage = models.SubjectThreadMessage
	SubjectAudience      = models.SubjectAudience
	SubjectStanding      = models.SubjectStanding
)

const (
	AudiencePrivate  = models.AudiencePrivate
	AudienceGroup    = models.AudienceGroup
	StandingNone     = models.StandingNone
	StandingAudience = models.StandingAudience
	StandingAnswerer = models.StandingAnswerer
)

var (
	ErrSubjectNotFound = models.ErrSubjectNotFound
	ErrSubjectInvalid  = models.ErrSubjectInvalid
)

type (
	DMThread           = models.DMThread
	DMMessage          = models.DMMessage
	DMParticipant      = models.DMParticipant
	DMReaction         = models.DMReaction
	DMDeviceKey        = models.DMDeviceKey
	DMParticipantState = models.DMParticipantState
	DMRepository       = models.DMRepository
)

const (
	DMStateInvited DMParticipantState = models.DMStateInvited
	DMStateActive  DMParticipantState = models.DMStateActive
	DMStateLeft    DMParticipantState = models.DMStateLeft
	DMStateKicked  DMParticipantState = models.DMStateKicked
)

var (
	ErrDMNotFound       = models.ErrDMNotFound
	ErrDMAlreadyActive  = models.ErrDMAlreadyActive
	ErrDMNotLastToLeave = models.ErrDMNotLastToLeave
	ErrDMSelfKick       = models.ErrDMSelfKick
	ErrDMLastAdmin      = models.ErrDMLastAdmin

	ErrDMNotAdmin = models.ErrDMNotAdmin

	ErrDMInvalidReaction = models.ErrDMInvalidReaction
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

var (
	ErrModerationNotFound = models.ErrModerationNotFound
	ErrAlreadyDecided     = models.ErrAlreadyDecided
	ErrReviewerIsRemover  = models.ErrReviewerIsRemover
	ErrAlreadyRemoved     = models.ErrAlreadyRemoved
)

type (
	Notification                = models.Notification
	NotificationPreference      = models.NotificationPreference
	NotificationAcknowledgement = models.NotificationAcknowledgement
	NotificationReceipt         = models.NotificationReceipt
	NotificationCategory        = models.NotificationCategory
	NotificationRepository      = models.NotificationRepository
)

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

var (
	ErrNotificationNotFound       = models.ErrNotificationNotFound
	ErrNotificationInvalid        = models.ErrNotificationInvalid
	ErrNotificationCapSpent       = models.ErrNotificationCapSpent
	ErrNotificationNoSuchCategory = models.ErrNotificationNoSuchCategory
	ErrNotificationMuted          = models.ErrNotificationMuted
	ErrNotificationNotRaisable    = models.ErrNotificationNotRaisable
	ErrNotificationCategoryExempt = models.ErrNotificationCategoryExempt
)

func NotificationCategories() []NotificationCategory { return models.NotificationCategories() }

func IsNotificationCategory(c NotificationCategory) bool { return models.IsNotificationCategory(c) }

func IsNotificationCategoryExempt(c NotificationCategory) bool {
	return models.IsNotificationCategoryExempt(c)
}

func NotificationSMSChannel(c NotificationCategory) bool { return models.NotificationSMSChannel(c) }

func NotificationVoiceChannel(c NotificationCategory) bool { return models.NotificationVoiceChannel(c) }

type NotificationChannel = models.NotificationChannel

func NotificationChannels(c NotificationCategory) []NotificationChannel {
	return models.NotificationChannels(c)
}

type (
	PushSubscription = models.PushSubscription
	PushKeys         = models.PushKeys
)

var (
	ErrPushSubscriptionInvalid = models.ErrPushSubscriptionInvalid
	ErrPushSubscriptionGone    = models.ErrPushSubscriptionGone
)

type StandingMessage = models.StandingMessage

var (
	ErrStandingInvalid  = models.ErrStandingInvalid
	ErrStandingNotFound = models.ErrStandingNotFound
	ErrStandingStopped  = models.ErrStandingStopped
)
