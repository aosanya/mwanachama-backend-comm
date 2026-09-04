package mwanachamacomm

import "time"

// Clock is a testable time source, used by every store constructor to stamp
// a row's CreatedAt/UpdatedAt/etc. in Go before Create rather than relying
// on a database DEFAULT — the same reason actor's models.NowRFC3339 exists,
// generalised to an injectable func so today's deterministic-clock tests
// port with minimal change. IDGen and the lessByTimeThenID/newestFirst sort
// helpers this file used to hold are retired: GORM mints row ids (see
// gormstore.mintID) and every list read orders via the query builder's
// Order(...) instead of a Go-side sort.
type Clock func() time.Time

// SystemClock is the default clock.
func SystemClock() time.Time { return time.Now().UTC() }
