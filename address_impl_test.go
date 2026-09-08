package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func newAddressStore(t *testing.T) *mwanachamacomm.AddressStore {
	t.Helper()
	db, tables := newTestDB(t)
	s, err := mwanachamacomm.NewAddressStore(db, tables, monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("NewAddressStore: %v", err)
	}
	return s
}

func hashOf(s string) []byte { return []byte("hash:" + s) }

func TestAddressPublishAndResolve(t *testing.T) {
	s := newAddressStore(t)
	ctx := context.Background()
	a, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-1", Hash: hashOf("a1"), SaltID: 1, Index: 0})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if a.CreatedAt.IsZero() {
		t.Fatalf("Publish did not stamp CreatedAt")
	}
	got, err := s.Resolve(ctx, hashOf("a1"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.ActorID != "m-1" {
		t.Fatalf("Resolve.ActorID = %q, want m-1", got.ActorID)
	}
}

func TestAddressResolveUnknownRetiredExpiredAllNotFound(t *testing.T) {
	s := newAddressStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if _, err := s.Resolve(ctx, hashOf("nope")); !errors.Is(err, mwanachamacomm.ErrAddressNotFound) {
		t.Fatalf("unknown: expected ErrAddressNotFound, got %v", err)
	}

	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-1", Hash: hashOf("retired"), Index: 0}); err != nil {
		t.Fatalf("publish retired: %v", err)
	}
	if err := s.Retire(ctx, "m-1", 0, now); err != nil {
		t.Fatalf("retire: %v", err)
	}
	if _, err := s.Resolve(ctx, hashOf("retired")); !errors.Is(err, mwanachamacomm.ErrAddressNotFound) {
		t.Fatalf("retired: expected ErrAddressNotFound, got %v", err)
	}

	past := now.Add(-time.Hour)
	if _, err := s.Publish(ctx, mwanachamacomm.Address{
		ActorID: "m-1", Hash: hashOf("expired"), Index: 1,
		AddressSettings: mwanachamacomm.AddressSettings{ExpiresAt: &past, ExpiryMode: mwanachamacomm.AddressModeClosed},
	}); err != nil {
		t.Fatalf("publish expired: %v", err)
	}
	if _, err := s.Resolve(ctx, hashOf("expired")); !errors.Is(err, mwanachamacomm.ErrAddressNotFound) {
		t.Fatalf("expired: expected ErrAddressNotFound, got %v", err)
	}
}

func TestAddressPublishRefusesDuplicateHash(t *testing.T) {
	s := newAddressStore(t)
	ctx := context.Background()
	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-1", Hash: hashOf("dup"), Index: 0}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-2", Hash: hashOf("dup"), Index: 0}); err == nil {
		t.Fatalf("expected duplicate hash to be refused")
	}
}

func TestAddressPublishRefusesDuplicateActorIndex(t *testing.T) {
	s := newAddressStore(t)
	ctx := context.Background()
	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-1", Hash: hashOf("a"), Index: 0}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-1", Hash: hashOf("b"), Index: 0}); err == nil {
		t.Fatalf("expected duplicate (actor,index) to be refused")
	}
}

