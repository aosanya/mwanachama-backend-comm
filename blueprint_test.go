package mwanachamacomm

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

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

// carriers is the Go value that fills each role. specstore.New holds the two
// to each other in both directions at construction, so this map plus
// TestEveryExampleFitsTheTypes is what replaced the row structs the
// conversion deleted.
var carriers = map[string]any{
	roleChatThread:             models.ChatThread{},
	roleChatMessage:            models.ChatMessage{},
	roleDMThread:               models.DMThread{},
	roleDMParticipant:          models.DMParticipant{},
	roleDMMessage:              models.DMMessage{},
	roleDMReaction:             models.DMReaction{},
	roleDMDeviceKey:            models.DMDeviceKey{},
	roleReport:                 models.Report{},
	roleRemoval:                models.Removal{},
	roleDismissal:              models.Dismissal{},
	roleDispute:                models.Dispute{},
	roleAddress:                models.Address{},
	roleAddressBlock:           models.AddressBlock{},
	roleNotification:           models.Notification{},
	roleNotificationPreference: models.NotificationPreference{},
}

func TestEveryDeclaredColumnIsCarried(t *testing.T) {
	s := testSpec(t)
	for role, carrier := range carriers {
		o, ok := s.ByRole(role)
		if !ok {
			t.Errorf("role %q fills nothing", role)
			continue
		}

		held := specstore.ColumnsOf(reflect.TypeOf(carrier))
		declared := map[string]bool{}
		for _, f := range o.Fields {
			declared[f.Name] = true
			if !held[f.Name] {
				t.Errorf("%s declares %q, which %T does not carry, so every read of it comes back empty",
					role, f.Name, carrier)
			}
		}
		for name := range held {
			if !declared[name] {
				t.Errorf("%T carries %q, which %s declares nowhere — tag it spec:\"-\" if it is derived",
					carrier, name, role)
			}
		}
	}
}

// TestEveryExampleFitsTheTypes builds a store over every spec the module
// ships, not just the one a test happened to load: drift under an unloaded
// domain is invisible otherwise.
func TestEveryExampleFitsTheTypes(t *testing.T) {
	for domain, s := range shippedSpecs(t) {
		db := provisionTestDB(t)
		if err := Provision(db, s); err != nil {
			t.Fatalf("%s: Provision: %v", domain, err)
		}
		if _, err := newStore(db, s, carriers); err != nil {
			t.Errorf("%s: the declared objects and the Go types disagree: %v", domain, err)
		}
	}
}

// commsOwnVocabulary is every value comm's own acts produce. Its moderation
// writes one removal receipt; everything else in a notification's vocabulary
// is raised by another module through NotificationRepository.Raise, so the
// module declares these and a domain declares the rest.
var commsOwnVocabulary = map[string][]string{
	roleNotification + ".category":           {"chat"},
	roleNotification + ".event":              {"message_removed"},
	roleNotification + ".subject_kind":       {"message"},
	roleNotificationPreference + ".category": {"chat"},
}

func TestVocabularyMatchesTheBlueprint(t *testing.T) {
	b, err := Blueprint()
	if err != nil {
		t.Fatalf("Blueprint: %v", err)
	}

	for key, want := range commsOwnVocabulary {
		role, field, _ := strings.Cut(key, ".")
		o, ok := b.Object(role)
		if !ok {
			t.Errorf("the blueprint declares no role %q", role)
			continue
		}
		got := valuesOfField(t, o, field)
		sorted := append([]string(nil), got...)
		expect := append([]string(nil), want...)
		sort.Strings(sorted)
		sort.Strings(expect)
		if !reflect.DeepEqual(sorted, expect) {
			t.Errorf("the blueprint declares %s.%s as %v, want only comm's own %v — every other value belongs to whichever module raises it, and is the domain's to declare",
				role, field, got, want)
		}
	}
}

func valuesOfField(t *testing.T, o spec.Object, field string) []string {
	t.Helper()
	for _, f := range o.Fields {
		if f.Name == field {
			return f.Values
		}
	}
	t.Fatalf("%s declares no field %q", o.Role, field)
	return nil
}

func TestCivicVocabularyMatchesTheGoConstants(t *testing.T) {
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
			t.Errorf("%s.%s: the civic spec declares %v, Go constants are %v — a stored value outlives a rename, so a drift here is a data bug rather than a compile error",
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

// The three hashes are key material, and the live gateway database holds
// them as bytea. SQLite is dynamically typed and stores whatever it is
// handed, so the column type has to be asserted on the emitted DDL rather
// than on a round trip.
func TestTheHashColumnsAreBytesNotText(t *testing.T) {
	s := testSpec(t)
	ddl := strings.Join(s.DDL("postgres"), "\n")

	for _, c := range []struct{ role, field string }{
		{roleAddress, "hash"},
		{roleAddressBlock, "hash"},
		{roleDMThread, "opened_via_address_hash"},
	} {
		o, ok := s.ByRole(c.role)
		if !ok {
			t.Fatalf("role %q fills nothing", c.role)
		}
		var declared spec.FieldType
		for _, f := range o.Fields {
			if f.Name == c.field {
				declared = f.Type
			}
		}
		if declared != spec.TypeBytes {
			t.Errorf("%s.%s is declared %q, want bytes — a text column cannot hold key material, and the live column is bytea",
				c.role, c.field, declared)
		}
		if !strings.Contains(ddl, c.field+" bytea") {
			t.Errorf("the Postgres DDL does not emit %s as bytea:\n%s", c.field, ddl)
		}
	}
}
