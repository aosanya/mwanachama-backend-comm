package models

import (
	"errors"
	"testing"
	"time"
)

var stateNow = time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

func at(t time.Time) *time.Time { return &t }

func TestAddressLiveUntilRetiredOrExpired(t *testing.T) {
	live := Address{}
	if !live.Live(stateNow) {
		t.Fatalf("a plain address is live")
	}
	retired := Address{RetiredAt: at(stateNow.Add(-time.Hour))}
	if retired.Live(stateNow) || !retired.Retired() {
		t.Fatalf("a retired address is not live")
	}
	expiring := Address{ExpiresAt: at(stateNow), ExpiryMode: AddressModeClosed}
	if !expiring.Expired(stateNow) || expiring.Live(stateNow) {
		t.Fatalf("an address is expired at the instant it expires")
	}
	if expiring.Expired(stateNow.Add(-time.Second)) {
		t.Fatalf("an address is not expired a second before")
	}
}

func TestAddressOpenFollowsLiveEnabledAndHoursState(t *testing.T) {
	hours := &AddressHours{Zone: "UTC", Mode: AddressModeClosed}
	hours.Days[0] = &AddressDayWindow{FromMinute: 9 * 60, ToMinute: 11 * 60}
	cases := []struct {
		name        string
		a           Address
		open, unavl bool
	}{
		{"plain", Address{}, true, false},
		{"within hours", Address{Hours: hours}, true, false},
		{"outside hours", Address{Hours: &AddressHours{Zone: "UTC", Mode: AddressModeClosed, Days: [7]*AddressDayWindow{nil, {FromMinute: 0, ToMinute: 60}}}}, false, true},
		{"disabled", Address{DisabledAt: at(stateNow), DisabledMode: AddressModeClosed}, false, true},
		{"retired", Address{RetiredAt: at(stateNow)}, false, false},
		{"expired", Address{ExpiresAt: at(stateNow), ExpiryMode: AddressModeClosed}, false, false},
	}
	for _, c := range cases {
		if got := c.a.Open(stateNow); got != c.open {
			t.Errorf("%s: open=%v want %v", c.name, got, c.open)
		}
		if got := c.a.Unavailable(stateNow); got != c.unavl {
			t.Errorf("%s: unavailable=%v want %v", c.name, got, c.unavl)
		}
	}
}

func TestAddressSilencedOnlyWhenTheFlavourSaysSilent(t *testing.T) {
	closedHours := &AddressHours{Zone: "UTC", Mode: AddressModeClosed, Days: [7]*AddressDayWindow{nil, {FromMinute: 0, ToMinute: 60}}}
	silentHours := &AddressHours{Zone: "UTC", Mode: AddressModeSilent, Days: [7]*AddressDayWindow{nil, {FromMinute: 0, ToMinute: 60}}}
	cases := []struct {
		name string
		a    Address
		want bool
	}{
		{"nothing set", Address{}, false},
		{"expired closed", Address{ExpiresAt: at(stateNow), ExpiryMode: AddressModeClosed}, false},
		{"expired silent", Address{ExpiresAt: at(stateNow), ExpiryMode: AddressModeSilent}, true},
		{"not yet expired silent", Address{ExpiresAt: at(stateNow.Add(time.Hour)), ExpiryMode: AddressModeSilent}, false},
		{"disabled closed", Address{DisabledAt: at(stateNow), DisabledMode: AddressModeClosed}, false},
		{"disabled silent", Address{DisabledAt: at(stateNow), DisabledMode: AddressModeSilent}, true},
		{"outside hours closed", Address{Hours: closedHours}, false},
		{"outside hours silent", Address{Hours: silentHours}, true},
	}
	for _, c := range cases {
		if got := c.a.Silenced(stateNow); got != c.want {
			t.Errorf("%s: silenced=%v want %v", c.name, got, c.want)
		}
	}
}

