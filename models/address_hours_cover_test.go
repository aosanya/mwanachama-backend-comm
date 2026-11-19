package models

import (
	"errors"
	"testing"
	"time"
)

func weekdayWindowHours(from, to int) AddressHours {
	h := AddressHours{Zone: "Africa/Nairobi", Mode: AddressModeClosed}
	for i := 0; i < 5; i++ {
		h.Days[i] = &AddressDayWindow{FromMinute: from, ToMinute: to}
	}
	return h
}

func nairobiInNovember(day, hour, minute int) time.Time {
	loc, _ := time.LoadLocation("Africa/Nairobi")
	return time.Date(2026, 11, day, hour, minute, 0, 0, loc)
}

func TestHoursOpenAtTestsTheOwnersLocalWindow(t *testing.T) {
	h := weekdayWindowHours(9*60, 17*60)
	cases := []struct {
		name string
		at   time.Time
		open bool
	}{
		{"monday before opening", nairobiInNovember(9, 8, 59), false},
		{"monday at opening", nairobiInNovember(9, 9, 0), true},
		{"monday last minute", nairobiInNovember(9, 16, 59), true},
		{"monday at closing", nairobiInNovember(9, 17, 0), false},
		{"saturday has no window", nairobiInNovember(14, 12, 0), false},
		{"sunday has no window", nairobiInNovember(15, 12, 0), false},
		{"friday midday", nairobiInNovember(13, 12, 0), true},
	}
	for _, c := range cases {
		if got := h.OpenAt(c.at); got != c.open {
			t.Errorf("%s: open=%v, want %v", c.name, got, c.open)
		}
	}
}

func TestHoursReadTheInstantInTheOwnersZoneNotUTC(t *testing.T) {
	h := weekdayWindowHours(9*60, 17*60)
	utcMorning := time.Date(2026, 11, 9, 6, 30, 0, 0, time.UTC)
	if !h.OpenAt(utcMorning) {
		t.Fatalf("06:30 UTC is 09:30 in Nairobi and should be open")
	}
	utcLate := time.Date(2026, 11, 9, 14, 30, 0, 0, time.UTC)
	if h.OpenAt(utcLate) {
		t.Fatalf("14:30 UTC is 17:30 in Nairobi and should be closed")
	}
}

func TestHoursUntilMidnightUsesTheWholeLastMinute(t *testing.T) {
	h := AddressHours{Zone: "Africa/Nairobi", Mode: AddressModeSilent}
	h.Days[6] = &AddressDayWindow{FromMinute: 18 * 60, ToMinute: 1440}
	if !h.OpenAt(nairobiInNovember(15, 23, 59)) {
		t.Fatalf("sunday 23:59 should be open")
	}
	if h.OpenAt(nairobiInNovember(16, 0, 0)) {
		t.Fatalf("monday midnight has no window")
	}
}

func TestHoursWithAnUnknownZoneStayOpen(t *testing.T) {
	h := AddressHours{Zone: "Nowhere/Land", Mode: AddressModeClosed}
	h.Days[0] = &AddressDayWindow{FromMinute: 0, ToMinute: 60}
	if !h.OpenAt(nairobiInNovember(9, 12, 0)) {
		t.Fatalf("an unreadable zone must not close the address")
	}
}

func TestHoursAnyAndValidate(t *testing.T) {
	if (AddressHours{}).Any() {
		t.Fatalf("empty hours are not any hours")
	}
	if !weekdayWindowHours(60, 120).Any() {
		t.Fatalf("hours with windows are any hours")
	}
	if err := weekdayWindowHours(9*60, 17*60).Validate(); err != nil {
		t.Fatalf("good hours refused: %v", err)
	}
	bad := map[string]func(*AddressHours){
		"no zone":            func(h *AddressHours) { h.Zone = "" },
		"unknown zone":       func(h *AddressHours) { h.Zone = "Nowhere/Land" },
		"bad mode":           func(h *AddressHours) { h.Mode = "sleepy" },
		"opens below zero":   func(h *AddressHours) { h.Days[0].FromMinute = -1 },
		"opens at midnight+": func(h *AddressHours) { h.Days[0].FromMinute = 1440 },
		"closes before open": func(h *AddressHours) { h.Days[0].ToMinute = h.Days[0].FromMinute },
		"closes past day":    func(h *AddressHours) { h.Days[0].ToMinute = 1441 },
		"never opens":        func(h *AddressHours) { h.Days = [7]*AddressDayWindow{} },
	}
	for name, change := range bad {
		h := weekdayWindowHours(9*60, 17*60)
		change(&h)
		if err := h.Validate(); !errors.Is(err, ErrAddressBadSettings) {
			t.Errorf("%s: want bad settings, got %v", name, err)
		}
	}
}

func TestHoursMondayIsFirstAndSundayLast(t *testing.T) {
	cases := map[time.Weekday]int{time.Monday: 0, time.Tuesday: 1, time.Saturday: 5, time.Sunday: 6}
	for day, want := range cases {
		if got := addressDayIndex(day); got != want {
			t.Errorf("%v: index %d, want %d", day, got, want)
		}
	}
}
