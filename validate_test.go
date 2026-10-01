package mwanachamacomm

import (
	"errors"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestCheckReadsTheRulesOffTheSpec(t *testing.T) {
	s := testSpec(t)

	t.Run("required is read off the spec", func(t *testing.T) {
		err := Check(s, roleNotification, models.Notification{SubjectID: "x"})
		if !errors.Is(err, models.ErrNotificationInvalid) {
			t.Fatalf("empty actor_id: got %v", err)
		}
	})

	t.Run("enum values are read off the spec", func(t *testing.T) {
		err := Check(s, roleNotification, models.Notification{
			ActorID: "a", SubjectID: "s", Category: "not-a-category",
		})
		if !errors.Is(err, models.ErrNotificationInvalid) {
			t.Fatalf("bad category: got %v", err)
		}
	})

	t.Run("a widened domain value is accepted", func(t *testing.T) {
		school := shippedSpecs(t)["school"]
		err := Check(school, roleNotificationPreference, models.NotificationPreference{
			ActorID: "a", Category: "attendance", ChangedAt: time.Now(),
		})
		if err != nil {
			t.Fatalf("school's own category was refused: %v", err)
		}
		if err := Check(s, roleNotificationPreference, models.NotificationPreference{
			ActorID: "a", Category: "attendance", ChangedAt: time.Now(),
		}); err == nil {
			t.Fatal("civic accepted school's category")
		}
	})

	t.Run("matches is read off the spec", func(t *testing.T) {
		err := Check(s, roleAddress, models.Address{
			ActorID: "a", Hash: []byte("ab"), PublicAddress: "not an address",
		})
		if !errors.Is(err, models.ErrAddressBadSettings) {
			t.Fatalf("bad public_address: got %v", err)
		}
	})

	t.Run("an unsupplied pattern is an error, not a silent pass", func(t *testing.T) {
		delete(patterns, "address")
		defer func() { patterns["address"] = models.AddressValid }()
		err := Check(s, roleAddress, models.Address{ActorID: "a", Hash: []byte("ab"), PublicAddress: "x"})
		if err == nil {
			t.Fatal("a spec naming a pattern nobody supplies passed silently")
		}
	})
}
