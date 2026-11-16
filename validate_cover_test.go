package mwanachamacomm

import (
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestCheckRefusesARowMissingARequiredField(t *testing.T) {
	s, err := SpecFor("check")
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	if err := Check(s, roleAddress, models.Address{}); !errors.Is(err, models.ErrAddressBadSettings) {
		t.Fatalf("address without owner: %v", err)
	}
	if err := Check(s, roleNotification, models.Notification{}); !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("notification without owner: %v", err)
	}
	if err := Check(s, roleSubjectThread, models.SubjectThread{SubjectKind: "record"}); !errors.Is(err, models.ErrSubjectInvalid) {
		t.Fatalf("subject thread without subject id or audience: %v", err)
	}
}

func TestCheckRefusesAValueOutsideTheDeclaredList(t *testing.T) {
	s, _ := SpecFor("check")
	row := models.Notification{ActorID: "a1", Category: "gossip", SubjectID: "s1"}
	err := Check(s, roleNotification, row)
	if !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("an invented category passed: %v", err)
	}
	thread := models.SubjectThread{SubjectKind: "record", SubjectID: "r1", Audience: "everyone"}
	if err := Check(s, roleSubjectThread, thread); !errors.Is(err, models.ErrSubjectInvalid) {
		t.Fatalf("an invented audience passed: %v", err)
	}
}

func TestCheckHoldsTextToItsPattern(t *testing.T) {
	s, _ := SpecFor("check")
	if err := Check(s, roleAddress, models.Address{ActorID: "a1", PublicAddress: "not-an-address"}); !errors.Is(err, models.ErrAddressBadSettings) {
		t.Fatalf("a malformed public address passed: %v", err)
	}
	if err := Check(s, roleAddress, models.Address{ActorID: "a1", PublicAddress: "MKU4827YUT3391"}); err != nil {
		t.Fatalf("a canonical address refused: %v", err)
	}
	if err := Check(s, roleAddress, models.Address{ActorID: "a1"}); err != nil {
		t.Fatalf("an address with no public form refused: %v", err)
	}
}

func TestCheckAcceptsAWellFormedNotification(t *testing.T) {
	s, _ := SpecFor("check")
	row := models.Notification{
		ActorID:     "a1",
		Category:    models.CategoryCase,
		Event:       models.EventCaseClosed,
		SubjectKind: "case",
		SubjectID:   "c1",
		CreatedAt:   time.Now(),
	}
	if err := Check(s, roleNotification, row); err != nil {
		t.Fatalf("a good notification refused: %v", err)
	}
}

func TestCheckNamesARoleNothingFills(t *testing.T) {
	s, _ := SpecFor("check")
	if err := Check(s, "no_such_role", models.Address{}); err == nil {
		t.Fatalf("an unknown role passed")
	}
}

func TestInvalidFallsBackToAReferenceErrorForRolesWithoutTheirOwn(t *testing.T) {
	if !errors.Is(invalid(roleAddress), models.ErrAddressBadSettings) {
		t.Fatalf("address has its own invalid error")
	}
	if !errors.Is(invalid(roleChatThread), ErrInvalidReference) {
		t.Fatalf("chat thread should fall back to an invalid reference")
	}
}

func TestHoldsFindsExactValuesOnly(t *testing.T) {
	if !holds([]string{"closed", "silent"}, "silent") || holds([]string{"closed", "silent"}, "Silent") || holds(nil, "x") {
		t.Fatalf("holds is not an exact match")
	}
}

func TestTheBlueprintDeclaresTheSameVocabularyAsTheCode(t *testing.T) {
	s, _ := SpecFor("check")
	o, ok := s.ByRole(roleNotification)
	if !ok {
		t.Fatalf("no notification object")
	}
	declared := map[string][]string{}
	for _, f := range o.Fields {
		declared[f.Name] = f.Values
	}
	var events, categories, kinds []string
	for _, e := range models.NotificationEvents() {
		events = append(events, string(e))
	}
	for _, c := range models.NotificationCategories() {
		categories = append(categories, string(c))
	}
	for _, k := range models.NotificationSubjectKinds() {
		kinds = append(kinds, string(k))
	}
	for name, code := range map[string][]string{"event": events, "category": categories, "subject_kind": kinds} {
		a := append([]string(nil), declared[name]...)
		b := append([]string(nil), code...)
		sort.Strings(a)
		sort.Strings(b)
		if len(a) != len(b) {
			t.Errorf("%s: blueprint has %d values, code has %d", name, len(a), len(b))
			continue
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("%s: blueprint %q vs code %q", name, a[i], b[i])
				break
			}
		}
	}
}
