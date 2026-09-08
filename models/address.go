package models

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strings"
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

// AddressDomainSeparator prefixes the hash input so an address can never
// collide with a digest this project computes for some other purpose over
// the same key. Versioned: changing the derivation means changing this
// string, and every address derived under the old one keeps resolving
// because the gateway stores what it was told and re-derives with the
// version the row was written under.
const AddressDomainSeparator = "mwanachama:actor-address:v1"

const (
	addressLetters   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	addressLettersN  = 26
	addressLetterRun = 3
	addressDigitRun  = 4
)

// AddressDerive computes the canonical address for a public key. The key is
// taken as the opaque string the device published — whatever encoding that
// is, it is hashed verbatim, so the two sides cannot disagree about
// padding.
func AddressDerive(publicKey string) (string, error) {
	if strings.TrimSpace(publicKey) == "" {
		return "", ErrAddressMalformed
	}
	sum := sha256.Sum256([]byte(AddressDomainSeparator + "\x00" + publicKey))

	// 128 bits of digest reduced into a 2^54.8 space — the modulo bias is
	// far below anything observable, and taking the whole digest would not
	// change a single address anyone ever reads.
	n := new(big.Int).SetBytes(sum[:16])

	var b strings.Builder
	b.Grow(2*addressLetterRun + 2*addressDigitRun)
	emitAddressLetters(&b, n)
	emitAddressDigits(&b, n)
	emitAddressLetters(&b, n)
	emitAddressDigits(&b, n)
	return b.String(), nil
}

func emitAddressLetters(b *strings.Builder, n *big.Int) {
	m := new(big.Int)
	for i := 0; i < addressLetterRun; i++ {
		n.QuoRem(n, big.NewInt(addressLettersN), m)
		b.WriteByte(addressLetters[m.Int64()])
	}
}

func emitAddressDigits(b *strings.Builder, n *big.Int) {
	m := new(big.Int)
	for i := 0; i < addressDigitRun; i++ {
		n.QuoRem(n, big.NewInt(10), m)
		b.WriteByte(byte('0' + m.Int64()))
	}
}

// AddressNormalize accepts an address the way a person typed it — spaced,
// hyphenated, lower-case — and returns the canonical fourteen-character
// form. A person reading `MKU 4827 YUT 3391` off a screen and typing it
// back must not be told they got it wrong because of the spaces the screen
// put there.
func AddressNormalize(s string) (string, error) {
	var b strings.Builder
	b.Grow(2*addressLetterRun + 2*addressDigitRun)
	for _, r := range s {
		switch {
		case r == ' ' || r == '-' || r == '\t' || r == '_':
			continue
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 32)
		default:
			b.WriteRune(r)
		}
	}
	out := b.String()
	if !AddressValid(out) {
		return "", ErrAddressMalformed
	}
	return out, nil
}

