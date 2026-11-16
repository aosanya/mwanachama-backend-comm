package models

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrDMNotFound = errors.New("directmessage: not found")

var ErrDMAlreadyActive = errors.New("directmessage: already an active participant")

var ErrDMSelfKick = errors.New("directmessage: cannot kick yourself")

var ErrDMNotAdmin = errors.New("directmessage: not admin")

var ErrDMInvalidReaction = errors.New("directmessage: invalid reaction")

var ErrDMLastAdmin = errors.New("directmessage: the last admin cannot leave while others remain; promote a successor first")

var ErrDMNotLastToLeave = errors.New("directmessage: only the actor who was last to leave can bring this thread back")

type DMParticipantState string

const (
	DMStateInvited DMParticipantState = "invited"
	DMStateActive  DMParticipantState = "active"
	DMStateLeft    DMParticipantState = "left"
	DMStateKicked  DMParticipantState = "kicked"
)

type DMThread struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`

	MessageTTLSeconds *int `json:"message_ttl_seconds,omitempty"`

	OpenedViaAddressHash []byte `json:"-"`

	OpenedViaAddress bool `json:"opened_via_address" spec:"-"`

	OpenedViaAddressOwner string `json:"-"`

	SentFromAddressOwner string `json:"-"`
	SentFromAddressIndex *int   `json:"-"`

	SentFromAddressSealed json.RawMessage `json:"sent_from_address_sealed,omitempty"`

	MyAddressIndex *int `json:"my_address_index,omitempty" spec:"-"`

	OpenedViaAddressIndex *int `json:"-"`
}

func (t DMThread) WithDerived() DMThread {
	t.OpenedViaAddress = len(t.OpenedViaAddressHash) > 0
	return t
}

type DMParticipant struct {
	ThreadID  string             `json:"thread_id"`
	ActorID   string             `json:"actor_id"`
	State     DMParticipantState `json:"state"`
	IsAdmin   bool               `json:"is_admin"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type DMMessage struct {
	ID                string            `json:"id"`
	ThreadID          string            `json:"thread_id"`
	SenderID          string            `json:"sender_id"`
	SenderDeviceKeyID string            `json:"sender_device_key_id"`
	PayloadCiphertext string            `json:"payload_ciphertext"`
	PerRecipientKeys  map[string]string `json:"per_recipient_keys"`
	CreatedAt         time.Time         `json:"created_at"`
	WithdrawnAt       string            `json:"withdrawn_at,omitempty"`
}

type DMReaction struct {
	MessageID string    `json:"message_id"`
	ActorID   string    `json:"actor_id"`
	Emoji     string    `json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
}

type DMDeviceKey struct {
	ActorID   string    `json:"actor_id"`
	KeyID     string    `json:"key_id"`
	PublicKey string    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`

	PublishedBy string `json:"published_by,omitempty"`

	DeviceID string `json:"device_id,omitempty"`

	RetiredAt *time.Time `json:"retired_at,omitempty"`
}

type DMRepository interface {
	CreateThread(ctx context.Context, t DMThread, initialActors []string) (DMThread, error)
	GetThread(ctx context.Context, id string) (DMThread, error)
	ListThreadsFor(ctx context.Context, actorID string) ([]DMThread, error)

	Invite(ctx context.Context, threadID, actorID string, byActorID string) (DMParticipant, error)
	Accept(ctx context.Context, threadID, actorID string) (DMParticipant, error)
	Leave(ctx context.Context, threadID, actorID string) error
	Kick(ctx context.Context, threadID, actorID string, byActorID string) error
	Promote(ctx context.Context, threadID, actorID string, byActorID string) (DMParticipant, error)
	ReEnable(ctx context.Context, threadID, actorID string, byActorID string) (DMParticipant, error)
	ListParticipants(ctx context.Context, threadID string) ([]DMParticipant, error)

	Post(ctx context.Context, m DMMessage) (DMMessage, error)
	ListMessages(ctx context.Context, threadID string) ([]DMMessage, error)

	GetMessage(ctx context.Context, id string) (DMMessage, error)

	SetReaction(ctx context.Context, r DMReaction) error
	ClearReaction(ctx context.Context, messageID, actorID string) error

	ListReactions(ctx context.Context, threadID string) ([]DMReaction, error)

	PublishDeviceKey(ctx context.Context, k DMDeviceKey) (DMDeviceKey, error)
	LookupDeviceKeys(ctx context.Context, actorIDs []string) ([]DMDeviceKey, error)

	RetireDeviceKeysForDevice(ctx context.Context, deviceID string, at time.Time) (int, error)
}
