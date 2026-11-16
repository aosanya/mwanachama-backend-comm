package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	comm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

var addressNow = time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

func addressStore(t *testing.T) *comm.AddressStore {
	t.Helper()
	_, db, s := gatedManagerOn(t, nil)
	st, err := comm.NewAddressStore(db, s, func() time.Time { return addressNow })
	if err != nil {
		t.Fatalf("NewAddressStore: %v", err)
	}
	return st
}

func addressRow(actor string, index int, hash string) models.Address {
	return models.Address{ActorID: actor, AddressIndex: index, Hash: []byte(hash)}
}

func TestAnAddressStoreNeedsADatabase(t *testing.T) {
	if _, err := comm.NewAddressStore(nil, nil, nil); err == nil {
		t.Fatalf("a nil database was accepted")
	}
}

func TestPublishedAddressesListInIndexOrderAndStampTheClock(t *testing.T) {
	ctx := context.Background()
	st := addressStore(t)
	for _, i := range []int{2, 0, 1} {
		if _, err := st.Publish(ctx, addressRow("a1", i, "h"+string(rune('0'+i)))); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}
	got, err := st.ListFor(ctx, "a1")
	if err != nil || len(got) != 3 {
		t.Fatalf("list: %v %v", got, err)
	}
	for i, a := range got {
		if a.AddressIndex != i {
			t.Fatalf("position %d holds index %d", i, a.AddressIndex)
		}
		if !a.CreatedAt.Equal(addressNow) {
			t.Fatalf("created at %v, want the clock's %v", a.CreatedAt, addressNow)
		}
	}
	if other, _ := st.ListFor(ctx, "someone-else"); len(other) != 0 {
		t.Fatalf("another actor sees %d addresses", len(other))
	}
}

func TestPublishRefusesAnOwnerlessAddressAndABadSetting(t *testing.T) {
	ctx := context.Background()
	st := addressStore(t)
	if _, err := st.Publish(ctx, addressRow("", 0, "h")); !errors.Is(err, models.ErrAddressBadSettings) {
		t.Fatalf("ownerless: %v", err)
	}
	bad := addressRow("a1", 0, "h")
	bad.ExpiryMode = models.AddressModeClosed
	if _, err := st.Publish(ctx, bad); !errors.Is(err, models.ErrAddressBadSettings) {
		t.Fatalf("a flavour without an expiry: %v", err)
	}
}

func TestPublishingTheSameIndexTwiceIsAConflict(t *testing.T) {
	ctx := context.Background()
	st := addressStore(t)
	if _, err := st.Publish(ctx, addressRow("a1", 0, "h1")); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := st.Publish(ctx, addressRow("a1", 0, "h2")); !errors.Is(err, comm.ErrConflict) {
		t.Fatalf("second: want conflict, got %v", err)
	}
}

