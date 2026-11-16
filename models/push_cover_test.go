package models

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

func pushKey(n int) string { return base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, n)) }

func goodSubscription() PushSubscription {
	return PushSubscription{
		ID:         "p1",
		ActorID:    "actor-1",
		Endpoint:   "https://push.example.com/send/abc",
		PublicKey:  pushKey(PushPublicKeyBytes),
		AuthSecret: pushKey(PushAuthSecretBytes),
	}
}

func TestPushSubscriptionValidAsTheBrowserSentIt(t *testing.T) {
	if err := goodSubscription().Validate(); err != nil {
		t.Fatalf("good subscription refused: %v", err)
	}
}

func TestPushSubscriptionRefusedWhenAnyPartIsWrong(t *testing.T) {
	cases := map[string]func(*PushSubscription){
		"nobody":              func(p *PushSubscription) { p.ActorID = "  " },
		"plain http":          func(p *PushSubscription) { p.Endpoint = "http://push.example.com/x" },
		"not a url":           func(p *PushSubscription) { p.Endpoint = "%%%" },
		"no host":             func(p *PushSubscription) { p.Endpoint = "https:///path" },
		"credentials in url":  func(p *PushSubscription) { p.Endpoint = "https://user:pw@push.example.com/x" },
		"an ip address":       func(p *PushSubscription) { p.Endpoint = "https://10.0.0.5/x" },
		"localhost":           func(p *PushSubscription) { p.Endpoint = "https://localhost/x" },
		"bare host":           func(p *PushSubscription) { p.Endpoint = "https://pushservice/x" },
		"short public key":    func(p *PushSubscription) { p.PublicKey = pushKey(64) },
		"public key not b64":  func(p *PushSubscription) { p.PublicKey = "!!!" },
		"short auth secret":   func(p *PushSubscription) { p.AuthSecret = pushKey(15) },
		"auth secret not b64": func(p *PushSubscription) { p.AuthSecret = "***" },
	}
	for name, change := range cases {
		p := goodSubscription()
		change(&p)
		if err := p.Validate(); !errors.Is(err, ErrPushSubscriptionInvalid) {
			t.Errorf("%s: want invalid, got %v", name, err)
		}
	}
}

func TestDecodePushKeyReadsEveryBase64SpellingABrowserUses(t *testing.T) {
	raw := []byte{0xfb, 0xff, 0xfe, 0x01}
	spellings := []string{
		base64.RawURLEncoding.EncodeToString(raw),
		base64.URLEncoding.EncodeToString(raw),
		base64.RawStdEncoding.EncodeToString(raw),
		base64.StdEncoding.EncodeToString(raw),
	}
	for _, s := range spellings {
		got, err := DecodePushKey(s)
		if err != nil || !bytes.Equal(got, raw) {
			t.Errorf("%q decoded to %v, %v", s, got, err)
		}
	}
	if _, err := DecodePushKey("not base64 at all!"); err == nil {
		t.Fatalf("garbage decoded")
	}
}
