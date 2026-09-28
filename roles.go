package mwanachamacomm

const (
	roleChatThread             = "chat_thread"
	roleChatMessage            = "chat_message"
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
)

var allRoles = []string{
	roleChatThread,
	roleChatMessage,
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
}