func TestResolveFindsLiveAddressesOnly(t *testing.T) {
	ctx := context.Background()
	st := addressStore(t)
	live := addressRow("a1", 0, "live-hash")
	expired := addressRow("a1", 1, "expired-hash")
	past := addressNow.Add(-time.Minute)
	expired.ExpiresAt, expired.ExpiryMode = &past, models.AddressModeClosed
	retired := addressRow("a1", 2, "retired-hash")
	for _, a := range []models.Address{live, expired, retired} {
		if _, err := st.Publish(ctx, a); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	if err := st.Retire(ctx, "a1", 2, time.Time{}); err != nil {
		t.Fatalf("retire: %v", err)
	}
	if got, err := st.Resolve(ctx, []byte("live-hash")); err != nil || got.ActorID != "a1" {
		t.Fatalf("live: %v %v", got, err)
	}
	for _, h := range []string{"expired-hash", "retired-hash", "unknown-hash"} {
		if _, err := st.Resolve(ctx, []byte(h)); !errors.Is(err, models.ErrAddressNotFound) {
			t.Errorf("%s: want not found, got %v", h, err)
		}
	}
}

func TestRetireWorksOnceAndOnlyForTheOwner(t *testing.T) {
	ctx := context.Background()
	st := addressStore(t)
	st.Publish(ctx, addressRow("a1", 0, "h"))
	if err := st.Retire(ctx, "a2", 0, addressNow); !errors.Is(err, models.ErrAddressNotFound) {
		t.Fatalf("someone else retiring: %v", err)
	}
	if err := st.Retire(ctx, "a1", 0, addressNow); err != nil {
		t.Fatalf("retire: %v", err)
	}
	if err := st.Retire(ctx, "a1", 0, addressNow); !errors.Is(err, models.ErrAddressNotFound) {
		t.Fatalf("retiring twice: %v", err)
	}
	got, _ := st.ListFor(ctx, "a1")
	if len(got) != 1 || got[0].RetiredAt == nil || !got[0].Retired() {
		t.Fatalf("the retired address should stay listed as retired: %+v", got)
	}
}

func TestCountPublicCountsOnlyAddressesWithAPublicForm(t *testing.T) {
	ctx := context.Background()
	st := addressStore(t)
	private := addressRow("a1", 0, "h0")
	public := addressRow("a1", 1, "h1")
	public.PublicAddress = "MKU4827YUT3391"
	for _, a := range []models.Address{private, public} {
		if _, err := st.Publish(ctx, a); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	if n, err := st.CountPublic(ctx, "a1"); err != nil || n != 1 {
		t.Fatalf("count = %d, %v", n, err)
	}
	if n, _ := st.CountPublic(ctx, "a2"); n != 0 {
		t.Fatalf("another actor has %d", n)
	}
}

func TestUpdateSettingsChangesTheLiveAddressAndReturnsIt(t *testing.T) {
	ctx := context.Background()
	st := addressStore(t)
	st.Publish(ctx, addressRow("a1", 0, "h"))
	ttl := 600
	disabled := addressNow
	got, err := st.UpdateSettings(ctx, "a1", 0, models.AddressSettings{
		PublicAddress:     "MKU4827YUT3391",
		MessageTTLSeconds: &ttl,
		DisabledAt:        &disabled,
		DisabledMode:      models.AddressModeSilent,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.PublicAddress != "MKU4827YUT3391" || got.MessageTTLSeconds == nil || *got.MessageTTLSeconds != 600 || !got.Disabled() || got.DisabledMode != models.AddressModeSilent {
		t.Fatalf("settings not stored: %+v", got)
	}
	if got.Open(addressNow) || !got.Silenced(addressNow) {
		t.Fatalf("a silently disabled address must be neither open nor audible")
	}
	cleared, err := st.UpdateSettings(ctx, "a1", 0, models.AddressSettings{})
	if err != nil || cleared.Disabled() || cleared.PublicAddress != "" || cleared.MessageTTLSeconds != nil {
		t.Fatalf("clearing did not clear: %+v %v", cleared, err)
	}
}

func TestUpdateSettingsRefusesBadSettingsAndMissingOrRetiredAddresses(t *testing.T) {
	ctx := context.Background()
	st := addressStore(t)
	st.Publish(ctx, addressRow("a1", 0, "h"))
	zero := 0
	if _, err := st.UpdateSettings(ctx, "a1", 0, models.AddressSettings{MessageTTLSeconds: &zero}); !errors.Is(err, models.ErrAddressBadSettings) {
		t.Fatalf("zero timer: %v", err)
	}
	if _, err := st.UpdateSettings(ctx, "a1", 0, models.AddressSettings{PublicAddress: "bad"}); !errors.Is(err, models.ErrAddressBadSettings) {
		t.Fatalf("malformed public address: %v", err)
	}
	if _, err := st.UpdateSettings(ctx, "a1", 9, models.AddressSettings{}); !errors.Is(err, models.ErrAddressNotFound) {
		t.Fatalf("missing index: %v", err)
	}
	st.Retire(ctx, "a1", 0, addressNow)
	if _, err := st.UpdateSettings(ctx, "a1", 0, models.AddressSettings{}); !errors.Is(err, models.ErrAddressNotFound) {
		t.Fatalf("retired address changed: %v", err)
	}
}

func TestBlockingIsPerActorAndIdempotent(t *testing.T) {
	ctx := context.Background()
	st := addressStore(t)
	block := models.AddressBlock{ActorID: "a1", Hash: []byte("spammer")}
	for i := 0; i < 2; i++ {
		if err := st.Block(ctx, block); err != nil {
			t.Fatalf("block %d: %v", i, err)
		}
	}
	if yes, err := st.IsBlocked(ctx, "a1", []byte("spammer")); err != nil || !yes {
		t.Fatalf("blocked = %v, %v", yes, err)
	}
	if yes, _ := st.IsBlocked(ctx, "a2", []byte("spammer")); yes {
		t.Fatalf("a block leaked to another actor")
	}
	if yes, _ := st.IsBlocked(ctx, "a1", []byte("friend")); yes {
		t.Fatalf("an unblocked sender reads as blocked")
	}
}

func TestListListedShowsLiveListedAddressesExceptTheCallers(t *testing.T) {
	ctx := context.Background()
	st := addressStore(t)
	listed := func(actor string, index int, public string) models.Address {
		a := addressRow(actor, index, actor+public)
		at := addressNow.Add(-time.Hour)
		a.PublicAddress, a.ListedAt = public, &at
		return a
	}
	rows := []models.Address{
		listed("a1", 0, "MKU4827YUT3391"),
		listed("a2", 0, "ABC1234DEF5678"),
		addressRow("a3", 0, "unlisted"),
	}
	expired := listed("a4", 0, "ZZZ0000ZZZ0000")
	past := addressNow.Add(-time.Minute)
	expired.ExpiresAt, expired.ExpiryMode = &past, models.AddressModeClosed
	rows = append(rows, expired)
	for _, a := range rows {
		if _, err := st.Publish(ctx, a); err != nil {
			t.Fatalf("publish %s: %v", a.ActorID, err)
		}
	}
	all, err := st.ListListed(ctx, "", addressNow)
	if err != nil || len(all) != 2 {
		t.Fatalf("all = %d, %v", len(all), err)
	}
	others, _ := st.ListListed(ctx, "a1", addressNow)
	if len(others) != 1 || others[0].ActorID != "a2" {
		t.Fatalf("excluding a1 left %+v", others)
	}
}
