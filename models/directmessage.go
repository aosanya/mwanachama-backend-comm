package models

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrDMNotFound is returned when a thread, participant, or message has no
// record.
var ErrDMNotFound = errors.New("directmessage: not found")

var ErrDMAlreadyActive = errors.New("directmessage: already an active participant")

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

	// OpenedViaAddress reports whether an address opened this thread, without
	// saying which. It is what a client needs in order to offer **Block**, and
	// all it needs.
	OpenedViaAddress bool `json:"opened_via_address"`

	OpenedViaAddressOwner string `json:"-"`

	// SentFromAddressOwner / SentFromAddressIndex are the sender's side of the
	// pair: which of *their own* addresses they chose to write from (DEV-1539).
	SentFromAddressOwner string `json:"-"`
	SentFromAddressIndex *int   `json:"-"`

	// SentFromAddressSealed is the sender's own address, encrypted to the
	// recipient's devices (DEV-1561). Opaque here: the gateway stores it and
	// serves it back, and holds no key to it.
	SentFromAddressSealed json.RawMessage `json:"sent_from_address_sealed,omitempty"`

	// MyAddressIndex is whichever of the two above belongs to **the caller**,
	// resolved per reader by `forCaller` — see there.
	MyAddressIndex *int `json:"my_address_index,omitempty"`

	// OpenedViaAddressIndex is which of the owner's addresses was used, and it
	// is **never serialised** — `MyAddressIndex` is what a reader gets, so a
	// new read path cannot leak this by forgetting to filter it.
	OpenedViaAddressIndex *int `json:"-"`
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

	// DeviceID is the handset the key belongs to, from the same session. Empty
	// is legitimate: a session minted by the phone flow carries no device.
	DeviceID string `json:"device_id,omitempty"`

	// RetiredAt marks a key that must no longer be sealed to — set when the
	// same handset publishes a different key. A lookup never returns a retired
	// key, because a sender who wraps to one produces a message that is
	// undecryptable by construction with no error anywhere.
	RetiredAt *time.Time `json:"retired_at,omitempty"`
}

// DMRepository is the persistence boundary for the direct-message domain.
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

	// GetMessage returns one message by id, so a caller acting on a message
	// (reacting to it) can find the thread whose participant fence governs it.
	GetMessage(ctx context.Context, id string) (DMMessage, error)

	SetReaction(ctx context.Context, r DMReaction) error
	ClearReaction(ctx context.Context, messageID, actorID string) error
	// ListReactions returns every reaction on every message of a thread, so a
	// conversation loads its reactions in one read rather than one per message.
	ListReactions(ctx context.Context, threadID string) ([]DMReaction, error)

	PublishDeviceKey(ctx context.Context, k DMDeviceKey) (DMDeviceKey, error)
	LookupDeviceKeys(ctx context.Context, actorIDs []string) ([]DMDeviceKey, error)

	// RetireDeviceKeysForDevice retires every live key published by one
	// handset and reports how many it retired (DEV-1272).
	RetireDeviceKeysForDevice(ctx context.Context, deviceID string, at time.Time) (int, error)
}