func TestAddressListForOrdersByIndexAndIncludesRetired(t *testing.T) {
	s := newAddressStore(t)
	ctx := context.Background()
	for i := 2; i >= 0; i-- {
		if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-1", Hash: hashOf(string(rune('a' + i))), Index: i}); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}
	if err := s.Retire(ctx, "m-1", 1, time.Now().UTC()); err != nil {
		t.Fatalf("retire: %v", err)
	}
	out, err := s.ListFor(ctx, "m-1")
	if err != nil || len(out) != 3 {
		t.Fatalf("ListFor = %+v, err %v, want 3", out, err)
	}
	for i, a := range out {
		if a.Index != i {
			t.Fatalf("ListFor[%d].Index = %d, want %d (not in address_index order)", i, a.Index, i)
		}
	}
	if !out[1].Retired() {
		t.Fatalf("ListFor[1] should be retired")
	}
}

func TestAddressRetireIsScopedAndNotIdempotentOnTimestamp(t *testing.T) {
	s := newAddressStore(t)
	ctx := context.Background()
	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-1", Hash: hashOf("a"), Index: 0}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// Naming someone else's address touches nothing.
	if err := s.Retire(ctx, "m-2", 0, time.Now().UTC()); !errors.Is(err, mwanachamacomm.ErrAddressNotFound) {
		t.Fatalf("cross-owner retire: expected ErrAddressNotFound, got %v", err)
	}
	if err := s.Retire(ctx, "m-1", 0, time.Now().UTC()); err != nil {
		t.Fatalf("retire: %v", err)
	}
	// Retiring twice cannot move the timestamp — the second call is a no-op
	// row match failure, not a second write.
	if err := s.Retire(ctx, "m-1", 0, time.Now().UTC()); !errors.Is(err, mwanachamacomm.ErrAddressNotFound) {
		t.Fatalf("second retire: expected ErrAddressNotFound, got %v", err)
	}
}

func TestAddressCountPublicIncludesRetired(t *testing.T) {
	s := newAddressStore(t)
	ctx := context.Background()
	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-1", Hash: hashOf("a"), Index: 0}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := s.UpdateSettings(ctx, "m-1", 0, mwanachamacomm.AddressSettings{PublicAddress: "MKU4827YUT3391"}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	n, err := s.CountPublic(ctx, "m-1")
	if err != nil || n != 1 {
		t.Fatalf("CountPublic = %d, err %v, want 1", n, err)
	}
	if err := s.Retire(ctx, "m-1", 0, time.Now().UTC()); err != nil {
		t.Fatalf("retire: %v", err)
	}
	n, err = s.CountPublic(ctx, "m-1")
	if err != nil || n != 1 {
		t.Fatalf("CountPublic after retire = %d, err %v, want 1 (retired rows still count)", n, err)
	}
}

func TestAddressUpdateSettingsWholeObjectAndValidation(t *testing.T) {
	s := newAddressStore(t)
	ctx := context.Background()
	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-1", Hash: hashOf("a"), Index: 0}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Invalid: listed without public.
	future := time.Now().UTC().Add(time.Hour)
	_, err := s.UpdateSettings(ctx, "m-1", 0, mwanachamacomm.AddressSettings{ListedAt: &future})
	if !errors.Is(err, mwanachamacomm.ErrAddressBadSettings) {
		t.Fatalf("expected ErrAddressBadSettings, got %v", err)
	}

	// Valid: set public+listed together.
	updated, err := s.UpdateSettings(ctx, "m-1", 0, mwanachamacomm.AddressSettings{
		PublicAddress: "MKU4827YUT3391",
		ListedAt:      &future,
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if !updated.Listed() || !updated.IsPublic() {
		t.Fatalf("expected the row to be public and listed, got %+v", updated.AddressSettings)
	}

	// Whole-object write: a second call with none of those fields clears
	// them, because absent means off.
	cleared, err := s.UpdateSettings(ctx, "m-1", 0, mwanachamacomm.AddressSettings{})
	if err != nil {
		t.Fatalf("clearing UpdateSettings: %v", err)
	}
	if cleared.Listed() || cleared.IsPublic() {
		t.Fatalf("expected a whole-object write to clear public/listed, got %+v", cleared.AddressSettings)
	}

	// Retired rows are excluded from UpdateSettings.
	if err := s.Retire(ctx, "m-1", 0, time.Now().UTC()); err != nil {
		t.Fatalf("retire: %v", err)
	}
	if _, err := s.UpdateSettings(ctx, "m-1", 0, mwanachamacomm.AddressSettings{}); !errors.Is(err, mwanachamacomm.ErrAddressNotFound) {
		t.Fatalf("UpdateSettings on a retired row: expected ErrAddressNotFound, got %v", err)
	}
}

func TestAddressListListedExcludesUnlistedAndOneActor(t *testing.T) {
	s := newAddressStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-1", Hash: hashOf("listed"), Index: 0}); err != nil {
		t.Fatalf("publish m-1: %v", err)
	}
	if _, err := s.UpdateSettings(ctx, "m-1", 0, mwanachamacomm.AddressSettings{PublicAddress: "MKU4827YUT3391", ListedAt: &now}); err != nil {
		t.Fatalf("list m-1: %v", err)
	}
	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-2", Hash: hashOf("unlisted"), Index: 0}); err != nil {
		t.Fatalf("publish m-2: %v", err)
	}
	if _, err := s.Publish(ctx, mwanachamacomm.Address{ActorID: "m-3", Hash: hashOf("also-listed"), Index: 0}); err != nil {
		t.Fatalf("publish m-3: %v", err)
	}
	if _, err := s.UpdateSettings(ctx, "m-3", 0, mwanachamacomm.AddressSettings{PublicAddress: "ERL2393NOP1234", ListedAt: &now}); err != nil {
		t.Fatalf("list m-3: %v", err)
	}

	out, err := s.ListListed(ctx, "", now)
	if err != nil || len(out) != 2 {
		t.Fatalf("ListListed(no exclude) = %+v, err %v, want 2", out, err)
	}

	out, err = s.ListListed(ctx, "m-3", now)
	if err != nil || len(out) != 1 || out[0].ActorID != "m-1" {
		t.Fatalf("ListListed(exclude m-3) = %+v, err %v, want just m-1", out, err)
	}
}

func TestAddressBlockIsIdempotentAndScoped(t *testing.T) {
	s := newAddressStore(t)
	ctx := context.Background()
	blocked, err := s.IsBlocked(ctx, "m-1", hashOf("a"))
	if err != nil || blocked {
		t.Fatalf("IsBlocked before any block = %v, err %v, want false", blocked, err)
	}
	if err := s.Block(ctx, mwanachamacomm.AddressBlock{ActorID: "m-1", Hash: hashOf("a")}); err != nil {
		t.Fatalf("Block: %v", err)
	}
	if err := s.Block(ctx, mwanachamacomm.AddressBlock{ActorID: "m-1", Hash: hashOf("a")}); err != nil {
		t.Fatalf("Block again: %v", err)
	}
	blocked, err = s.IsBlocked(ctx, "m-1", hashOf("a"))
	if err != nil || !blocked {
		t.Fatalf("IsBlocked after block = %v, err %v, want true", blocked, err)
	}
	blocked, err = s.IsBlocked(ctx, "m-2", hashOf("a"))
	if err != nil || blocked {
		t.Fatalf("IsBlocked for a different actor = %v, err %v, want false", blocked, err)
	}
}
