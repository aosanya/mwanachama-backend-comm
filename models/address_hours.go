package models

import (
	"fmt"
	"time"

	// The zone an address's hours are written in arrives as an IANA name
	// off a handset, and the gateway has to be able to load it wherever it
	// runs. A scratch container carries no zoneinfo, and
	// time.LoadLocation failing there would turn "open 09:00–17:00 in
	// Nairobi" into an address that is closed forever — silently, and
	// only in production. Embedding the database costs about 450KB and
	// removes the whole class.
	_ "time/tzdata"
)

// AddressHours are when an address is open — a window per weekday, in the
// owner's own zone.
//
// **A recurring rule, evaluated on read, exactly like ExpiresAt.** Nothing
// sweeps it and nothing schedules anything: a live read tests whether *now*
// falls inside today's window. So editing the hours changes the effect
// immediately, in both directions, and an address that was closed a minute
// ago is open the moment the clock passes the opening time — nothing was
// torn down, so nothing has to be rebuilt.
//
// **Why a whole window per day rather than one window and a set of days:**
// real opening hours are not uniform. Saturday closes early, Sunday is
// shut, and the Friday rally runs late. Seven independent windows say all
// of that; one window and seven checkboxes cannot say any of it.
type AddressHours struct {
	// Zone is the IANA name the windows are written in —
	// "Africa/Nairobi", "Europe/Berlin".
	//
	// **It has to be stored, and it has to come from the handset.** The
	// window means *nine in the morning where the member is standing*,
	// and neither UTC minutes nor the server's own zone can reconstruct
	// that: a fixed offset stored today is wrong the day the zone changes
	// its rules, and the server's zone is a fact about a data centre. The
	// name is the only form that stays true.
	Zone string `json:"zone"`

	// Mode is what being outside the hours does — AddressModeClosed or
	// AddressModeSilent, the same two words the expiry uses and with the
	// same meanings: *closed* stops anyone new reaching you and leaves
	// your conversations alone; *silent* also stops the conversations
	// opened through this address, until the hours come round again.
	Mode string `json:"mode"`

	// Days holds one window per weekday, **Monday first**, and a nil
	// entry is a day the address is shut all day.
	//
	// Monday first rather than Go's Sunday-first time.Weekday because
	// that is how opening hours are written on a door everywhere this
	// runs; the conversion happens in exactly one place, [addressDayIndex],
	// so nothing else has to remember it.
	Days [7]*AddressDayWindow `json:"days"`
}

// AddressDayWindow is one day's opening, in minutes past local midnight.
//
// **Minutes, not a time-of-day type**, because that is what survives the
// round trip through JSON, Postgres and a mobile client without any of the
// three inventing a date to hang the time on.
//
// A window that runs past midnight is written as two days — Friday 22:00
// to 24:00 and Saturday 00:00 to 02:00 — rather than as a wrapping range.
// Wrapping would make "is now inside it" depend on yesterday's row as well
// as today's, and every reader would have to get that right; two windows
// are unambiguous and a member reading them back on the door can check
// them.
type AddressDayWindow struct {
	// FromMinute is when the day opens, 0 (midnight) to 1439.
	FromMinute int `json:"from_minute"`

	// ToMinute is when it closes, 1 to 1440. **1440 is midnight at the
	// end of the day**, and it exists so "open until midnight" does not
	// have to be written as 23:59.
	ToMinute int `json:"to_minute"`
}

// addressMinutesInDay is 24×60. ToMinute may equal it; FromMinute may not.
const addressMinutesInDay = 24 * 60

// OpenAt reports whether the address is inside its hours at t.
//
// **A failure to load the zone reads as open**, not as closed. The
// alternative is an address that silently stops reaching its owner
// because a zone name went stale between the release that wrote it and
// the release that reads it — a member would see nothing wrong on their
// own screen and simply never hear from anybody. Erring open costs a
// window being honoured late; erring closed costs a member their
// correspondence, with nothing on any screen to explain it.
func (h AddressHours) OpenAt(t time.Time) bool {
	loc, err := time.LoadLocation(h.Zone)
	if err != nil {
		return true
	}
	local := t.In(loc)
	w := h.Days[addressDayIndex(local.Weekday())]
	if w == nil {
		return false
	}
	minute := local.Hour()*60 + local.Minute()
	return minute >= w.FromMinute && minute < w.ToMinute
}

// Any reports whether these hours open at all. Seven nil days is a rule
// that can never let anybody through, which [AddressSettings.Validate]
// refuses — a member who wants that has asked to disable the address, and
// there is a switch for it that they can undo.
func (h AddressHours) Any() bool {
	for _, w := range h.Days {
		if w != nil {
			return true
		}
	}
	return false
}

// Validate reports whether these hours describe a schedule an address can
// be on. Called from [AddressSettings.Validate]; kept here so the shape
// and its rule live in the same file.
func (h AddressHours) Validate() error {
	if h.Zone == "" {
		return fmt.Errorf("%w: hours need the zone they are written in", ErrAddressBadSettings)
	}
	if _, err := time.LoadLocation(h.Zone); err != nil {
		return fmt.Errorf("%w: %q is not a zone this gateway knows", ErrAddressBadSettings, h.Zone)
	}
	if h.Mode != AddressModeClosed && h.Mode != AddressModeSilent {
		return fmt.Errorf("%w: hours mode must be %q or %q", ErrAddressBadSettings, AddressModeClosed, AddressModeSilent)
	}
	for i, w := range h.Days {
		if w == nil {
			continue
		}
		if w.FromMinute < 0 || w.FromMinute >= addressMinutesInDay {
			return fmt.Errorf("%w: day %d opens outside the day", ErrAddressBadSettings, i)
		}
		if w.ToMinute <= w.FromMinute || w.ToMinute > addressMinutesInDay {
			return fmt.Errorf("%w: day %d closes before it opens", ErrAddressBadSettings, i)
		}
	}
	if !h.Any() {
		return fmt.Errorf("%w: hours that never open are a disabled address, which is its own switch", ErrAddressBadSettings)
	}
	return nil
}

// addressDayIndex maps Go's Sunday-first weekday onto Monday-first
// [AddressHours.Days].
func addressDayIndex(d time.Weekday) int {
	if d == time.Sunday {
		return 6
	}
	return int(d) - 1
}
