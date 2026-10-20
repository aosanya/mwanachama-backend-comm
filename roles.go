package mwanachamacomm

const (
	roleChatThread             = "chat_thread"
	roleChatMessage            = "chat_message"
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
	rolePushSubscription       = "push_subscription"
)

var allRoles = []string{
	roleChatThread,
	roleChatMessage,
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
	rolePushSubscription,
}
