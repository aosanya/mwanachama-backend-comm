package mwanachamacomm

const (
	roleChatThread             = "chat_thread"
	roleChatMessage            = "chat_message"
	roleChatReadMarker         = "chat_read_marker"
	roleChatArchive            = "chat_archive"
	roleSubjectThread          = "subject_thread"
	roleSubjectMessage         = "subject_message"
	roleDMThread               = "dm_thread"
	roleDMParticipant          = "dm_participant"
	roleDMMessage              = "dm_message"
	roleDMReaction             = "dm_reaction"
	roleDMDeviceKey            = "dm_device_key"
	roleReport                 = "report"
	roleRemoval                = "removal"
	roleDismissal              = "dismissal"
	roleDispute                = "dispute"
	roleAddress                = "address"
	roleAddressBlock           = "address_block"
	roleNotification           = "notification"
	roleNotificationPreference = "notification_preference"
	roleNotificationAck        = "notification_acknowledgement"
	rolePushSubscription       = "push_subscription"
	rolePushDelivery           = "push_delivery"
	roleStandingMessage        = "standing_message"
)

var allRoles = []string{
	roleChatThread,
	roleChatMessage,
	roleChatReadMarker,
	roleChatArchive,
	roleSubjectThread,
	roleSubjectMessage,
	roleDMThread,
	roleDMParticipant,
	roleDMMessage,
	roleDMReaction,
	roleDMDeviceKey,
	roleReport,
	roleRemoval,
	roleDismissal,
	roleDispute,
	roleAddress,
	roleAddressBlock,
	roleNotification,
	roleNotificationPreference,
	roleNotificationAck,
	rolePushSubscription,
	rolePushDelivery,
	roleStandingMessage,
}
