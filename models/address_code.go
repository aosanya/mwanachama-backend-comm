package models

import (
	"crypto/sha256"
	"math/big"
	"strings"
)

// AddressDomainSeparator prefixes the hash input so an address can never
// collide with a digest this project computes for some other purpose over
// the same key. Versioned: changing the derivation means changing this
// string, and every address derived under the old one keeps resolving
// because the gateway stores what it was told and re-derives with the
// version the row was written under.
const AddressDomainSeparator = "mwanachama:actor-address:v1"

const (
	addressLetters   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	addressLettersN  = 26
	addressLetterRun = 3
	addressDigitRun  = 4
)

// AddressDerive computes the canonical address for a public key. The key is
// taken as the opaque string the device published — whatever encoding that
// is, it is hashed verbatim, so the two sides cannot disagree about
// padding.
func AddressDerive(publicKey string) (string, error) {
	if strings.TrimSpace(publicKey) == "" {
		return "", ErrAddressMalformed
	}
	sum := sha256.Sum256([]byte(AddressDomainSeparator + "\x00" + publicKey))

	// 128 bits of digest reduced into a 2^54.8 space — the modulo bias is
	// far below anything observable, and taking the whole digest would not
	// change a single address anyone ever reads.
	n := new(big.Int).SetBytes(sum[:16])

	var b strings.Builder
	b.Grow(2*addressLetterRun + 2*addressDigitRun)
	emitAddressLetters(&b, n)
	emitAddressDigits(&b, n)
	emitAddressLetters(&b, n)
	emitAddressDigits(&b, n)
	return b.String(), nil
}

func emitAddressLetters(b *strings.Builder, n *big.Int) {
	m := new(big.Int)
	for i := 0; i < addressLetterRun; i++ {
		n.QuoRem(n, big.NewInt(addressLettersN), m)
		b.WriteByte(addressLetters[m.Int64()])
	}
}

func emitAddressDigits(b *strings.Builder, n *big.Int) {
	m := new(big.Int)
	for i := 0; i < addressDigitRun; i++ {
		n.QuoRem(n, big.NewInt(10), m)
		b.WriteByte(byte('0' + m.Int64()))
	}
}

// AddressNormalize accepts an address the way a person typed it — spaced,
// hyphenated, lower-case — and returns the canonical fourteen-character
// form. A person reading `MKU 4827 YUT 3391` off a screen and typing it
// back must not be told they got it wrong because of the spaces the screen
// put there.
func AddressNormalize(s string) (string, error) {
	var b strings.Builder
	b.Grow(2*addressLetterRun + 2*addressDigitRun)
	for _, r := range s {
		switch {
		case r == ' ' || r == '-' || r == '\t' || r == '_':
			continue
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 32)
		default:
			b.WriteRune(r)
		}
	}
	out := b.String()
	if !AddressValid(out) {
		return "", ErrAddressMalformed
	}
	return out, nil
}

// AddressValid reports whether s is exactly the canonical form.
func AddressValid(s string) bool {
	if len(s) != 2*addressLetterRun+2*addressDigitRun {
		return false
	}
	isAlpha := func(c byte) bool { return c >= 'A' && c <= 'Z' }
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	for i := 0; i < addressLetterRun; i++ {
		if !isAlpha(s[i]) {
			return false
		}
	}
	for i := addressLetterRun; i < addressLetterRun+addressDigitRun; i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	for i := addressLetterRun + addressDigitRun; i < 2*addressLetterRun+addressDigitRun; i++ {
		if !isAlpha(s[i]) {
			return false
		}
	}
	for i := 2*addressLetterRun + addressDigitRun; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// AddressFormat groups the canonical form for display: `MKU 4827 YUT 3391`.
func AddressFormat(s string) string {
	if !AddressValid(s) {
		return s
	}
	a, b := addressLetterRun, addressLetterRun+addressDigitRun
	c := b + addressLetterRun
	return s[:a] + " " + s[a:b] + " " + s[b:c] + " " + s[c:]
}
