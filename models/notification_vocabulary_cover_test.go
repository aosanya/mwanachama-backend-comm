package models

import (
	"errors"
	"sort"
	"testing"
)

func TestEveryEventBelongsToAKnownCategory(t *testing.T) {
	events := NotificationEvents()
	if len(events) == 0 {
		t.Fatalf("no events")
	}
	if !sort.SliceIsSorted(events, func(i, j int) bool { return events[i] < events[j] }) {
		t.Fatalf("events are not sorted: %v", events)
	}
	for _, e := range events {
		c, ok := NotificationCategoryOf(e)
		if !ok || !IsNotificationCategory(c) {
			t.Errorf("event %q has category %q, known=%v", e, c, ok)
		}
	}
}

func TestUnknownEventHasNoCategoryAndIsMutable(t *testing.T) {
	if _, ok := NotificationCategoryOf("no_such_event"); ok {
		t.Fatalf("an unknown event found a category")
	}
	if NotificationEventUnmutable("no_such_event") {
		t.Fatalf("an unknown event cannot be unmutable")
	}
}

func TestCaseClosedCannotBeMutedButBelongsToTheCaseCategory(t *testing.T) {
	if !NotificationEventUnmutable(EventCaseClosed) {
		t.Fatalf("case closed must always reach the person")
	}
	if c, _ := NotificationCategoryOf(EventCaseClosed); c != CategoryCase {
		t.Fatalf("case closed is in %q", c)
	}
}

func TestOnlySurveyAndSecurityAreExemptFromPreferences(t *testing.T) {
	for _, c := range NotificationCategories() {
		want := c == CategorySurvey || c == CategorySecurity
		if got := IsNotificationCategoryExempt(c); got != want {
			t.Errorf("category %q exempt=%v want %v", c, got, want)
		}
	}
	if IsNotificationCategory("gossip") {
		t.Fatalf("an invented category was accepted")
	}
}

func TestCategoryListsAreCopiesTheCallerCannotCorrupt(t *testing.T) {
	cats := NotificationCategories()
	first := cats[0]
	cats[0] = "tampered"
	if NotificationCategories()[0] != first {
		t.Fatalf("editing the returned list changed the vocabulary")
	}
	kinds := NotificationSubjectKinds()
	firstKind := kinds[0]
	kinds[0] = "tampered"
	if NotificationSubjectKinds()[0] != firstKind {
		t.Fatalf("editing the returned kinds changed the vocabulary")
	}
}

func TestParseNotificationRulesReadsAWellFormedList(t *testing.T) {
	rules, err := ParseNotificationRules([]any{
		map[string]any{"event": "case_closed", "told": []any{"owner"}, "fallback": []any{"admin"}, "mutable": false},
		map[string]any{"event": "post_added", "told": []any{"member"}, "fallback": []any{}, "mutable": true},
	})
	if err != nil || len(rules) != 2 {
		t.Fatalf("got %v, %v", rules, err)
	}
	if rules[0].Event != "case_closed" || rules[0].Told[0] != "owner" || rules[0].Fallback[0] != "admin" || rules[0].Mutable {
		t.Fatalf("first rule read wrong: %+v", rules[0])
	}
	if !rules[1].Mutable {
		t.Fatalf("second rule should be mutable")
	}
}

func TestParseNotificationRulesTakesNothingAsNoRules(t *testing.T) {
	rules, err := ParseNotificationRules(nil)
	if err != nil || rules == nil || len(rules) != 0 {
		t.Fatalf("nil gave %v, %v", rules, err)
	}
}

func TestParseNotificationRulesRefusesWhatCannotBeRules(t *testing.T) {
	bad := map[string]any{
		"not a list":         "case_closed",
		"empty event":        []any{map[string]any{"event": "", "told": []any{"owner"}}},
		"event named twice":  []any{map[string]any{"event": "a"}, map[string]any{"event": "a"}},
		"empty told role":    []any{map[string]any{"event": "a", "told": []any{""}}},
		"empty fallback":     []any{map[string]any{"event": "a", "fallback": []any{"ok", ""}}},
		"unmarshalable":      make(chan int),
		"wrong field shapes": []any{map[string]any{"event": 5}},
	}
	for name, in := range bad {
		if _, err := ParseNotificationRules(in); !errors.Is(err, ErrNotificationRuleInvalid) {
			t.Errorf("%s: want invalid, got %v", name, err)
		}
	}
}

func TestStandingMessageStoppedOnceItHasAStopTime(t *testing.T) {
	m := StandingMessage{ID: "s1"}
	if m.Stopped() {
		t.Fatalf("a running message reads as stopped")
	}
	m.StoppedAt = at(stateNow)
	if !m.Stopped() {
		t.Fatalf("a stopped message reads as running")
	}
}
