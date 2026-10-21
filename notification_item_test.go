package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func itemNotice(event models.NotificationEvent, actor string, kind models.NotificationSubjectKind, subjectID string) mwanachamacomm.Notification {
	return mwanachamacomm.Notification{
		ActorID:     actor,
		Category:    models.CategoryItem,
		Event:       event,
		SubjectKind: kind,
		SubjectID:   subjectID,
	}
}

func TestEachItemNoticeRaisesToItsRaiserUnderItem(t *testing.T) {
	events := []models.NotificationEvent{
		models.EventItemAcknowledged, models.EventItemAnswered,
		models.EventItemStatusChanged, models.EventItemFiledForYou,
	}
	s := newNotificationStore(t)
	ctx := context.Background()
	for _, e := range events {
		if _, err := s.Raise(ctx, itemNotice(e, "resident-1", models.SubjectItem, "entry-1")); err != nil {
			t.Fatalf("Raise %s: %v", e, err)
		}
	}
	out, err := s.List(ctx, "resident-1", 0)
	if err != nil || len(out) != len(events) {
		t.Fatalf("List = %d notices, err %v, want %d", len(out), err, len(events))
	}
}

func TestAnItemNoticeOutsideItemIsRefused(t *testing.T) {
	s := newNotificationStore(t)
	bad := itemNotice(models.EventItemAnswered, "resident-1", models.SubjectItem, "entry-1")
	bad.Category = models.CategoryMembership
	if _, err := s.Raise(context.Background(), bad); !errors.Is(err, mwanachamacomm.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid, got %v", err)
	}
}

func TestTheOverdueDigestReachesAnOwnerOncePerDay(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	digest := func(owner, day string) error {
		_, err := s.Raise(ctx, itemNotice(models.EventItemsOverdue, owner, models.SubjectItemDigest, day))
		return err
	}
	if err := digest("owner-1", "2026-10-21"); err != nil {
		t.Fatalf("first digest: %v", err)
	}
	if err := digest("owner-1", "2026-10-21"); !errors.Is(err, mwanachamacomm.ErrNotificationCapSpent) {
		t.Fatalf("second digest the same day: expected ErrNotificationCapSpent, got %v", err)
	}
	if err := digest("owner-2", "2026-10-21"); err != nil {
		t.Fatalf("another owner the same day: %v", err)
	}
	if err := digest("owner-1", "2026-10-22"); err != nil {
		t.Fatalf("the next day: %v", err)
	}
}

func TestMutingItemSilencesItsNotices(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	if _, err := s.SetPreference(ctx, "resident-1", models.CategoryItem, true); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	_, err := s.Raise(ctx, itemNotice(models.EventItemAnswered, "resident-1", models.SubjectItem, "entry-1"))
	if !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
		t.Fatalf("expected ErrNotificationMuted, got %v", err)
	}
}