func TestAddressSettingsRoundTripThroughTheAddress(t *testing.T) {
	ttl := 30
	s := AddressSettings{ExpiresAt: at(stateNow), ExpiryMode: AddressModeSilent, MessageTTLSeconds: &ttl, PublicAddress: "MKU4827YUT3391", ListedAt: at(stateNow), DisabledAt: at(stateNow), DisabledMode: AddressModeClosed}
	a := Address{ActorID: "a1", AddressIndex: 3}.WithSettings(s)
	got := a.Settings()
	if got.ExpiryMode != AddressModeSilent || *got.MessageTTLSeconds != 30 || got.PublicAddress != "MKU4827YUT3391" || got.DisabledMode != AddressModeClosed {
		t.Fatalf("settings did not survive: %+v", got)
	}
	if a.ActorID != "a1" || a.AddressIndex != 3 {
		t.Fatalf("settings overwrote identity: %+v", a)
	}
	if !a.IsPublic() || !a.Listed() || !a.Disabled() {
		t.Fatalf("flags did not follow the settings")
	}
	mine := a.Mine()
	if mine.Index != 3 || mine.PublicAddress != "MKU4827YUT3391" {
		t.Fatalf("mine view wrong: %+v", mine)
	}
	if pub := a.Public("MKU4827YUT3391"); pub.ActorID != "a1" || pub.Address != "MKU4827YUT3391" {
		t.Fatalf("public view wrong: %+v", pub)
	}
}

func TestAddressInDirectoryNeedsListedAndLive(t *testing.T) {
	listed := Address{PublicAddress: "X", ListedAt: at(stateNow)}
	if !listed.InDirectory(stateNow) {
		t.Fatalf("a listed live address is in the directory")
	}
	listed.RetiredAt = at(stateNow)
	if listed.InDirectory(stateNow) {
		t.Fatalf("a retired address leaves the directory")
	}
	if (Address{}).InDirectory(stateNow) {
		t.Fatalf("an unlisted address is not in the directory")
	}
}

func TestAddressSettingsValidateRefusesHalfSetPairs(t *testing.T) {
	zero := 0
	bad := map[string]AddressSettings{
		"expiry without flavour": {ExpiresAt: at(stateNow)},
		"flavour without expiry": {ExpiryMode: AddressModeClosed},
		"zero timer":             {MessageTTLSeconds: &zero},
		"listed but not public":  {ListedAt: at(stateNow)},
		"off without flavour":    {DisabledAt: at(stateNow)},
		"flavour without off":    {DisabledMode: AddressModeSilent},
		"bad hours":              {Hours: &AddressHours{}},
	}
	for name, s := range bad {
		if err := s.Validate(); !errors.Is(err, ErrAddressBadSettings) {
			t.Errorf("%s: want bad settings, got %v", name, err)
		}
	}
	good := AddressSettings{ExpiresAt: at(stateNow), ExpiryMode: AddressModeClosed, PublicAddress: "X", ListedAt: at(stateNow)}
	if err := good.Validate(); err != nil {
		t.Fatalf("good settings refused: %v", err)
	}
}

func TestDirectoryQueryNormalizedClampsPaging(t *testing.T) {
	cases := []struct {
		in         AddressDirectoryQuery
		limit, off int
		search     string
	}{
		{AddressDirectoryQuery{}, AddressDefaultDirectoryLimit, 0, ""},
		{AddressDirectoryQuery{Limit: 500, Offset: -4, Search: "  amina  "}, AddressMaxDirectoryLimit, 0, "amina"},
		{AddressDirectoryQuery{Limit: 40, Offset: 10}, 40, 10, ""},
	}
	for _, c := range cases {
		got := c.in.Normalized()
		if got.Limit != c.limit || got.Offset != c.off || got.Search != c.search {
			t.Errorf("%+v became %+v", c.in, got)
		}
	}
}

func TestDirectoryAddressSearchOnlyForAddressShapedText(t *testing.T) {
	if got := (AddressDirectoryQuery{Search: "mku 4827-yut"}).AddressSearch(); got != "MKU4827YUT" {
		t.Fatalf("got %q", got)
	}
	if got := (AddressDirectoryQuery{Search: "Amina Wanjiru"}).AddressSearch(); got != "AMINAWANJIRU" {
		t.Fatalf("letters only text still reads as address letters, got %q", got)
	}
	if got := (AddressDirectoryQuery{Search: "amina@home"}).AddressSearch(); got != "" {
		t.Fatalf("a name with punctuation cannot be an address, got %q", got)
	}
}
