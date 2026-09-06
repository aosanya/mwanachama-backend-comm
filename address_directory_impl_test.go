package mwanachamacomm_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

// newAddressDirectoryStore builds an AddressStore and an
// AddressDirectoryStore over the same scratch *gorm.DB (sqlite's
// ":memory:" DSN is one isolated database per open, so both stores and the
// stand-in member table must share this one connection, not each open
// their own).
func newAddressDirectoryStore(t *testing.T) (*mwanachamacomm.AddressDirectoryStore, *mwanachamacomm.AddressStore, *gorm.DB) {
	t.Helper()
	db, tables := newTestDB(t)
	createTestMembers(t, db)
	clock := monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	addrs, err := mwanachamacomm.NewAddressStore(db, tables, clock)
	if err != nil {
		t.Fatalf("NewAddressStore: %v", err)
	}
	dir := mwanachamacomm.NewAddressDirectoryStore(db, tables, testMembersTable, clock)
	return dir, addrs, db
}

func publishListed(t *testing.T, addrs *mwanachamacomm.AddressStore, memberID string, index int, publicAddr string, listedAt time.Time) {
	t.Helper()
	ctx := context.Background()
	if _, err := addrs.Publish(ctx, models.Address{MemberID: memberID, Hash: hashOf(memberID + publicAddr), Index: index}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := addrs.UpdateSettings(ctx, memberID, index, models.AddressSettings{
		PublicAddress: publicAddr,
		ListedAt:      &listedAt,
	}); err != nil {
		t.Fatalf("UpdateSettings (listing): %v", err)
	}
}

func TestAddressDirectorySearchOnlyListsPublicAndListed(t *testing.T) {
	dir, addrs, db := newAddressDirectoryStore(t)
	ctx := context.Background()

	insertTestMember(t, db, "m-1", "Alice")
	insertTestMember(t, db, "m-2", "Bob")

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	publishListed(t, addrs, "m-1", 0, "MKU4827YUT3391", now)

	// m-2 publishes an address but never lists it — must not appear.
	if _, err := addrs.Publish(ctx, models.Address{MemberID: "m-2", Hash: hashOf("m-2-unlisted"), Index: 0}); err != nil {
		t.Fatalf("publish unlisted: %v", err)
	}
	if _, err := addrs.UpdateSettings(ctx, "m-2", 0, models.AddressSettings{PublicAddress: "ERL2393NOP1234"}); err != nil {
		t.Fatalf("UpdateSettings unlisted: %v", err)
	}

	out, err := dir.Search(ctx, models.AddressDirectoryQuery{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("Search = %+v, want 1 listed row", out)
	}
	if out[0].Address != "MKU4827YUT3391" || out[0].MemberID != "m-1" || out[0].DisplayName != "Alice" {
		t.Fatalf("Search[0] = %+v, want Alice's MKU4827YUT3391", out[0])
	}
	if !out[0].Available {
		t.Fatalf("Search[0].Available = false, want true (no disable, no hours)")
	}
}

func TestAddressDirectorySearchExcludesCallerAndMatchesEitherField(t *testing.T) {
	dir, addrs, db := newAddressDirectoryStore(t)
	ctx := context.Background()
	insertTestMember(t, db, "m-1", "Alice Wanjiru")
	insertTestMember(t, db, "m-2", "Bob Otieno")

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	publishListed(t, addrs, "m-1", 0, "MKU4827YUT3391", now)
	publishListed(t, addrs, "m-2", 0, "ERL2393NOP1234", now.Add(time.Minute))

	// Excluding m-1 drops Alice's row.
	out, err := dir.Search(ctx, models.AddressDirectoryQuery{ExcludeMemberID: "m-1"})
	if err != nil || len(out) != 1 || out[0].MemberID != "m-2" {
		t.Fatalf("Search(exclude m-1) = %+v, err %v, want just m-2", out, err)
	}

	// Name search, case-insensitive.
	out, err = dir.Search(ctx, models.AddressDirectoryQuery{Search: "wanjiru"})
	if err != nil || len(out) != 1 || out[0].MemberID != "m-1" {
		t.Fatalf("Search(name) = %+v, err %v, want just m-1", out, err)
	}

	// Address search, punctuation-and-case tolerant.
	out, err = dir.Search(ctx, models.AddressDirectoryQuery{Search: "erl 2393"})
	if err != nil || len(out) != 1 || out[0].MemberID != "m-2" {
		t.Fatalf("Search(address fragment) = %+v, err %v, want just m-2", out, err)
	}
}

func TestAddressDirectorySearchOrdersNewestListedFirst(t *testing.T) {
	dir, addrs, db := newAddressDirectoryStore(t)
	ctx := context.Background()
	insertTestMember(t, db, "m-1", "Alice")
	insertTestMember(t, db, "m-2", "Bob")

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	publishListed(t, addrs, "m-1", 0, "MKU4827YUT3391", base)
	publishListed(t, addrs, "m-2", 0, "ERL2393NOP1234", base.Add(time.Hour))

	out, err := dir.Search(ctx, models.AddressDirectoryQuery{})
	if err != nil || len(out) != 2 {
		t.Fatalf("Search = %+v, err %v, want 2", out, err)
	}
	if out[0].MemberID != "m-2" {
		t.Fatalf("Search[0] = %+v, want the most recently listed (m-2) first", out[0])
	}
}

func TestAddressDirectorySearchShowsUnavailableRatherThanHiding(t *testing.T) {
	dir, addrs, db := newAddressDirectoryStore(t)
	ctx := context.Background()
	insertTestMember(t, db, "m-1", "Alice")

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if _, err := addrs.Publish(ctx, models.Address{MemberID: "m-1", Hash: hashOf("m-1-switched-off"), Index: 0}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := addrs.UpdateSettings(ctx, "m-1", 0, models.AddressSettings{
		PublicAddress: "MKU4827YUT3391",
		ListedAt:      &now,
		DisabledAt:    &now,
		DisabledMode:  models.AddressModeClosed,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	out, err := dir.Search(ctx, models.AddressDirectoryQuery{})
	if err != nil || len(out) != 1 {
		t.Fatalf("Search = %+v, err %v, want 1 (switched-off is shown, not hidden)", out, err)
	}
	if out[0].Available {
		t.Fatalf("Search[0].Available = true, want false (switched off)")
	}
}
