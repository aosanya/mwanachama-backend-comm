package mwanachamacomm

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// domainWords are words that mean something in one domain and nothing in
// another. A reader from a completely different domain — a school, a clinic,
// a union — would not recognise them, so a module that stores "a
// conversation" and "a notice" must not name them.
var domainWords = []string{
	"chapter", "member", "citizen", "constituency", "ward", "civic",
	"survey", "contribution", "merchandise", "enrollment", "handout",
	"ballot", "voter", "petition", "mwanachama",
}

// exemptFiles are the files the scan does not walk, each for a reason that
// is written down rather than assumed.
var exemptFiles = map[string]string{
	// The civic notification vocabulary, which CM25 moved out of the
	// blueprint and CM26 will move out of Go. Until the event-to-category
	// pairing can be declared, these constants are the only place that
	// mapping can live, so the file is exempt by name and the exemption
	// shrinks to nothing when CM26 lands.
	"models/notification_category.go": "CM26",

	// The legacy table and column names Provision migrates *away from*.
	// They have to say chapter and member because that is what the live
	// gateway database calls them; renaming them here would break adoption.
	"provision.go": "the legacy names adoption reads",
}

// exemptIdentifiers is the civic notification vocabulary where it is
// re-exported from the root package, which is the same CM26 set as
// models/notification_category.go and goes with it. Named one by one rather
// than by file, so the rest of doc.go is still scanned.
var exemptIdentifiers = map[string]bool{
	"CategorySurvey": true, "CategoryContribution": true, "CategoryMerchandise": true,
	"EventSurveyReached": true, "EventSurveyReminder": true, "EventSurveyNudge": true,
	"EventContributionMatched": true, "EventContributionOrphaned": true,
	"EventContributionReportRefused": true, "EventMerchandiseRecorded": true,
	"EventEnrollmentKeyRotated": true,
	"SubjectSurvey":             true, "SubjectContribution": true, "SubjectContributionReport": true,
	"SubjectHandout": true, "SubjectEnrollmentKey": true,
}

func TestNoIdentifierCarriesADomainWord(t *testing.T) {
	for _, dir := range []string{".", "models", "routes"} {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, dir, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", dir, err)
		}
		for _, pkg := range pkgs {
			for path, file := range pkg.Files {
				rel := filepath.ToSlash(path)
				if strings.HasSuffix(rel, "_test.go") {
					continue
				}
				if why, exempt := exemptFiles[rel]; exempt {
					t.Logf("%s is exempt: %s", rel, why)
					continue
				}
				walkIdentifiers(t, rel, file)
			}
		}
	}
}

func walkIdentifiers(t *testing.T, path string, file *ast.File) {
	t.Helper()
	report := func(kind, name string) {
		if exemptIdentifiers[name] {
			return
		}
		if word, carries := carriesDomainWord(name); carries {
			t.Errorf("%s: the %s %q carries the domain word %q — a word that means something in one domain and nothing in another does not belong in the module",
				path, kind, name, word)
		}
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch d := n.(type) {
		case *ast.TypeSpec:
			report("type", d.Name.Name)
		case *ast.FuncDecl:
			report("function", d.Name.Name)
		case *ast.ValueSpec:
			for _, name := range d.Names {
				report("declaration", name.Name)
			}
		case *ast.Field:
			for _, name := range d.Names {
				report("field", name.Name)
			}
		}
		return true
	})
}

func carriesDomainWord(name string) (string, bool) {
	lower := strings.ToLower(name)
	for _, word := range domainWords {
		if strings.Contains(lower, word) {
			return word, true
		}
	}
	return "", false
}

// A stored enum value outlives a rename, which makes it the worst version of
// this failure: a drifted identifier is a compile error, a drifted stored
// value is a data bug. These are the module's own declared values, so a
// domain word here would be one every instance is stuck with.
func TestNoStoredValueTheModuleDeclaresCarriesADomainWord(t *testing.T) {
	b, err := Blueprint()
	if err != nil {
		t.Fatalf("Blueprint: %v", err)
	}
	for _, o := range b.Objects {
		if word, carries := carriesDomainWord(o.Role); carries {
			t.Errorf("the role %q carries the domain word %q", o.Role, word)
		}
		for _, f := range o.Fields {
			if word, carries := carriesDomainWord(f.Name); carries {
				t.Errorf("%s.%s carries the domain word %q", o.Role, f.Name, word)
			}
			for _, v := range f.Values {
				if word, carries := carriesDomainWord(v); carries {
					t.Errorf("%s.%s declares the value %q, which carries the domain word %q — a domain widens an enum, so this one belongs in the domain spec",
						o.Role, f.Name, v, word)
				}
			}
			if f.Default == "" {
				continue
			}
			if word, carries := carriesDomainWord(f.Default); carries {
				t.Errorf("%s.%s defaults to %q, which carries the domain word %q", o.Role, f.Name, f.Default, word)
			}
		}
	}
}

// Every gating action and every declared address is a name a mounting
// process and a permissions grant both read, so neither may carry a domain
// word either.
func TestNoDeclaredAddressOrActionCarriesADomainWord(t *testing.T) {
	ops, err := operationsSpec()
	if err != nil {
		t.Fatalf("parse the operations: %v", err)
	}
	for name, op := range ops {
		if word, carries := carriesDomainWord(name); carries {
			t.Errorf("the operation %q carries the domain word %q", name, word)
		}
		if word, carries := carriesDomainWord(op.action); carries {
			t.Errorf("%s's action %q carries the domain word %q", name, op.action, word)
		}
		if word, carries := carriesDomainWord(op.path); carries {
			t.Errorf("%s's path %q carries the domain word %q", name, op.path, word)
		}
	}
}

type declaredOp struct {
	action string
	path   string
}

func operationsSpec() (map[string]declaredOp, error) {
	var doc struct {
		Operations map[string]struct {
			Action string `json:"action"`
			Path   string `json:"path"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(Operations(), &doc); err != nil {
		return nil, err
	}
	out := make(map[string]declaredOp, len(doc.Operations))
	for name, op := range doc.Operations {
		out[name] = declaredOp{action: op.Action, path: op.Path}
	}
	return out, nil
}
