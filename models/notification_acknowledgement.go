package models

import "time"

const (
	NotificationReceiptsDefaultPage = 200
	NotificationReceiptsMaxPage     = 1000
)

type NotificationAcknowledgement struct {
	ID             string    `json:"id"`
	NotificationID string    `json:"notification_id"`
	ActorID        string    `json:"actor_id"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
}

type NotificationReceipt struct {
	NotificationID string            `json:"notification_id"`
	ActorID        string            `json:"actor_id"`
	Event          NotificationEvent `json:"event"`
	CreatedAt      time.Time         `json:"created_at"`
	ReadAt         *time.Time        `json:"read_at"`
	AcknowledgedAt *time.Time        `json:"acknowledged_at"`
}
