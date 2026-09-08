package models

import (
	"context"
	"strings"
	"time"
)

// Listed reports whether the owner asked for this address to be findable
// in the directory. Never true where IsPublic is false — the two are
// checked together by AddressSettings.Validate, so a listed row with no
// plaintext is a state that cannot be written.
func (a Address) Listed() bool { return a.ListedAt != nil }

// InDirectory reports whether this address should appear to somebody
// searching.
//
// **Listed and still alive — not listed and open.** An address switched
// off for the weekend or outside its own opening hours is *shut*, not
// *gone*, and telling somebody it does not exist would make them throw
// away a perfectly good address because they looked on a Sunday. A phone
// book lists the shop that closes on Sundays. So the directory drops only
// the permanent stops — retired, and past its own clock — and carries the
// temporary ones as AddressListing.Available, which the client draws
// rather than hides.
func (a Address) InDirectory(now time.Time) bool { return a.Listed() && a.Live(now) }

type AddressListing struct {
	// Address is the canonical form, `MKU4827YUT3391`. The client groups
	// it for reading; the wire keeps it canonical so a client cannot be
	// made to depend on the server's spacing.
	Address string `json:"address"`

	ActorID string `json:"actor_id"`

	DisplayName string `json:"display_name"`

	// ListedAt is when the owner asked to be found. Carried so the newest
	// listings can be shown first to somebody browsing with no search
	// term — a directory whose default order is alphabetical shows the
	// same faces forever.
	ListedAt time.Time `json:"listed_at"`

	// Available is whether the address will actually open a thread right
	// now — false while its owner has it switched off, or outside its
	// opening hours.
	//
	// **Shown rather than filtered.** A listing that vanishes on Sunday
	// is a directory that lies about who is in it; a listing that says
	// *closed right now* is one somebody writes down and uses on Monday.
	//
	// Permanent stops are not in here: an address that was retired or has
	// run out its clock is absent from the directory entirely
	// (Address.InDirectory).
	Available bool `json:"available"`
}

// AddressDirectoryQuery is one search.
type AddressDirectoryQuery struct {
	// Search matches a name **or** an address, and the caller does not
	// say which — somebody reading `ERL 2393` off a leaflet and somebody
	// typing `QA 3` are both looking for the same row, and asking them to
	// pick a mode first would be asking them to know something they do
	// not.
	//
	// An address is matched against the canonical form, so the spacing a
	// person uses never decides whether they get a hit. Empty browses.
	Search string

	ExcludeActorID string

	// Limit caps one page. Zero means AddressDefaultDirectoryLimit;
	// anything above AddressMaxDirectoryLimit is clamped rather than
	// refused, because a client asking for too many has made a mistake
	// about paging and not an attack — and the clamp is the same number
	// either way.
	Limit int

	// Offset pages. The order is total and stable (see AddressDirectory),
	// so a second page cannot repeat or skip a row that did not move.
	Offset int
}

// How much of the directory one request may take.
//
// **A directory is enumerable and that is what it is for**, so the cap is
// not pretending otherwise — it is a page size, not a defence. What keeps
// the disclosure bounded is that every row in it was put there by its
// owner.
const (
	// AddressDefaultDirectoryLimit is a screenful and a bit.
	AddressDefaultDirectoryLimit = 25

	// AddressMaxDirectoryLimit is what a client may ask for at once.
	AddressMaxDirectoryLimit = 100
)

func (q AddressDirectoryQuery) Normalized() AddressDirectoryQuery {
	q.Search = strings.TrimSpace(q.Search)
	if q.Limit <= 0 {
		q.Limit = AddressDefaultDirectoryLimit
	}
	if q.Limit > AddressMaxDirectoryLimit {
		q.Limit = AddressMaxDirectoryLimit
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	return q
}

// AddressSearch renders the search term as an address fragment, or "" if
// it cannot be one.
//
// **Punctuation is stripped and the rest upper-cased**, which is exactly
// what AddressNormalize does to a whole address — a fragment gets the
// same treatment for the same reason, so `erl 2393` and `ERL 2393` and
// `ERL-2393` are one search. Empty when the term holds a character no
// address contains, which is the common case for a name: matching
// `O'Brien` against the address column would spend a scan to find
// nothing.
func (q AddressDirectoryQuery) AddressSearch() string {
	var b strings.Builder
	for _, r := range q.Search {
		switch {
		case r == ' ' || r == '-' || r == '_' || r == '\t':
			continue
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 32)
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			return ""
		}
	}
	return b.String()
}

type AddressDirectory interface {
	// Search returns one page of listings, most recently listed first,
	// then by address so the order is total.
	//
	// **Most recent first is a choice about who gets seen.** Alphabetical
	// is the obvious default and it is the wrong one: a directory ordered
	// by name shows the same handful of people to everybody who browses
	// it, forever, and somebody who lists an address today would never
	// appear on a first page.
	//
	// Only rows that are listed and still alive come back — see
	// Address.InDirectory for why *shut for the weekend* is not the same
	// as *gone*, and rides along as AddressListing.Available instead of
	// being filtered. An empty page is an empty page, never an error.
	Search(ctx context.Context, q AddressDirectoryQuery) ([]AddressListing, error)
}
