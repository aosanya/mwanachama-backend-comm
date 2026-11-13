package models

import (
	"errors"
	"testing"
	"time"
)

func nairobi(t *testing.T, value string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		t.Fatal(err)
	}
	at, err := time.ParseInLocation("2006-01-02 15:04", value, loc)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func weekdayHours(mode string) *AddressHours {
	h := &AddressHours{Zone: "Africa/Nairobi", Mode: mode}
	for i := 0; i < 5; i++ {
		h.Days[i] = &AddressDayWindow{FromMinute: 9 * 60, ToMinute: 17 * 60}
	}
	return h
}

func TestAddressHoursOpenAtUsesTheOwnersZoneAndWeekday(t *testing.T) {
	h := weekdayHours(AddressModeClosed)
	cases := []struct {
		at   string
		want bool
	}{
		{"2026-11-09 09:00", true},
		{"2026-11-09 16:59", true},
		{"2026-11-09 17:00", false},
		{"2026-11-09 08:59", false},
		{"2026-11-14 12:00", false},
		{"2026-11-15 12:00", false},
	}
	for _, c := range cases {
		if got := h.OpenAt(nairobi(t, c.at)); got != c.want {
			t.Errorf("%s: OpenAt = %v, want %v", c.at, got, c.want)
		}
	}
}

func TestAddressHoursOpenAtConvertsFromUTC(t *testing.T) {
	h := weekdayHours(AddressModeClosed)
	if !h.OpenAt(time.Date(2026, 11, 9, 6, 30, 0, 0, time.UTC)) {
		t.Fatal("06:30 UTC is 09:30 in Nairobi and should be open")
	}
	if h.OpenAt(time.Date(2026, 11, 9, 14, 30, 0, 0, time.UTC)) {
		t.Fatal("14:30 UTC is 17:30 in Nairobi and should be shut")
	}
}

func TestAddressHoursSundayIsTheLastDay(t *testing.T) {
	h := &AddressHours{Zone: "UTC", Mode: AddressModeClosed}
	h.Days[6] = &AddressDayWindow{FromMinute: 0, ToMinute: 1440}
	if !h.OpenAt(time.Date(2026, 11, 15, 23, 59, 0, 0, time.UTC)) {
		t.Fatal("a Sunday window should be open on Sunday")
	}
	if h.OpenAt(time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("Monday has no window and should be shut")
	}
}

func TestAddressHoursUnknownZoneFailsOpen(t *testing.T) {
	h := AddressHours{Zone: "Nowhere/Land", Mode: AddressModeClosed}
	if !h.OpenAt(time.Now()) {
		t.Fatal("an unloadable zone should read as open")
	}
}

func TestAddressHoursAny(t *testing.T) {
	if (AddressHours{}).Any() {
		t.Fatal("no windows should not count as any")
	}
	if !weekdayHours(AddressModeClosed).Any() {
		t.Fatal("a weekday window should count")
	}
}

func TestAddressHoursValidateRefusals(t *testing.T) {
	good := func() AddressHours { return *weekdayHours(AddressModeSilent) }
	if err := good().Validate(); err != nil {
		t.Fatalf("good hours refused: %v", err)
	}
	cases := map[string]func(h *AddressHours){
		"no zone":            func(h *AddressHours) { h.Zone = "" },
		"unknown zone":       func(h *AddressHours) { h.Zone = "Nowhere/Land" },
		"bad mode":           func(h *AddressHours) { h.Mode = "off" },
		"opens below zero":   func(h *AddressHours) { h.Days[0].FromMinute = -1 },
		"opens at midnight+": func(h *AddressHours) { h.Days[0].FromMinute = 1440 },
		"closes at opening":  func(h *AddressHours) { h.Days[0].ToMinute = h.Days[0].FromMinute },
		"closes after day":   func(h *AddressHours) { h.Days[0].ToMinute = 1441 },
		"never opens":        func(h *AddressHours) { h.Days = [7]*AddressDayWindow{} },
	}
	for name, mutate := range cases {
		h := good()
		for i, w := range h.Days {
			if w != nil {
				copied := *w
				h.Days[i] = &copied
			}
		}
		mutate(&h)
		if err := h.Validate(); !errors.Is(err, ErrAddressBadSettings) {
			t.Errorf("%s: want ErrAddressBadSettings, got %v", name, err)
		}
	}
}

func TestAddressHoursAllowsClosingAtEndOfDay(t *testing.T) {
	h := AddressHours{Zone: "UTC", Mode: AddressModeClosed}
	h.Days[2] = &AddressDayWindow{FromMinute: 0, ToMinute: 1440}
	if err := h.Validate(); err != nil {
		t.Fatalf("1440 should be allowed: %v", err)
	}
}

func TestAddressLiveRetiredAndExpired(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(time.Hour)

	if !(Address{}).Live(now) {
		t.Fatal("a plain address is live")
	}
	if (Address{RetiredAt: &past}).Live(now) {
		t.Fatal("a retired address is not live")
	}
	if !(Address{ExpiresAt: &future}).Live(now) {
		t.Fatal("an address expiring later is live")
	}
	if (Address{ExpiresAt: &now}).Live(now) {
		t.Fatal("an address is expired at the instant it expires")
	}
	if !(Address{ExpiresAt: &past}).Expired(now) {
		t.Fatal("a past expiry is expired")
	}
}

