// address_directory_impl.go — GORM-backed AddressDirectory implementation.
// Ported from mwanachama-backend-api-gateway's
// internal/store/postgres/address_directory.go.
package mwanachamacomm

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// AddressDirectoryStore answers the public-address directory in SQL.
//
// **One statement, joining two things.** A listing is an address's
// plaintext and its owner's display name, and that row exists in neither
// table this package owns. The join reaches the gateway's member table by
// its fixed physical name (`member_actors`, mwanachama-backend-actor's own
// production table for its "member" mounted instance) rather than by
// importing that module's Go types — this package cannot depend on
// another product repo's package, but every domain the gateway composes
// shares one physical Postgres database, and reaching a table by name is
// not a Go import. moderation_impl.go's `REFERENCES member_actors (id)`
// FKs in this repo's own migrations/000002_comm_tables.up.sql already
// lean on the identical fact.
type AddressDirectoryStore struct {
	db     *gorm.DB
	tables TableNames
	clock  Clock

	// membersTable is the fixed name of the gateway's member table —
	// "member_actors" in production. Overridable only so this package's
	// own tests can point it at a scratch stand-in table (see
	// testdb_test.go); every real caller uses [DefaultMembersTable].
	membersTable string
}

// DefaultMembersTable is mwanachama-backend-actor's production table name
// for the gateway's one mounted "member" instance
// (gormstore.DefaultTableNames("member").Actors, over there).
const DefaultMembersTable = "member_actors"

// NewAddressDirectoryStore constructs an AddressDirectoryStore backed by
// db, joining against membersTable (see [DefaultMembersTable] for the
// production value). clock defaults to [SystemClock] when nil.
func NewAddressDirectoryStore(db *gorm.DB, t TableNames, membersTable string, clock Clock) *AddressDirectoryStore {
	if clock == nil {
		clock = SystemClock
	}
	if membersTable == "" {
		membersTable = DefaultMembersTable
	}
	return &AddressDirectoryStore{db: db, tables: t, clock: clock, membersTable: membersTable}
}

// Search returns one page of the directory.
//
// ⚠️ **Only the permanent stops are in the WHERE.** retired_at and the
// expiry drop a row from the directory for good; disabled_at and hours do
// not, because an address shut for the weekend is one somebody should
// still be able to write down — see models.Address.InDirectory. Those two
// are read out and answered as `available`, which also keeps the paging
// exact: nothing is dropped in Go after the LIMIT has already been
// applied here.
//
// **The address is matched on the canonical form**, which is the only
// form this column ever holds, so the caller's spacing cannot decide
// whether they get a hit — AddressDirectoryQuery.AddressSearch has
// already stripped it. A term holding a character no address contains
// arrives as "" and the address half is skipped rather than scanned for
// something that cannot be there.
//
// **Byte-order tiebreak on both dialects, not a locale-aware one.**
// COLLATE "C" on Postgres and sqlite's own default TEXT collation
// (BINARY) already agree, so no dialect branch is needed for the ORDER
// BY itself — only for whether "COLLATE \"C\"" is valid syntax to say out
// loud, which it is not on sqlite.
//
// Case-insensitive matching is done with LOWER(...) LIKE LOWER(...)
// rather than Postgres's ILIKE, so the same statement runs on both
// dialects this package supports.
func (s *AddressDirectoryStore) Search(ctx context.Context, q models.AddressDirectoryQuery) ([]models.AddressListing, error) {
	q = q.Normalized()
	now := s.clock()

	collate := ""
	if s.db.Dialector.Name() == "postgres" {
		collate = ` COLLATE "C"`
	}

	query := `
SELECT a.public_address, a.member_id, m.display_name, a.listed_at, a.disabled_at, a.hours
  FROM ` + s.tables.Addresses + ` a
  JOIN ` + s.membersTable + ` m ON m.id = a.member_id
 WHERE a.listed_at IS NOT NULL AND a.public_address IS NOT NULL
   AND a.retired_at IS NULL AND (a.expires_at IS NULL OR a.expires_at > ?)
   AND (? = '' OR a.member_id <> ?)
   AND (? = '' OR LOWER(m.display_name) LIKE ? OR (? <> '' AND a.public_address LIKE ?))
 ORDER BY a.listed_at DESC, a.public_address` + collate + `
 LIMIT ? OFFSET ?`

	nameFrag := strings.ToLower(q.Search)
	addrFrag := q.AddressSearch()
	rows, err := s.db.WithContext(ctx).Raw(query,
		now,
		q.ExcludeMemberID, q.ExcludeMemberID,
		q.Search, "%"+nameFrag+"%", addrFrag, "%"+addrFrag+"%",
		q.Limit, q.Offset,
	).Rows()
	if err != nil {
		return nil, classify(err)
	}
	defer func() { _ = rows.Close() }()

	out := []models.AddressListing{}
	for rows.Next() {
		var l models.AddressListing
		var disabled flexTime
		var listed flexTime
		var hours []byte
		if err := rows.Scan(&l.Address, &l.MemberID, &l.DisplayName, &listed, &disabled, &hours); err != nil {
			return nil, classify(err)
		}
		if listed.Valid {
			l.ListedAt = listed.Time
		}
		l.Available = addressAvailableNow(disabled, hours, now)
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, classify(err)
	}
	return out, nil
}

// addressAvailableNow answers whether an address will open a thread right
// now, from the two reversible things that stop it.
//
// **A schedule that will not parse reads as no schedule**, and the row is
// still offered — the same errs-open reasoning models.AddressHours.OpenAt
// documents.
func addressAvailableNow(disabled flexTime, hours []byte, now time.Time) bool {
	if disabled.Valid {
		return false
	}
	if len(hours) == 0 {
		return true
	}
	var h models.AddressHours
	if err := json.Unmarshal(hours, &h); err != nil {
		return true
	}
	return h.OpenAt(now)
}

var _ models.AddressDirectory = (*AddressDirectoryStore)(nil)
