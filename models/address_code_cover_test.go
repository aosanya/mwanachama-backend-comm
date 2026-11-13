package models

import (
	"errors"
	"testing"
)

func TestAddressDeriveIsStableAndCanonical(t *testing.T) {
	a, err := AddressDerive("key-one")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := AddressDerive("key-one")
	if a != b {
		t.Fatalf("same key derived %q then %q", a, b)
	}
	if !AddressValid(a) {
		t.Fatalf("derived address %q is not canonical", a)
	}
	other, _ := AddressDerive("key-two")
	if other == a {
		t.Fatalf("different keys derived the same address %q", a)
	}
}

func TestAddressDeriveRefusesABlankKey(t *testing.T) {
	for _, key := range []string{"", "   ", "\t\n"} {
		if _, err := AddressDerive(key); !errors.Is(err, ErrAddressMalformed) {
			t.Fatalf("key %q: want ErrAddressMalformed, got %v", key, err)
		}
	}
}

func TestAddressNormalizeAcceptsHowAPersonTypesIt(t *testing.T) {
	for _, typed := range []string{"MKU 4827 YUT 3391", "mku-4827-yut-3391", "mku_4827\tyut 3391", "MKU4827YUT3391"} {
		got, err := AddressNormalize(typed)
		if err != nil || got != "MKU4827YUT3391" {
			t.Fatalf("%q: got %q, %v", typed, got, err)
		}
	}
}

func TestAddressNormalizeRefusesMalformed(t *testing.T) {
	for _, typed := range []string{"", "MKU4827YUT339", "MKU4827YUT33911", "4827MKU3391YUT", "MKU4827YUT339X", "MK!4827YUT3391"} {
		if _, err := AddressNormalize(typed); !errors.Is(err, ErrAddressMalformed) {
			t.Fatalf("%q: want ErrAddressMalformed, got %v", typed, err)
		}
	}
}

func TestAddressValidIsExactlyTheCanonicalForm(t *testing.T) {
	cases := map[string]bool{
		"MKU4827YUT3391":    true,
		"mku4827yut3391":    false,
		"MKU 4827 YUT 3391": false,
		"MK14827YUT3391":    false,
		"MKU48A7YUT3391":    false,
		"MKU4827Y1T3391":    false,
		"MKU4827YUT33A1":    false,
		"":                  false,
	}
	for in, want := range cases {
		if got := AddressValid(in); got != want {
			t.Errorf("AddressValid(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestAddressFormatGroupsOnlyACanonicalAddress(t *testing.T) {
	if got := AddressFormat("MKU4827YUT3391"); got != "MKU 4827 YUT 3391" {
		t.Fatalf("got %q", got)
	}
	if got := AddressFormat("not an address"); got != "not an address" {
		t.Fatalf("non-canonical input changed to %q", got)
	}
}

func TestAddressFormatRoundTripsThroughNormalize(t *testing.T) {
	a, _ := AddressDerive("round-trip")
	back, err := AddressNormalize(AddressFormat(a))
	if err != nil || back != a {
		t.Fatalf("got %q, %v want %q", back, err, a)
	}
}
