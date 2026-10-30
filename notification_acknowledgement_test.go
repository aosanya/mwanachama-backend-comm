package mwanachamacomm_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func raiseInstruction(t *testing.T, s *mwanachamacomm.NotificationStore, subjectID string, recipients ...string) map[string]models.Notification {
	t.Helper()
	out := map[string]models.Notification{}
	for _, actor := range recipients {
		n, err := s.Raise(context.Background(), models.Notification{
			ActorID: actor, Category: models.CategoryItem, Event: models.EventItemFiledForYou,
			SubjectKind: models.SubjectItem, SubjectID: subjectID,
		})
		if err != nil {
			t.Fatalf("Raise to %s: %v", actor, err)
		}
		out[actor] = n
	}
	return out
}

func TestAcknowledgingANotificationRecordsTheRecipientAndReadsIt(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	raised := raiseInstruction(t, s, "instruction-1", "ward-1")

	ack, err := s.AcknowledgeNotification(ctx, "ward-1", raised["ward-1"].ID)
	if err != nil {
		t.Fatalf("AcknowledgeNotification: %v", err)
	}
	if ack.NotificationID != raised["ward-1"].ID || ack.ActorID != "ward-1" || ack.AcknowledgedAt.IsZero() {
		t.Fatalf("acknowledgement = %+v", ack)
	}
	unread, err := s.UnreadCount(ctx, "ward-1")
	if err != nil || unread != 0 {
		t.Fatalf("unread after acknowledging = %d, err %v, want 0", unread, err)
	}
}

func TestAcknowledgingTwiceKeepsTheFirstAcknowledgement(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	raised := raiseInstruction(t, s, "instruction-1", "ward-1")

	first, err := s.AcknowledgeNotification(ctx, "ward-1", raised["ward-1"].ID)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := s.AcknowledgeNotification(ctx, "ward-1", raised["ward-1"].ID)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.ID != first.ID || !second.AcknowledgedAt.Equal(first.AcknowledgedAt) {
		t.Fatalf("second = %+v, want the first %+v", second, first)
	}
	got, err := s.NotificationReceipts(ctx, models.SubjectItem, "instruction-1", "", "", 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("receipts = %+v, err %v, want one", got, err)
	}
}

func TestOnlyTheRecipientCanAcknowledgeANotification(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	raised := raiseInstruction(t, s, "instruction-1", "ward-1", "ward-2")

	if _, err := s.AcknowledgeNotification(ctx, "ward-2", raised["ward-1"].ID); !errors.Is(err, models.ErrNotificationNotFound) {
		t.Fatalf("acknowledging somebody else's notification: %v, want ErrNotificationNotFound", err)
	}
	if _, err := s.AcknowledgeNotification(ctx, "ward-1", "no-such-notification"); !errors.Is(err, models.ErrNotificationNotFound) {
		t.Fatalf("acknowledging an unknown notification: %v, want ErrNotificationNotFound", err)
	}
	got, err := s.NotificationReceipts(ctx, models.SubjectItem, "instruction-1", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if r.AcknowledgedAt != nil {
			t.Fatalf("a refused acknowledgement left %+v", r)
		}
	}
}

func TestReceiptsSayWhichRecipientsAcknowledgedAndWhichWentQuiet(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	raised := raiseInstruction(t, s, "instruction-1", "ward-1", "ward-2", "ward-3")
	raiseInstruction(t, s, "instruction-2", "ward-1")

	if _, err := s.AcknowledgeNotification(ctx, "ward-1", raised["ward-1"].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkRead(ctx, "ward-2", []string{raised["ward-2"].ID}, raised["ward-2"].CreatedAt); err != nil {
		t.Fatal(err)
	}

	got, err := s.NotificationReceipts(ctx, models.SubjectItem, "instruction-1", "", "", 0)
	if err != nil {
		t.Fatalf("NotificationReceipts: %v", err)
	}
	byActor := map[string]models.NotificationReceipt{}
	for _, r := range got {
		byActor[r.ActorID] = r
	}
	if len(got) != 3 {
		t.Fatalf("receipts = %+v, want the three recipients of instruction-1 and no other subject's", got)
	}
	if r := byActor["ward-1"]; r.AcknowledgedAt == nil || r.ReadAt == nil || r.NotificationID != raised["ward-1"].ID {
		t.Fatalf("ward-1 = %+v, want acknowledged and read", r)
	}
	if r := byActor["ward-2"]; r.AcknowledgedAt != nil || r.ReadAt == nil {
		t.Fatalf("ward-2 = %+v, want read and not acknowledged", r)
	}
	if r := byActor["ward-3"]; r.AcknowledgedAt != nil || r.ReadAt != nil {
		t.Fatalf("ward-3 = %+v, want neither read nor acknowledged", r)
	}
}

func TestReceiptsCanBeNarrowedToOneEvent(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	raiseInstruction(t, s, "instruction-1", "ward-1")
	if _, err := s.Raise(ctx, models.Notification{
		ActorID: "ward-1", Category: models.CategoryItem, Event: models.EventItemAnswered,
		SubjectKind: models.SubjectItem, SubjectID: "instruction-1",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := s.NotificationReceipts(ctx, models.SubjectItem, "instruction-1", string(models.EventItemAnswered), "", 0)
	if err != nil || len(got) != 1 || got[0].Event != models.EventItemAnswered {
		t.Fatalf("receipts for one event = %+v, err %v", got, err)
	}
}

func TestReceiptsArePagedByCursor(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	var recipients []string
	for i := 0; i < 5; i++ {
		recipients = append(recipients, fmt.Sprintf("ward-%d", i))
	}
	raiseInstruction(t, s, "instruction-1", recipients...)

	seen := map[string]bool{}
	after := ""
	for page := 0; page < 10; page++ {
		got, err := s.NotificationReceipts(ctx, models.SubjectItem, "instruction-1", "", after, 2)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if len(got) > 2 {
			t.Fatalf("page %d holds %d, want at most 2", page, len(got))
		}
		if len(got) == 0 {
			break
		}
		for _, r := range got {
			if seen[r.NotificationID] {
				t.Fatalf("%s came twice", r.NotificationID)
			}
			seen[r.NotificationID] = true
		}
		after = got[len(got)-1].NotificationID
	}
	if len(seen) != 5 {
		t.Fatalf("paging reached %d recipients, want 5", len(seen))
	}
}

func TestReceiptsNeedASubject(t *testing.T) {
	s := newNotificationStore(t)
	for _, c := range [][2]string{{"", "x"}, {"item", ""}, {" ", " "}} {
		if _, err := s.NotificationReceipts(context.Background(), models.NotificationSubjectKind(c[0]), c[1], "", "", 0); !errors.Is(err, models.ErrNotificationInvalid) {
			t.Fatalf("receipts for (%q, %q): %v, want ErrNotificationInvalid", c[0], c[1], err)
		}
	}
}