// AddressValid reports whether s is exactly the canonical form.
func AddressValid(s string) bool {
	if len(s) != 2*addressLetterRun+2*addressDigitRun {
		return false
	}
	isAlpha := func(c byte) bool { return c >= 'A' && c <= 'Z' }
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	for i := 0; i < addressLetterRun; i++ {
		if !isAlpha(s[i]) {
			return false
		}
	}
	for i := addressLetterRun; i < addressLetterRun+addressDigitRun; i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	for i := addressLetterRun + addressDigitRun; i < 2*addressLetterRun+addressDigitRun; i++ {
		if !isAlpha(s[i]) {
			return false
		}
	}
	for i := 2*addressLetterRun + addressDigitRun; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// AddressFormat groups the canonical form for display: `MKU 4827 YUT 3391`.
func AddressFormat(s string) string {
	if !AddressValid(s) {
		return s
	}
	a, b := addressLetterRun, addressLetterRun+addressDigitRun
	c := b + addressLetterRun
	return s[:a] + " " + s[a:b] + " " + s[b:c] + " " + s[c:]
}

type Address struct {
	ActorID string `json:"actor_id"`

	// Hash is what the shared database actually holds — the keyed hash of
	// the address under the organization's salt, never the address itself.
	//
	// **The public key is deliberately not stored either.** An address is
	// a pure public function of its key, so a row holding the key would
	// hand the address to anyone who read the table — the hash would buy
	// nothing.
	//
	// Keyed rather than plain: the address space is 3.089 × 10¹⁶, about
	// 2⁵⁵ — enumerable offline by anyone who obtains an unsalted table.
	// Under the org's phone-number-salt-style HMAC a stolen database is
	// not enough; you need the salt too.
	Hash []byte `json:"-"`

	// SaltID records which salt Hash was computed under, so a rotation can
	// tell which rows still need re-hashing.
	SaltID int `json:"-"`

	Index int `json:"index"`

	CreatedAt time.Time `json:"created_at"`

	// RetiredAt set means: refuse new threads addressed here. Threads
	// already open are untouched — retiring an address must never sever a
	// conversation the other party is in the middle of.
	RetiredAt *time.Time `json:"retired_at,omitempty"`

	AddressSettings
}

type AddressSettings struct {
	// ExpiresAt is when this address stops, or nil for never.
	//
	// **A clock, not an event.** Nothing sweeps it: a live read tests it
	// against now. Because it is a predicate, moving the date moves the
	// effect — push an expired address's date forward and it works again,
	// silenced threads included, because nothing was torn down.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// ExpiryMode is what expiring does — AddressModeClosed or
	// AddressModeSilent — and is set exactly when ExpiresAt is. Empty
	// means the address is on no clock.
	ExpiryMode string `json:"expiry_mode,omitempty"`

	// MessageTTLSeconds is the disappearing-message timer, nil for off.
	//
	// **It lives here rather than on the handset because it has to.** The
	// setting belongs to the address's owner, but the thread is opened by
	// whoever was *given* the address — whose device cannot know the
	// owner's preference and must not be trusted to report it. The
	// gateway reads it off this row at open and freezes a copy onto the
	// thread.
	MessageTTLSeconds *int `json:"message_ttl_seconds,omitempty"`

	PublicAddress string `json:"address,omitempty"`

	// ListedAt is when the owner asked for this address to appear in the
	// directory, or nil for not listed.
	//
	// **A second act, not a synonym for PublicAddress.** Publishing says
	// *hold this plaintext for me*; listing says *and hand it to anyone
	// who looks*. Listing implies publishing: the directory hands back
	// the plaintext, so there is nothing to list where none is held —
	// Validate refuses the pair. The corollary runs in the other
	// direction and is deliberate: turning an address private un-lists it
	// in the same write.
	//
	// ⚠️ **What un-listing does not do**, exactly as un-publishing does
	// not: it takes the row out of the directory and out of nobody's
	// notebook. The address keeps working.
	ListedAt *time.Time `json:"listed_at,omitempty"`

	DisabledAt *time.Time `json:"disabled_at,omitempty"`

	// DisabledMode is what being switched off does — AddressModeClosed or
	// AddressModeSilent — and is set exactly when DisabledAt is.
	DisabledMode string `json:"disabled_mode,omitempty"`

	// Hours is the address's opening hours, or nil for always open. See
	// [AddressHours]: a window per weekday, in the owner's own zone,
	// evaluated on read.
	Hours *AddressHours `json:"hours,omitempty"`
}

// The two flavours of expiry: `closed` *stops new connections*, `silent`
// *stops new communication*.
const (
	// AddressModeClosed stops the address resolving, and therefore stops
	// new threads. Threads already open under it keep working, untouched
	// — identical to RetiredAt, on a clock instead of a button.
	AddressModeClosed = "closed"

	// AddressModeSilent stops new threads **and** new messages in every
	// thread opened via this address. Those go quiet, in both directions.
	//
	// **It announces itself, and a block does not.** A block is hidden on
	// purpose — telling somebody invites them back from another address.
	// A clock has no route around it, so swallowing the message without a
	// word would just be a lie.
	AddressModeSilent = "silent"
)

func (a Address) Retired() bool { return a.RetiredAt != nil }

// Expired reports whether the clock has run out at now. False for an
// address on no clock.
func (a Address) Expired(now time.Time) bool {
	return a.ExpiresAt != nil && !now.Before(*a.ExpiresAt)
}

// Live reports whether the address still accepts new threads at now.
//
// **It takes the time rather than reading the clock** so that the one
// predicate every caller depends on cannot quietly differ between a
// handler that passed time.Now() and a store that ran now() inside SQL.
// Both flavours of expiry stop resolution; only AddressModeSilent goes on
// to stop the post path, and that is a separate rule on a separate route.
func (a Address) Live(now time.Time) bool { return !a.Retired() && !a.Expired(now) }

// Disabled reports whether the owner switched this address off.
// Reversible, and deliberately not Retired: see AddressSettings.DisabledAt.
func (a Address) Disabled() bool { return a.DisabledAt != nil }

// WithinHours reports whether now falls inside this address's opening
// hours. True for an address on no schedule, which is the default and the
// common case.
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
func (s AddressSettings) Validate() error {
	if (s.ExpiresAt == nil) != (s.ExpiryMode == "") {
		return fmt.Errorf("%w: an expiry needs a flavour and a flavour needs an expiry", ErrAddressBadSettings)
	}
	if s.ExpiryMode != "" && s.ExpiryMode != AddressModeClosed && s.ExpiryMode != AddressModeSilent {
		return fmt.Errorf("%w: expiry_mode must be %q or %q", ErrAddressBadSettings, AddressModeClosed, AddressModeSilent)
	}
	if s.MessageTTLSeconds != nil && *s.MessageTTLSeconds <= 0 {
		return fmt.Errorf("%w: a timer of zero seconds is off, which is null", ErrAddressBadSettings)
	}
	if s.PublicAddress != "" && !AddressValid(s.PublicAddress) {
		return fmt.Errorf("%w: a public address must be the canonical form", ErrAddressBadSettings)
	}
	// **Listed implies public, and the caller is told which half is
	// missing.** The directory hands back the plaintext, so a row asking
	// to be listed without one is asking the gateway to publish something
	// it does not have.
	if s.ListedAt != nil && s.PublicAddress == "" {
		return fmt.Errorf("%w: an address cannot be listed in the directory unless it is public", ErrAddressBadSettings)
	}
	// The switch and its flavour go together for the same reason the
	// expiry and its flavour do: half of either is a state the address
	// cannot be in.
	if (s.DisabledAt == nil) != (s.DisabledMode == "") {
		return fmt.Errorf("%w: switching an address off needs a flavour and a flavour needs the switch", ErrAddressBadSettings)
	}
	if s.DisabledMode != "" && s.DisabledMode != AddressModeClosed && s.DisabledMode != AddressModeSilent {
		return fmt.Errorf("%w: disabled_mode must be %q or %q", ErrAddressBadSettings, AddressModeClosed, AddressModeSilent)
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
		Index:           a.Index,
		CreatedAt:       a.CreatedAt,
		RetiredAt:       a.RetiredAt,
		AddressSettings: a.AddressSettings,
	}
}

// IsPublic reports whether the owner published this address's plaintext.
func (a Address) IsPublic() bool { return a.PublicAddress != "" }

type AddressBlock struct {
	ActorID string `json:"actor_id"`
	// Hash is what they are refusing — the keyed hash of the address,
	// never the address, for the same reason Address.Hash exists.
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
