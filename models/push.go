package models

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

var ErrPushSubscriptionInvalid = errors.New("push: invalid subscription")

var ErrPushSubscriptionGone = errors.New("push: the push service no longer knows this subscription")

const (
	PushPublicKeyBytes  = 65
	PushAuthSecretBytes = 16
)

type PushKeys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}

type PushSubscription struct {
	ID         string    `json:"id"`
	ActorID    string    `json:"actor_id"`
	DeviceID   string    `json:"device_id,omitempty"`
	Endpoint   string    `json:"endpoint"`
	PublicKey  string    `json:"-"`
	AuthSecret string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}

func (p PushSubscription) Validate() error {
	if strings.TrimSpace(p.ActorID) == "" {
		return fmt.Errorf("%w: a subscription belongs to somebody", ErrPushSubscriptionInvalid)
	}
	if err := validPushEndpoint(p.Endpoint); err != nil {
		return err
	}
	if n, err := decodedLength(p.PublicKey); err != nil || n != PushPublicKeyBytes {
		return fmt.Errorf("%w: keys.p256dh must be an uncompressed P-256 point, base64url", ErrPushSubscriptionInvalid)
	}
	if n, err := decodedLength(p.AuthSecret); err != nil || n != PushAuthSecretBytes {
		return fmt.Errorf("%w: keys.auth must be a 16-byte secret, base64url", ErrPushSubscriptionInvalid)
	}
	return nil
}

func validPushEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("%w: endpoint must be an https URL", ErrPushSubscriptionInvalid)
	}
	host := u.Hostname()
	if net.ParseIP(host) != nil || strings.EqualFold(host, "localhost") || !strings.Contains(host, ".") {
		return fmt.Errorf("%w: endpoint must name a push service by its host name", ErrPushSubscriptionInvalid)
	}
	return nil
}

func DecodePushKey(s string) ([]byte, error) {
	trimmed := strings.TrimRight(s, "=")
	if b, err := base64.RawURLEncoding.DecodeString(trimmed); err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(trimmed)
}

func decodedLength(s string) (int, error) {
	b, err := DecodePushKey(s)
	return len(b), err
}

type PushDeliveryState string

const (
	PushDeliverySent   PushDeliveryState = "sent"
	PushDeliveryFailed PushDeliveryState = "failed"
	PushDeliveryGone   PushDeliveryState = "gone"
)

type PushDelivery struct {
	ID             string            `json:"id"`
	NotificationID string            `json:"notification_id"`
	ActorID        string            `json:"actor_id"`
	SubscriptionID string            `json:"subscription_id"`
	State          PushDeliveryState `json:"state"`
	At             time.Time         `json:"at"`
}
