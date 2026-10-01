package models

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrAddressNotFound is returned when no live address matches.
var ErrAddressNotFound = errors.New("address: not found")

// ErrAddressMalformed is returned when a string is not a well-formed
// address.
var ErrAddressMalformed = errors.New("address: malformed")

// ErrAddressBadSettings is returned when a settings change does not
// describe a state an address can be in.
var ErrAddressBadSettings = errors.New("address: settings are not well formed")

type Address struct {
	ActorID string `json:"actor_id"`

	Hash []byte `json:"-"`

	SaltID int `json:"-"`

	AddressIndex int `json:"index"`

	CreatedAt time.Time `json:"created_at"`

	RetiredAt *time.Time `json:"retired_at,omitempty"`

	ExpiresAt         *time.Time    `json:"expires_at,omitempty"`
	ExpiryMode        string        `json:"expiry_mode,omitempty"`
	MessageTTLSeconds *int          `json:"message_ttl_seconds,omitempty"`
	PublicAddress     string        `json:"address,omitempty"`
	ListedAt          *time.Time    `json:"listed_at,omitempty"`
	DisabledAt        *time.Time    `json:"disabled_at,omitempty"`
	DisabledMode      string        `json:"disabled_mode,omitempty"`
	Hours             *AddressHours `json:"hours,omitempty"`
}

func (a Address) Settings() AddressSettings {
	return AddressSettings{
		ExpiresAt:         a.ExpiresAt,
		ExpiryMode:        a.ExpiryMode,
		MessageTTLSeconds: a.MessageTTLSeconds,
		PublicAddress:     a.PublicAddress,
		ListedAt:          a.ListedAt,
		DisabledAt:        a.DisabledAt,
		DisabledMode:      a.DisabledMode,
		Hours:             a.Hours,
	}
}

func (a Address) WithSettings(s AddressSettings) Address {
	a.ExpiresAt = s.ExpiresAt
	a.ExpiryMode = s.ExpiryMode
	a.MessageTTLSeconds = s.MessageTTLSeconds
	a.PublicAddress = s.PublicAddress
	a.ListedAt = s.ListedAt
	a.DisabledAt = s.DisabledAt
	a.DisabledMode = s.DisabledMode
	a.Hours = s.Hours
	return a
}

// AddressSettings is the mutable half of an address, as one value so a
// settings write is one call. Every field's prose lives in the blueprint's
// description for the column it lands in.
type AddressSettings struct {
	ExpiresAt         *time.Time    `json:"expires_at,omitempty"`
	ExpiryMode        string        `json:"expiry_mode,omitempty"`
	MessageTTLSeconds *int          `json:"message_ttl_seconds,omitempty"`
	PublicAddress     string        `json:"address,omitempty"`
	ListedAt          *time.Time    `json:"listed_at,omitempty"`
	DisabledAt        *time.Time    `json:"disabled_at,omitempty"`
	DisabledMode      string        `json:"disabled_mode,omitempty"`
	Hours             *AddressHours `json:"hours,omitempty"`
}

// The two flavours of every stop an address can be under: closed stops new
// connections, silent stops new communication. Both are declared values of
// expiry_mode and disabled_mode; which conversations go quiet is Silenced.
const (
	AddressModeClosed = "closed"
	AddressModeSilent = "silent"
)

func (a Address) Retired() bool { return a.RetiredAt != nil }

func (a Address) Expired(now time.Time) bool {
	return a.ExpiresAt != nil && !now.Before(*a.ExpiresAt)
}

// Live takes the time rather than reading the clock, so the one predicate
// every caller depends on cannot differ between a handler that passed
// time.Now() and a store that ran now() inside SQL.
func (a Address) Live(now time.Time) bool { return !a.Retired() && !a.Expired(now) }

func (a Address) Disabled() bool { return a.DisabledAt != nil }

func (a Address) WithinHours(now time.Time) bool {
	return a.Hours == nil || a.Hours.OpenAt(now)
}

// Open reports whether this address accepts a new conversation at now.
//
// **The one predicate every route asks.** Live() answers the permanent
// half — retired, or a clock that ran out — and the two temporary halves
// are asked here, so a caller cannot honour one and forget the other. Both
// flavours of every stop close the door to new threads; the flavour only
// decides whether the conversations already running go quiet, which is
// Silenced.
func (a Address) Open(now time.Time) bool {
	return a.Live(now) && !a.Disabled() && a.WithinHours(now)
}