func TestAddressOpenNeedsLiveEnabledAndWithinHours(t *testing.T) {
	now := nairobi(t, "2026-11-09 10:00")
	late := nairobi(t, "2026-11-09 20:00")
	hours := weekdayHours(AddressModeClosed)
	off := now.Add(-time.Minute)

	if !(Address{Hours: hours}).Open(now) {
		t.Fatal("inside hours should be open")
	}
	if (Address{Hours: hours}).Open(late) {
		t.Fatal("outside hours should be shut")
	}
	if (Address{DisabledAt: &off, DisabledMode: AddressModeClosed}).Open(now) {
		t.Fatal("a switched-off address should be shut")
	}
	if !(Address{}).Open(now) {
		t.Fatal("no hours means always within hours")
	}
}

func TestAddressUnavailableSeparatesTemporarilyShutFromGone(t *testing.T) {
	now := nairobi(t, "2026-11-09 20:00")
	gone := now.Add(-time.Hour)
	hours := weekdayHours(AddressModeClosed)

	if !(Address{Hours: hours}).Unavailable(now) {
		t.Fatal("outside hours is unavailable")
	}
	if (Address{RetiredAt: &gone, Hours: hours}).Unavailable(now) {
		t.Fatal("a retired address must look like it never existed")
	}
	if (Address{ExpiresAt: &gone}).Unavailable(now) {
		t.Fatal("an expired address must look like it never existed")
	}
	if (Address{}).Unavailable(now) {
		t.Fatal("an open address is not unavailable")
	}
}

func TestAddressSilencedOnlyForTheSilentFlavour(t *testing.T) {
	now := nairobi(t, "2026-11-09 20:00")
	past := now.Add(-time.Hour)

	cases := map[string]struct {
		a    Address
		want bool
	}{
		"expired silent":   {Address{ExpiresAt: &past, ExpiryMode: AddressModeSilent}, true},
		"expired closed":   {Address{ExpiresAt: &past, ExpiryMode: AddressModeClosed}, false},
		"disabled silent":  {Address{DisabledAt: &past, DisabledMode: AddressModeSilent}, true},
		"disabled closed":  {Address{DisabledAt: &past, DisabledMode: AddressModeClosed}, false},
		"off hours silent": {Address{Hours: weekdayHours(AddressModeSilent)}, true},
		"off hours closed": {Address{Hours: weekdayHours(AddressModeClosed)}, false},
		"in hours silent":  {Address{Hours: weekdayHours(AddressModeSilent)}, false},
		"nothing set":      {Address{}, false},
	}
	for name, c := range cases {
		at := now
		if name == "in hours silent" {
			at = nairobi(t, "2026-11-09 10:00")
		}
		if got := c.a.Silenced(at); got != c.want {
			t.Errorf("%s: Silenced = %v, want %v", name, got, c.want)
		}
	}
}

func TestAddressSettingsValidateRefusals(t *testing.T) {
	at := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	zero, five := 0, 5
	cases := map[string]AddressSettings{
		"expiry without flavour":   {ExpiresAt: &at},
		"flavour without expiry":   {ExpiryMode: AddressModeClosed},
		"zero second timer":        {MessageTTLSeconds: &zero},
		"listed but not public":    {ListedAt: &at},
		"disabled without flavour": {DisabledAt: &at},
		"flavour without switch":   {DisabledMode: AddressModeSilent},
		"bad hours":                {Hours: &AddressHours{}},
	}
	for name, s := range cases {
		if err := s.Validate(); !errors.Is(err, ErrAddressBadSettings) {
			t.Errorf("%s: want ErrAddressBadSettings, got %v", name, err)
		}
	}
	ok := AddressSettings{ExpiresAt: &at, ExpiryMode: AddressModeClosed, MessageTTLSeconds: &five, PublicAddress: "MKU4827YUT3391", ListedAt: &at, Hours: weekdayHours(AddressModeClosed)}
	if err := ok.Validate(); err != nil {
		t.Fatalf("well formed settings refused: %v", err)
	}
	if err := (AddressSettings{}).Validate(); err != nil {
		t.Fatalf("empty settings refused: %v", err)
	}
}

func TestAddressSettingsRoundTripThroughWithSettings(t *testing.T) {
	at := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	s := AddressSettings{DisabledAt: &at, DisabledMode: AddressModeSilent, PublicAddress: "MKU4827YUT3391"}
	a := Address{ActorID: "a1", AddressIndex: 3}.WithSettings(s)
	if a.ActorID != "a1" || a.AddressIndex != 3 {
		t.Fatalf("identity changed: %+v", a)
	}
	got := a.Settings()
	if got.DisabledMode != AddressModeSilent || got.PublicAddress != "MKU4827YUT3391" || got.DisabledAt != &at {
		t.Fatalf("settings did not round trip: %+v", got)
	}
}

func TestAddressProjections(t *testing.T) {
	created := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	a := Address{ActorID: "a1", AddressIndex: 2, CreatedAt: created, PublicAddress: "MKU4827YUT3391"}

	if p := a.Public("MKU4827YUT3391"); p.Address != "MKU4827YUT3391" || p.ActorID != "a1" {
		t.Fatalf("public projection: %+v", p)
	}
	m := a.Mine()
	if m.Index != 2 || !m.CreatedAt.Equal(created) || m.PublicAddress != "MKU4827YUT3391" {
		t.Fatalf("mine projection: %+v", m)
	}
	if !a.IsPublic() || (Address{}).IsPublic() {
		t.Fatal("IsPublic should follow PublicAddress")
	}
}
