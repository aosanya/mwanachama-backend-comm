package mwanachamacomm

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type PushSender interface {
	Send(ctx context.Context, sub models.PushSubscription, payload []byte) error
}

type VAPID struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

const (
	EnvVAPIDPublicKey  = "VAPID_PUBLIC_KEY"
	EnvVAPIDPrivateKey = "VAPID_PRIVATE_KEY"
	EnvVAPIDSubject    = "VAPID_SUBJECT"
)

var ErrVAPIDInvalid = errors.New("push: invalid VAPID configuration")

func VAPIDFromEnv(getenv func(string) string) (VAPID, bool) {
	v := VAPID{
		PublicKey:  strings.TrimSpace(getenv(EnvVAPIDPublicKey)),
		PrivateKey: strings.TrimSpace(getenv(EnvVAPIDPrivateKey)),
		Subject:    strings.TrimSpace(getenv(EnvVAPIDSubject)),
	}
	return v, v.PublicKey != "" || v.PrivateKey != ""
}

func (v VAPID) Validate() error {
	priv, err := models.DecodePushKey(v.PrivateKey)
	if err != nil || len(priv) != 32 {
		return fmt.Errorf("%w: the private key must be a 32-byte P-256 scalar, base64url", ErrVAPIDInvalid)
	}
	pub, err := models.DecodePushKey(v.PublicKey)
	if err != nil || len(pub) != models.PushPublicKeyBytes {
		return fmt.Errorf("%w: the public key must be an uncompressed P-256 point, base64url", ErrVAPIDInvalid)
	}
	key, err := ecdh.P256().NewPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("%w: the private key is not on P-256", ErrVAPIDInvalid)
	}
	if !bytes.Equal(key.PublicKey().Bytes(), pub) {
		return fmt.Errorf("%w: the public key is not the private key's", ErrVAPIDInvalid)
	}
	if _, err := v.Contact(); err != nil {
		return err
	}
	return nil
}

const VAPIDSubjectSeparator = ","

func (v VAPID) Contact() (string, error) {
	contacts, err := v.Contacts()
	if err != nil {
		return "", err
	}
	return contacts[0], nil
}

func (v VAPID) Contacts() ([]string, error) {
	if strings.TrimSpace(v.Subject) == "" {
		return nil, fmt.Errorf("%w: %s names no contact", ErrVAPIDInvalid, EnvVAPIDSubject)
	}
	var contacts []string
	for _, entry := range strings.Split(v.Subject, VAPIDSubjectSeparator) {
		contact := strings.TrimSpace(entry)
		if !validContact(contact) {
			return nil, fmt.Errorf("%w: %q in %s is not a mailto: or https: contact", ErrVAPIDInvalid, contact, EnvVAPIDSubject)
		}
		contacts = append(contacts, contact)
	}
	return contacts, nil
}

func validContact(contact string) bool {
	if contact == "" || strings.ContainsAny(contact, " \t\r\n") {
		return false
	}
	u, err := url.Parse(contact)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "mailto":
		local, domain, ok := strings.Cut(u.Opaque, "@")
		return ok && local != "" && domain != "" && !strings.ContainsAny(domain, "@?") && u.RawQuery == ""
	case "https":
		return u.Opaque == "" && u.Host != "" && u.User == nil
	default:
		return false
	}
}

type WebPushSender struct {
	vapid   VAPID
	contact string
	client  webpush.HTTPClient
	ttl     time.Duration
}

const pushTTL = 24 * time.Hour

func NewWebPushSender(v VAPID, client webpush.HTTPClient) (*WebPushSender, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	contact, err := v.Contact()
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = publicOnlyClient()
	}
	return &WebPushSender{vapid: v, contact: contact, client: client, ttl: pushTTL}, nil
}

func (w *WebPushSender) Send(ctx context.Context, sub models.PushSubscription, payload []byte) error {
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.PublicKey, Auth: sub.AuthSecret},
	}, &webpush.Options{
		HTTPClient:      w.client,
		Subscriber:      strings.TrimPrefix(w.contact, "mailto:"),
		VAPIDPublicKey:  w.vapid.PublicKey,
		VAPIDPrivateKey: w.vapid.PrivateKey,
		TTL:             int(w.ttl.Seconds()),
		Urgency:         webpush.UrgencyNormal,
	})
	if err != nil {
		return fmt.Errorf("push: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return models.ErrPushSubscriptionGone
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	default:
		return fmt.Errorf("push: the push service answered %d", resp.StatusCode)
	}
}

func publicOnlyClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: refusePrivateAddress}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func refusePrivateAddress(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !publicAddress(ip) {
		return fmt.Errorf("push: refusing to reach %s, which is not a public address", host)
	}
	return nil
}

func publicAddress(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast())
}
