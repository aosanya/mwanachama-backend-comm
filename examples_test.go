package mwanachamacomm

import (
	"path/filepath"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func shippedSpecs(t *testing.T) map[string]*spec.Spec {
	t.Helper()
	out := map[string]*spec.Spec{"civic": testSpec(t)}
	school, err := LoadSpec(filepath.Join(".", "spec", "examples", "school.comm.json"))
	if err != nil {
		t.Fatalf("load the school spec: %v", err)
	}
	out["school"] = school
	return out
}

func TestASecondDomainFillsEveryRoleWithItsOwnNouns(t *testing.T) {
	school := shippedSpecs(t)["school"]

	if len(school.Objects) != len(allRoles) {
		t.Fatalf("the school spec fills %d roles, want comm's %d", len(school.Objects), len(allRoles))
	}
	for _, role := range allRoles {
		o, ok := school.ByRole(role)
		if !ok {
			t.Errorf("the school spec fills no %q", role)
			continue
		}
		if o.Name == role {
			t.Errorf("role %q is filled by an object of the same name, so the second domain does not exercise neutrality", role)
		}
	}
}

func TestADomainSuppliesADefaultTheModuleDoesNotDeclare(t *testing.T) {
	specs := shippedSpecs(t)

	civic, ok := specs["civic"].ByRole(roleAddress)
	if !ok {
		t.Fatal("civic fills no address")
	}
	school, ok := specs["school"].ByRole(roleAddress)
	if !ok {
		t.Fatal("school fills no address")
	}

	if got := defaultOf(t, civic, "expiry_mode"); got != "" {
		t.Errorf("civic declares a default of %q on expiry_mode, and the module declares none", got)
	}
	if got := defaultOf(t, school, "expiry_mode"); got != "closed" {
		t.Errorf("school's expiry_mode default is %q, want \"closed\" — a default is the one thing a domain may set on a declared field", got)
	}
}

func defaultOf(t *testing.T, o spec.Object, field string) string {
	t.Helper()
	for _, f := range o.Fields {
		if f.Name == field {
			return f.Default
		}
	}
	t.Fatalf("%s declares no field %q", o.Role, field)
	return ""
}

func TestADomainCannotRedeclareAFieldTheModuleOwns(t *testing.T) {
	raw := []byte(`{
	  "module": "comm",
	  "domain": "rogue",
	  "instance": "rogue",
	  "objects": [
	    {"role": "notification", "name": "alert", "table": "alerts",
	     "description": "A notification whose domain tries to reopen the category vocabulary.",
	     "fields": [{"name": "category", "type": "enum", "values": ["anything"],
	                 "description": "Whatever this domain likes."}]}
	  ]
	}`)
	if _, err := ParseSpec(raw); err == nil {
		t.Fatal("want a refusal: a domain may set a default on a declared field and nothing else")
	}
}

func TestTwoDomainsCoexist(t *testing.T) {
	db := provisionTestDB(t)
	specs := shippedSpecs(t)

	for domain, s := range specs {
		if err := Provision(db, s); err != nil {
			t.Fatalf("provision %s: %v", domain, err)
		}
	}

	seen := map[string]string{}
	for domain, s := range specs {
		for _, o := range s.Objects {
			table := s.TableFor(o)
			if !db.Migrator().HasTable(table) {
				t.Errorf("%s: %s (role %q) was not created", domain, table, o.Role)
			}
			if other, clash := seen[table]; clash {
				t.Errorf("%s and %s both land role %q in %s, so one domain reads the other's rows",
					other, domain, o.Role, table)
			}
			seen[table] = domain
		}
	}
}

func TestRequiredFieldsHaveNoDefault(t *testing.T) {
	for domain, s := range shippedSpecs(t) {
		for _, o := range s.Objects {
			for _, f := range o.Fields {
				if f.Required && f.Default != "" {
					t.Errorf("%s: %s.%s is required and defaults to %v, so an omitted value passes the check instead of failing it",
						domain, o.Role, f.Name, f.Default)
				}
			}
		}
	}
}
