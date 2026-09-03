package mwanachamacomm

// IDGen/Clock/SystemClock, and lessByTimeThenID/newestFirst below, are
// ported unchanged from mwanachama-backend-api-gateway's
// internal/store/memory/{ids,order}.go — generic helpers the memory store
// files need and that carry no gateway dependency of their own.

import (
	"fmt"
	"sync"
	"time"
)

// IDGen mints monotonically-increasing string ids scoped to a prefix.
type IDGen struct {
	mu   sync.Mutex
	next map[string]uint64
}

// NewIDGen returns a fresh generator.
func NewIDGen() *IDGen {
	return &IDGen{next: map[string]uint64{}}
}

// New returns the next id for a prefix (e.g. "chapter" -> "chapter-1").
func (g *IDGen) New(prefix string) string {
	g.mu.Lock()
	g.next[prefix]++
	n := g.next[prefix]
	g.mu.Unlock()
	return fmt.Sprintf("%s-%d", prefix, n)
}

// Clock is a testable time source.
type Clock func() time.Time

// SystemClock is the default clock.
func SystemClock() time.Time { return time.Now().UTC() }

// lessByTimeThenID matches `ORDER BY <timestamp>, id`.
func lessByTimeThenID(ti, tj time.Time, idI, idJ string) bool {
	if ti.Equal(tj) {
		return idI < idJ
	}
	return ti.Before(tj)
}

// newestFirst matches `ORDER BY <timestamp> DESC, id DESC` — the shape the
// moderation listings use.
func newestFirst(ti, tj time.Time, idI, idJ string) bool {
	if ti.Equal(tj) {
		return idI > idJ
	}
	return ti.After(tj)
}
