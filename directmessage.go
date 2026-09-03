package mwanachamacomm

// Models N-member direct/group threads: the participant roster
// (invite/accept/leave/kick/promote/re-enable), the ciphertext messages,
// device-key lookups for client-side encryption, and a realtime inbox feed.
//
// The gateway stores ciphertext + key ids only; it never decrypts. Encryption
// stays entirely client-side.
//
// "Group chat" is not a separate concept here: a DMThread with more than one
// recipient (or an explicit title) IS a group — see dm_handlers.go's
// createDMThread in the gateway for the branch point.

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrDMNotFound is returned when a thread, participant, or message has no
// record.
var ErrDMNotFound = errors.New("directmessage: not found")

// ErrDMAlreadyActive refuses an invite aimed at somebody who has already
// accepted (BUG-20260827-006, BUG-20260827-007).
//
// **A repeat invite used to be an upsert**, so `Invite` wrote `invited` over
// whatever state the row held — and over an *active* member's acceptance,
// which the message gates then read literally: she was answered `404
// directmessage: not found` on a thread she had been talking in a moment
// before, the same answer a stranger who was never invited gets. Nothing
// removed her; an invite did, silently.
//
// Removing an active member is `Kick`, and it is a different, visible act.
// A repeat invite for somebody still *invited* stays allowed: asking again is
// a nudge, and it changes nothing.
var ErrDMAlreadyActive = errors.New("directmessage: already an active participant")

// ErrDMNotLastToLeave refuses a re-enable that is not the one act it exists
// for (BUG-20260828-001).
//
// **Re-enabling yourself is not an admin act**, and gating it on `isAdmin` —
// which requires the caller to be *currently active* — refused the only caller
// it was ever written for: `Leave` has just set that member's own row to
// `left`, so the check could never pass again. The admission that matters is
// that nobody is active on the thread and the caller is the member who left it.
var ErrDMNotLastToLeave = errors.New("directmessage: only the member who was last to leave can bring this thread back")

// DMParticipantState is the current lifecycle position of a member in a
// thread.
type DMParticipantState string

const (
	DMStateInvited DMParticipantState = "invited"
	DMStateActive  DMParticipantState = "active"
	DMStateLeft    DMParticipantState = "left"
	DMStateKicked  DMParticipantState = "kicked"
)

// DMThread is an N-member direct/group thread.
type DMThread struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`

	// OpenedViaAddressHash is the **hash** of the public address this thread
	// was opened through, when it was opened that way at all (DEV-1526). Nil
	// for a thread between chapter-mates, who needed no address to reach each
	// other.
	//
	// It is recorded because **the recipient cannot block what they cannot
	// name**. A block withdraws reachability at one address rather than from a
	// person — a member may hold many, and they are mutually unlinkable — so
	// the thread has to remember which one was used or Block has nothing to
	// act on.
	//
	// A hash rather than the address, for the same reason `member_address`
	// holds hashes: storing the address here would put back exactly what that
	// table stopped holding, one join away. The recipient blocks by this hash
	// and never needs to read it — which is why `json:"-"` is wrong for it and
	// a bare boolean is what the client actually gets (see the gateway's
	// dm_handlers.go).
	MessageTTLSeconds *int `json:"message_ttl_seconds,omitempty"`

	OpenedViaAddressHash []byte `json:"-"`

	// OpenedViaAddress reports whether an address opened this thread, without
	// saying which. It is what a client needs in order to offer **Block**, and
	// all it needs.
	OpenedViaAddress bool `json:"opened_via_address"`

	// OpenedViaAddressOwner is the member whose address was used — the person
	// who was *reached*, not the one who typed it. Never serialised: it exists
	// so the handler can decide who may be told the index below.
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

// DMParticipant is a member's row in a thread.
type DMParticipant struct {
	ThreadID  string             `json:"thread_id"`
	MemberID  string             `json:"member_id"`
	State     DMParticipantState `json:"state"`
	IsAdmin   bool               `json:"is_admin"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// DMMessage is a ciphertext post; PayloadCiphertext is opaque to the gateway
// and PerRecipientKeys[recipientMemberID] is the ciphertext of the message key
// encrypted to that recipient's device key.
type DMMessage struct {
	ID                string            `json:"id"`
	ThreadID          string            `json:"thread_id"`
	SenderID          string            `json:"sender_id"`
	SenderDeviceKeyID string            `json:"sender_device_key_id"`
	PayloadCiphertext string            `json:"payload_ciphertext"`
	PerRecipientKeys  map[string]string `json:"per_recipient_keys"`
	CreatedAt         time.Time         `json:"created_at"`
}

// DMReaction is one member's emoji on one message (DEV-1530).
//
// ⚠️ **The emoji is stored in plaintext, and the message it is on is not.**
// One member holds at most one reaction per message: reacting again replaces,
// which is what the picker's behaviour implies and what stops the row count
// growing without bound.
type DMReaction struct {
	MessageID string    `json:"message_id"`
	MemberID  string    `json:"member_id"`
	Emoji     string    `json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
}

// DMDeviceKey is a member's active public device key, published so senders
// can wrap message keys for that recipient.
type DMDeviceKey struct {
	MemberID  string    `json:"member_id"`
	KeyID     string    `json:"key_id"`
	PublicKey string    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`

	// PublishedBy is the member off the VERIFIED session that wrote this row,
	// never a value from the request body (DEV-1265). Empty means the row
	// predates the column.
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
	CreateThread(ctx context.Context, t DMThread, initialMembers []string) (DMThread, error)
	GetThread(ctx context.Context, id string) (DMThread, error)
	ListThreadsFor(ctx context.Context, memberID string) ([]DMThread, error)

	Invite(ctx context.Context, threadID, memberID string, byMemberID string) (DMParticipant, error)
	Accept(ctx context.Context, threadID, memberID string) (DMParticipant, error)
	Leave(ctx context.Context, threadID, memberID string) error
	Kick(ctx context.Context, threadID, memberID string, byMemberID string) error
	Promote(ctx context.Context, threadID, memberID string, byMemberID string) (DMParticipant, error)
	ReEnable(ctx context.Context, threadID, memberID string, byMemberID string) (DMParticipant, error)
	ListParticipants(ctx context.Context, threadID string) ([]DMParticipant, error)

	Post(ctx context.Context, m DMMessage) (DMMessage, error)
	ListMessages(ctx context.Context, threadID string) ([]DMMessage, error)

	// GetMessage returns one message by id, so a caller acting on a message
	// (reacting to it) can find the thread whose participant fence governs it.
	GetMessage(ctx context.Context, id string) (DMMessage, error)

	// SetReaction records one member's emoji on one message, replacing whatever
	// they had on it before — one member, one reaction per message.
	SetReaction(ctx context.Context, r DMReaction) error
	// ClearReaction removes a member's reaction from a message. Removing one
	// that is not there is not an error: the caller asked for a state, and it
	// already holds.
	ClearReaction(ctx context.Context, messageID, memberID string) error
	// ListReactions returns every reaction on every message of a thread, so a
	// conversation loads its reactions in one read rather than one per message.
	ListReactions(ctx context.Context, threadID string) ([]DMReaction, error)

	PublishDeviceKey(ctx context.Context, k DMDeviceKey) (DMDeviceKey, error)
	LookupDeviceKeys(ctx context.Context, memberIDs []string) ([]DMDeviceKey, error)

	// RetireDeviceKeysForDevice retires every live key published by one
	// handset and reports how many it retired (DEV-1272).
	RetireDeviceKeysForDevice(ctx context.Context, deviceID string, at time.Time) (int, error)
}