// Unavailable reports the state a caller may need to tell apart from
// "gone": an address that is neither retired nor past its clock, but is
// shut right now — switched off, or outside its hours.
//
// It exists so a route can tell *temporarily shut* from *gone*, which are
// the two answers a person holding the address needs to be able to act on.
// It is deliberately false for a retired or expired address: those stay
// indistinguishable from never having existed, so probing the space still
// cannot take a census.
func (a Address) Unavailable(now time.Time) bool {
	return a.Live(now) && !a.Open(now)
}

// Silenced reports whether conversations already opened through this
// address go quiet at now.
//
// The three ways an address can stop each carry their own flavour, and
// only AddressModeSilent reaches a running conversation. AddressModeClosed
// stops new threads and leaves the old ones alone, which is what retiring
// does and what most people mean by closing an address.
func (a Address) Silenced(now time.Time) bool {
	if a.Expired(now) && a.ExpiryMode == AddressModeSilent {
		return true
	}
	if a.Disabled() && a.DisabledMode == AddressModeSilent {
		return true
	}
	return a.Hours != nil && !a.Hours.OpenAt(now) && a.Hours.Mode == AddressModeSilent
}

// Validate reports whether s describes a state an address can actually be
// in. Mirrors the schema's own CHECK constraints rather than trusting
// them: a constraint violation surfaces as a driver error at the bottom of
// a stack, and the caller deserves to be told which field it was.
// Validate holds the rules a declaration cannot state: three pairs that are
// both halves or neither, a count that must be positive rather than merely
// present, and the nested schedule's own shape. The two mode vocabularies
// and the public address's canonical form are declared, and are checked by
// the module's own Check against the spec.
func (s AddressSettings) Validate() error {
	if (s.ExpiresAt == nil) != (s.ExpiryMode == "") {
		return fmt.Errorf("%w: an expiry needs a flavour and a flavour needs an expiry", ErrAddressBadSettings)
	}
	if s.MessageTTLSeconds != nil && *s.MessageTTLSeconds <= 0 {
		return fmt.Errorf("%w: a timer of zero seconds is off, which is null", ErrAddressBadSettings)
	}
	if s.ListedAt != nil && s.PublicAddress == "" {
		return fmt.Errorf("%w: an address cannot be listed in the directory unless it is public", ErrAddressBadSettings)
	}
	if (s.DisabledAt == nil) != (s.DisabledMode == "") {
		return fmt.Errorf("%w: switching an address off needs a flavour and a flavour needs the switch", ErrAddressBadSettings)
	}
	if s.Hours != nil {
		if err := s.Hours.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type AddressPublic struct {
	Address string `json:"address"`
	ActorID string `json:"actor_id"`
}

// Public projects the row. addr is the address the caller supplied — the
// row itself does not carry one, because the database does not hold it.
func (a Address) Public(addr string) AddressPublic {
	return AddressPublic{Address: addr, ActorID: a.ActorID}
}

type AddressMine struct {
	Index     int        `json:"address_index"`
	CreatedAt time.Time  `json:"created_at"`
	RetiredAt *time.Time `json:"retired_at,omitempty"`

	// The owner's own settings come back beside the state, because the
	// sheet that sets them has to be able to draw what is currently set.
	AddressSettings
}

// Mine projects a row for its owner.
//
// **The `address` field is populated for a public row and empty for every
// other one**, and that asymmetry is the whole point on this side: the
// gateway hands back an address exactly when its owner told it to hold
// one. For a private row it has nothing to hand back — not by policy, but
// because it genuinely does not know.
func (a Address) Mine() AddressMine {
	return AddressMine{
		Index:           a.AddressIndex,
		CreatedAt:       a.CreatedAt,
		RetiredAt:       a.RetiredAt,
		AddressSettings: a.Settings(),
	}
}

// IsPublic reports whether the owner published this address's plaintext.
func (a Address) IsPublic() bool { return a.PublicAddress != "" }

type AddressBlock struct {
	ActorID   string    `json:"-"`
	Hash      []byte    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type AddressRepository interface {
	Publish(ctx context.Context, a Address) (Address, error)

	ListFor(ctx context.Context, actorID string) ([]Address, error)

	// Resolve finds the live address by its hash. A retired or unknown
	// one is ErrAddressNotFound, and deliberately the same answer: a
	// caller probing the space learns nothing from the difference.
	Resolve(ctx context.Context, hash []byte) (Address, error)

	Retire(ctx context.Context, actorID string, index int, at time.Time) error

	CountPublic(ctx context.Context, actorID string) (int, error)

	UpdateSettings(ctx context.Context, actorID string, index int, s AddressSettings) (Address, error)

	Block(ctx context.Context, b AddressBlock) error

	IsBlocked(ctx context.Context, actorID string, hash []byte) (bool, error)
}
