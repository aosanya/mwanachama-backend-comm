package mwanachamacomm

import (
	"reflect"
	"sort"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func testSpec(t *testing.T) *spec.Spec {
	t.Helper()
	s, err := SpecFor("mwanachama")
	if err != nil {
		t.Fatalf("SpecFor: %v", err)
	}
	return s
}

func TestBlueprintDeclaresEveryRoleTheDomainSpecFills(t *testing.T) {
	s := testSpec(t)
	if len(s.Objects) != 15 {
		t.Fatalf("declared %d objects, want comm's 15 tables", len(s.Objects))
	}
	for _, role := range allRoles {
		if _, ok := s.ByRole(role); !ok {
			t.Errorf("no object fills role %q", role)
		}
	}
}

func TestEveryDeclaredNameFitsPostgres(t *testing.T) {
	s := testSpec(t)
	for _, o := range s.Objects {
		if n := s.TableFor(o); len(n) > spec.MaxIdentifier {
			t.Errorf("table %q is %d bytes, over the %d Postgres truncates at", n, len(n), spec.MaxIdentifier)
		}
		for _, idx := range o.Indexes {
			if n := s.IndexFor(o, idx); len(n) > spec.MaxIdentifier {
				t.Errorf("index %q is %d bytes, over the %d Postgres truncates at", n, len(n), spec.MaxIdentifier)
			}
		}
	}
}

func TestEveryObjectAndFieldCarriesADescription(t *testing.T) {
	s := testSpec(t)
	for _, o := range s.Objects {
		if o.Description == "" {
			t.Errorf("object %q has no description", o.Role)
		}
		for _, f := range o.Fields {
			if f.Description == "" {
				t.Errorf("%s.%s has no description", o.Role, f.Name)
			}
		}
	}
}

// legacyRows is what each role's columns were before the conversion. The
// blueprint was transcribed from these, so holding the two to each other is
// what proves nothing was dropped on the way.
var legacyRows = map[string]any{
	roleChatThread:             gormstore.ChatThreadRow{},
	roleChatMessage:            gormstore.ChatMessageRow{},
	roleDMThread:               gormstore.DMThreadRow{},
	roleDMParticipant:          gormstore.DMParticipantRow{},
	roleDMMessage:              gormstore.DMMessageRow{},
	roleDMReaction:             gormstore.DMReactionRow{},
	roleDMDeviceKey:            gormstore.DMDeviceKeyRow{},
	roleReport:                 gormstore.ReportRow{},
	roleRemoval:                gormstore.RemovalRow{},
	roleDismissal:              gormstore.DismissalRow{},
	roleDispute:                gormstore.DisputeRow{},
	roleAddress:                gormstore.AddressRow{},
	roleAddressBlock:           gormstore.AddressBlockRow{},
	roleNotification:           gormstore.NotificationRow{},
	roleNotificationPreference: gormstore.NotificationPreferenceRow{},
}

// renamedColumns is every column the conversion deliberately renames, legacy
// name to declared name. The row structs said chapter/member because the
// gateway's original tables did; the domain types have said structure/actor
// since, and the translation lived in gormstore's ToRow/FromRow pairs. With
// those pairs deleted the declared name is the Go field's name, so the column
// has to move with them.
var renamedColumns = map[string]string{
	"chapter_id":        "structure_id",
	"member_id":         "actor_id",
	"review_chapter_id": "review_structure_id",
	"seat_chapter_id":   "seat_structure_id",
	"author_member_id":  "author_actor_id",
}

func TestEveryLegacyColumnHasADeclaredHome(t *testing.T) {
	s := testSpec(t)
	for role, row := range legacyRows {
		o, ok := s.ByRole(role)
		if !ok {
			t.Errorf("role %q fills nothing", role)
			continue
		}

		declared := map[string]bool{}
		for _, f := range o.Fields {
			declared[f.Name] = true
		}

		legacy := map[string]bool{}
		rt := reflect.TypeOf(row)
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if f.PkgPath != "" {
				continue
			}
			name := specstore.ColumnName(f.Name)
			if to, renamed := renamedColumns[name]; renamed {
				name = to
			}
			legacy[name] = true
			if !declared[name] {
				t.Errorf("%s: legacy column %q is declared nowhere, so the conversion would drop it", role, name)
			}
		}
		for name := range declared {
			if !legacy[name] {
				t.Errorf("%s: declares %q, which no legacy column feeds, so every existing row reads it back empty", role, name)
			}
		}
	}
}

func TestVocabularyMatchesTheBlueprint(t *testing.T) {
	s := testSpec(t)

	cases := []struct {
		role, field string
		constants   []string
	}{
		{roleNotification, "category", categoryStrings()},
		{roleNotification, "event", eventStrings()},
		{roleNotification, "subject_kind", subjectKindStrings()},
		{roleNotificationPreference, "category", categoryStrings()},
		{roleDMParticipant, "state", []string{
			string(models.DMStateInvited), string(models.DMStateActive),
			string(models.DMStateLeft), string(models.DMStateKicked),
		}},
		{roleDispute, "state", []string{
			string(models.DisputeOpen), string(models.DisputeReinstated), string(models.DisputeUpheld),
		}},
		{roleReport, "reason", []string{
			string(models.ReportReasonThreats), string(models.ReportReasonAbuse),
			string(models.ReportReasonFalseClaim), string(models.ReportReasonSpam),
		}},
		{roleRemoval, "reason", removalReasonStrings()},
		{roleDismissal, "reason", removalReasonStrings()},
	}

	for _, c := range cases {
		declared := declaredValues(t, s, c.role, c.field)
		want := append([]string(nil), c.constants...)
		sort.Strings(declared)
		sort.Strings(want)
		if !reflect.DeepEqual(declared, want) {
			t.Errorf("%s.%s: blueprint declares %v, Go constants are %v — a stored value outlives a rename, so a drift here is a data bug rather than a compile error",
				c.role, c.field, declared, want)
		}
	}
}

func declaredValues(t *testing.T, s *spec.Spec, role, field string) []string {
	t.Helper()
	o, ok := s.ByRole(role)
	if !ok {
		t.Fatalf("role %q fills nothing", role)
	}
	for _, f := range o.Fields {
		if f.Name == field {
			return append([]string(nil), f.Values...)
		}
	}
	t.Fatalf("%s declares no field %q", role, field)
	return nil
}

func categoryStrings() []string {
	cs := models.NotificationCategories()
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, string(c))
	}
	return out
}

func eventStrings() []string {
	out := make([]string, 0, len(models.NotificationEvents()))
	for _, e := range models.NotificationEvents() {
		out = append(out, string(e))
	}
	return out
}

func subjectKindStrings() []string {
	out := make([]string, 0, len(models.NotificationSubjectKinds()))
	for _, k := range models.NotificationSubjectKinds() {
		out = append(out, string(k))
	}
	return out
}

func removalReasonStrings() []string {
	return []string{
		string(models.RemovalReasonThreats),
		string(models.RemovalReasonAbuse),
		string(models.RemovalReasonFalseClaim),
	}
}
