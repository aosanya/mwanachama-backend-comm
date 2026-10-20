package mwanachamacomm_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func throwawayVAPID(t *testing.T) mwanachamacomm.VAPID {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return mwanachamacomm.VAPID{
		PublicKey:  base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		PrivateKey: base64.RawURLEncoding.EncodeToString(key.Bytes()),
		Subject:    "mailto:push@example.org",
	}
}

type pushed struct {
	header http.Header
	body   []byte
}

func pushService(t *testing.T, status int) (*httptest.Server, chan pushed) {
	t.Helper()
	got := make(chan pushed, 1)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- pushed{header: r.Header.Clone(), body: body}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func TestAPushIsEncryptedToTheBrowserAndSignedWithTheDeploymentsKey(t *testing.T) {
	vapid := throwawayVAPID(t)
	browser := newThrowawayBrowser(t)
	srv, got := pushService(t, http.StatusCreated)
	sender, err := mwanachamacomm.NewWebPushSender(vapid, srv.Client())
	if err != nil {
		t.Fatalf("NewWebPushSender: %v", err)
	}

	message := []byte(`{"id":"n-1","event":"results_published"}`)
	err = sender.Send(context.Background(), models.PushSubscription{
		Endpoint: srv.URL + "/send/abc", PublicKey: browser.PublicKey, AuthSecret: browser.AuthSecret,
	}, message)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	req := <-got

	if req.header.Get("Content-Encoding") != "aes128gcm" || req.header.Get("TTL") == "" {
		t.Fatalf("headers = %v, want an aes128gcm body with a TTL", req.header)
	}
	verifyVAPID(t, req.header.Get("Authorization"), vapid, srv.URL)

	plain := decryptAES128GCM(t, browser, req.body)
	if !bytes.Equal(plain, message) {
		t.Fatalf("the browser decrypts %q, want %q", plain, message)
	}
}

func TestAPushServiceAnsweringGoneOrNotFoundMeansGone(t *testing.T) {
	for status, want := range map[int]error{
		http.StatusGone:                models.ErrPushSubscriptionGone,
		http.StatusNotFound:            models.ErrPushSubscriptionGone,
		http.StatusCreated:             nil,
		http.StatusTooManyRequests:     errors.New("transient"),
		http.StatusInternalServerError: errors.New("transient"),
	} {
		srv, got := pushService(t, status)
		sender, err := mwanachamacomm.NewWebPushSender(throwawayVAPID(t), srv.Client())
		if err != nil {
			t.Fatal(err)
		}
		browser := newThrowawayBrowser(t)
		err = sender.Send(context.Background(), models.PushSubscription{
			Endpoint: srv.URL, PublicKey: browser.PublicKey, AuthSecret: browser.AuthSecret,
		}, []byte("{}"))
		<-got
		switch {
		case want == nil && err != nil:
			t.Errorf("%d: err = %v, want delivered", status, err)
		case errors.Is(want, models.ErrPushSubscriptionGone) && !errors.Is(err, models.ErrPushSubscriptionGone):
			t.Errorf("%d: err = %v, want gone", status, err)
		case want != nil && !errors.Is(want, models.ErrPushSubscriptionGone) && (err == nil || errors.Is(err, models.ErrPushSubscriptionGone)):
			t.Errorf("%d: err = %v, want a failure that keeps the subscription", status, err)
		}
	}
}

func TestAVAPIDPairIsCheckedBeforeAnythingIsSent(t *testing.T) {
	good := throwawayVAPID(t)
	other := throwawayVAPID(t)
	for name, v := range map[string]mwanachamacomm.VAPID{
		"mismatched pair":  {PublicKey: good.PublicKey, PrivateKey: other.PrivateKey, Subject: good.Subject},
		"no private key":   {PublicKey: good.PublicKey, Subject: good.Subject},
		"short public key": {PublicKey: "AAAA", PrivateKey: good.PrivateKey, Subject: good.Subject},
		"no subject":       {PublicKey: good.PublicKey, PrivateKey: good.PrivateKey},
		"a bare address":   {PublicKey: good.PublicKey, PrivateKey: good.PrivateKey, Subject: "push@example.org"},
		"blank subject":    {PublicKey: good.PublicKey, PrivateKey: good.PrivateKey, Subject: " , "},
		"http contact":     {PublicKey: good.PublicKey, PrivateKey: good.PrivateKey, Subject: "http://example.org/contact"},
		"bad second entry": {PublicKey: good.PublicKey, PrivateKey: good.PrivateKey, Subject: "mailto:push@example.org, push@example.org"},
		"empty entry":      {PublicKey: good.PublicKey, PrivateKey: good.PrivateKey, Subject: "mailto:push@example.org,,https://example.org"},
		"mailto no domain": {PublicKey: good.PublicKey, PrivateKey: good.PrivateKey, Subject: "mailto:push@"},
		"https no host":    {PublicKey: good.PublicKey, PrivateKey: good.PrivateKey, Subject: "https:example.org"},
	} {
		if _, err := mwanachamacomm.NewWebPushSender(v, nil); !errors.Is(err, mwanachamacomm.ErrVAPIDInvalid) {
			t.Errorf("%s: err = %v, want ErrVAPIDInvalid", name, err)
		}
	}
	if _, err := mwanachamacomm.NewWebPushSender(good, nil); err != nil {
		t.Fatalf("a matching pair was refused: %v", err)
	}
}

func TestVAPIDIsReadFromTheEnvironment(t *testing.T) {
	good := throwawayVAPID(t)
	env := map[string]string{
		mwanachamacomm.EnvVAPIDPublicKey:  good.PublicKey,
		mwanachamacomm.EnvVAPIDPrivateKey: " " + good.PrivateKey + "\n",
		mwanachamacomm.EnvVAPIDSubject:    good.Subject,
	}
	v, ok := mwanachamacomm.VAPIDFromEnv(func(k string) string { return env[k] })
	if !ok || v != good {
		t.Fatalf("VAPIDFromEnv = %+v, %v; want the pair as set", v, ok)
	}
	keysOnly := map[string]string{
		mwanachamacomm.EnvVAPIDPublicKey:  good.PublicKey,
		mwanachamacomm.EnvVAPIDPrivateKey: good.PrivateKey,
	}
	unset, ok := mwanachamacomm.VAPIDFromEnv(func(k string) string { return keysOnly[k] })
	if !ok || unset.Subject != "" {
		t.Fatalf("VAPIDFromEnv = %+v, %v; want the pair with no subject supplied for it", unset, ok)
	}
	if _, err := mwanachamacomm.NewWebPushSender(unset, nil); !errors.Is(err, mwanachamacomm.ErrVAPIDInvalid) {
		t.Fatalf("a pair with no %s built a sender: %v", mwanachamacomm.EnvVAPIDSubject, err)
	}
	if _, ok := mwanachamacomm.VAPIDFromEnv(func(string) string { return "" }); ok {
		t.Fatalf("an unset environment reported a VAPID pair")
	}
}

func TestAPushNamesTheFirstContactTheDeploymentLists(t *testing.T) {
	vapid := throwawayVAPID(t)
	vapid.Subject = "https://example.org/push-contact , mailto:push@example.org"
	contacts, err := vapid.Contacts()
	if err != nil || len(contacts) != 2 || contacts[0] != "https://example.org/push-contact" || contacts[1] != "mailto:push@example.org" {
		t.Fatalf("Contacts = %v, %v; want both, in the order configured", contacts, err)
	}
	browser := newThrowawayBrowser(t)
	srv, got := pushService(t, http.StatusCreated)
	sender, err := mwanachamacomm.NewWebPushSender(vapid, srv.Client())
	if err != nil {
		t.Fatalf("NewWebPushSender: %v", err)
	}
	if err := sender.Send(context.Background(), models.PushSubscription{
		Endpoint: srv.URL, PublicKey: browser.PublicKey, AuthSecret: browser.AuthSecret,
	}, []byte("{}")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	req := <-got
	verifyVAPID(t, req.header.Get("Authorization"), vapid, srv.URL)
}

func TestAMailtoContactReachesThePushServiceOnce(t *testing.T) {
	vapid := throwawayVAPID(t)
	vapid.Subject = "mailto:push@example.org,https://example.org/push-contact"
	browser := newThrowawayBrowser(t)
	srv, got := pushService(t, http.StatusCreated)
	sender, err := mwanachamacomm.NewWebPushSender(vapid, srv.Client())
	if err != nil {
		t.Fatalf("NewWebPushSender: %v", err)
	}
	if err := sender.Send(context.Background(), models.PushSubscription{
		Endpoint: srv.URL, PublicKey: browser.PublicKey, AuthSecret: browser.AuthSecret,
	}, []byte("{}")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	req := <-got
	verifyVAPID(t, req.header.Get("Authorization"), vapid, srv.URL)
}

func verifyVAPID(t *testing.T, header string, vapid mwanachamacomm.VAPID, origin string) {
	t.Helper()
	rest, ok := strings.CutPrefix(header, "vapid t=")
	if !ok {
		t.Fatalf("Authorization = %q, want the vapid scheme", header)
	}
	token, k, ok := strings.Cut(rest, ", k=")
	if !ok || k != vapid.PublicKey {
		t.Fatalf("Authorization names k=%q, want the deployment's public key", k)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(vapid.PublicKey)
	x, y := elliptic.Unmarshal(elliptic.P256(), raw)
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return pub, nil },
		jwt.WithValidMethods([]string{"ES256"})); err != nil {
		t.Fatalf("the VAPID token does not verify against the public key: %v", err)
	}
	contact, err := vapid.Contact()
	if err != nil {
		t.Fatalf("Contact: %v", err)
	}
	if claims["aud"] != origin || claims["sub"] != contact {
		t.Fatalf("claims = %v, want aud %s and sub %s", claims, origin, contact)
	}
}

func decryptAES128GCM(t *testing.T, browser throwawayBrowser, body []byte) []byte {
	t.Helper()
	if len(body) < 21 {
		t.Fatalf("a %d-byte body has no aes128gcm header", len(body))
	}
	salt := body[:16]
	_ = binary.BigEndian.Uint32(body[16:20])
	idlen := int(body[20])
	senderKey := body[21 : 21+idlen]
	ciphertext := body[21+idlen:]

	peer, err := ecdh.P256().NewPublicKey(senderKey)
	if err != nil {
		t.Fatalf("the sender's key: %v", err)
	}
	shared, err := browser.key.ECDH(peer)
	if err != nil {
		t.Fatal(err)
	}
	prkKey, _ := hkdf.Extract(sha256.New, shared, browser.auth)
	keyInfo := "WebPush: info\x00" + string(browser.key.PublicKey().Bytes()) + string(senderKey)
	ikm, _ := hkdf.Expand(sha256.New, prkKey, keyInfo, 32)
	prk, _ := hkdf.Extract(sha256.New, ikm, salt)
	cek, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)

	block, err := aes.NewCipher(cek)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	padded, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("the browser cannot decrypt the push: %v", err)
	}
	end := bytes.LastIndexByte(padded, 0x02)
	if end < 0 || new(big.Int).SetBytes(padded[end+1:]).Sign() != 0 {
		t.Fatalf("the record has no final-record delimiter")
	}
	return padded[:end]
}
