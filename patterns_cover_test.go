package mwanachamacomm

import "testing"

func TestSubjectKindPatternWantsLowerSnakeCase(t *testing.T) {
	match := patterns["subject_kind"]
	for _, ok := range []string{"record", "group_join_request", "a1", "x_"} {
		if !match(ok) {
			t.Errorf("%q should match", ok)
		}
	}
	for _, bad := range []string{"", "Record", "1record", "_record", "has space", "dash-ed", string(make([]byte, 65))} {
		if match(bad) {
			t.Errorf("%q should not match", bad)
		}
	}
}

func TestSubjectKindPatternAllowsSixtyFourCharactersNoMore(t *testing.T) {
	match := patterns["subject_kind"]
	long := "a"
	for len(long) < 64 {
		long += "b"
	}
	if !match(long) || match(long+"b") {
		t.Fatalf("the length limit is not 64")
	}
}

func TestAttachmentIDPatternAllowsStorageStyleIDs(t *testing.T) {
	match := patterns["attachment_id"]
	for _, ok := range []string{"a", "9f", "file:1.png", "bucket_a-b.c:d"} {
		if !match(ok) {
			t.Errorf("%q should match", ok)
		}
	}
	for _, bad := range []string{"", ".hidden", "-lead", "has space", "slash/inside"} {
		if match(bad) {
			t.Errorf("%q should not match", bad)
		}
	}
}

func TestAddressPatternIsTheCanonicalAddressRule(t *testing.T) {
	match := patterns["address"]
	if !match("MKU4827YUT3391") || match("mku4827yut3391") || match("MKU 4827 YUT 3391") {
		t.Fatalf("the address pattern is not the canonical form")
	}
}
