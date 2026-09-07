// Package mwanachamacomm models chapter chat rooms, N-member direct/group
// threads, and message moderation for mwanachama-backend-api-gateway,
// storing them via GORM.
//
// Layout, mirroring mwanachama-backend-actor's split:
//   - models/    — domain types (ChatThread, DMThread, Report, ...) and the
//     repository interfaces (ChatRepository, DMRepository,
//     ModerationRepository) they're read and written through
//   - gormstore/ — GORM row structs, row<->domain conversion, migration
//   - doc.go (this file), tables.go — table-name/migrate wrappers; this file
//     also aliases every models/ identifier a caller outside this module
//     actually names, mirroring mwanachama-backend-shared's orgsettings/
//     orgpolicy convention, so a caller needs only this package's import,
//     never models's directly — see the alias blocks below
//   - errors.go            — ErrInvalidReference/ErrConflict + classify()
//   - ids.go                — Clock, the one storage-agnostic helper left
//   - chat_impl.go, dm_impl.go, dm_message_impl.go, moderation_impl.go,
//     moderation_dismissal_impl.go — the GORM-backed store implementations
//   - routes/    — this package's own HTTP surface for the operations that
//     carry no gateway-only policy; see routes/doc.go for scope
//
// Ported from mwanachama-backend-api-gateway's internal/domain/{chat,
// directmessage,moderation} and internal/store/{postgres,memory}. See this
// repo's CLAUDE.md for what changed along the way, in particular why
// moderation's act-log write still doesn't import the gateway's custody
// package (unaffected by the storage swap this package went through
// 2026-09-04).
package mwanachamacomm

import "github.com/aosanya/mwanachama-backend-comm/models"

// Address is one published address of one member, and AddressSettings,
// AddressBlock, AddressMine, AddressListing, AddressDirectoryQuery are its
// surrounding shapes; AddressRepository and AddressDirectory are the
// persistence boundaries over them. Aliases of their models. counterparts —
// see [models.Address] et al.
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

// Report, Removal, Dismissal and Dispute are moderation's own shapes; Actor
// is who performed an act; ReportReason, RemovalReason and DisputeState are
// their closed vocabularies; ModerationRepository is the persistence
// boundary over all four. ActKind, ActEntry and ActWriter are the chapter
// act-log seam (see models/moderation_act.go). Aliases of their models.
// counterparts.
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
