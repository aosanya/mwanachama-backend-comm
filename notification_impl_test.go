package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func newNotificationStore(t *testing.T) *mwanachamacomm.NotificationStore {
	t.Helper()
	db, tables := newTestDB(t)
	s, err := mwanachamacomm.NewNotificationStore(db, tables, monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("NewNotificationStore: %v", err)
	}
	return s
}

func TestNotificationRaiseAndList(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	n, err := s.Raise(ctx, models.Notification{
		MemberID:    "m-1",
		Category:    models.CategoryResults,
		Event:       models.EventResultsPublished,
		SubjectKind: models.SubjectSurvey,
		SubjectID:   "survey-1",
	})
	if err != nil {
		t.Fatalf("Raise: %v", err)
	}
	if n.ID == "" {
		t.Fatalf("Raise did not mint an id")
	}
	out, err := s.List(ctx, "m-1", 0)
	if err != nil || len(out) != 1 {
		t.Fatalf("List = %+v, err %v, want 1 row", out, err)
	}
	unread, err := s.UnreadCount(ctx, "m-1")
	if err != nil || unread != 1 {
		t.Fatalf("UnreadCount = %d, err %v, want 1", unread, err)
	}
}

func TestNotificationRaiseValidatesShape(t *testing.T) {
	s := newNotificationStore(t)
	_, err := s.Raise(context.Background(), models.Notification{
		MemberID:    "m-1",
		Category:    models.CategoryChat, // wrong category for this event
		Event:       models.EventResultsPublished,
		SubjectKind: models.SubjectSurvey,
		SubjectID:   "survey-1",
	})
	if !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid, got %v", err)
	}
}

func TestNotificationRaiseRefusesWhenMuted(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	if _, err := s.SetPreference(ctx, "m-1", models.CategoryResults, true); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	_, err := s.Raise(ctx, models.Notification{
		MemberID:    "m-1",
		Category:    models.CategoryResults,
		Event:       models.EventResultsPublished,
		SubjectKind: models.SubjectSurvey,
		SubjectID:   "survey-1",
	})
	if !errors.Is(err, models.ErrNotificationMuted) {
		t.Fatalf("expected ErrNotificationMuted, got %v", err)
	}
}

func TestNotificationReminderCapIsOnePerMemberPerSubject(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	raise := func() error {
		_, err := s.Raise(ctx, models.Notification{
			MemberID:    "m-1",
			Category:    models.CategorySurvey,
			Event:       models.EventSurveyReminder,
			SubjectKind: models.SubjectSurvey,
			SubjectID:   "survey-1",
		})
		return err
	}
	if err := raise(); err != nil {
		t.Fatalf("first reminder: %v", err)
	}
	if err := raise(); !errors.Is(err, models.ErrNotificationCapSpent) {
		t.Fatalf("second reminder: expected ErrNotificationCapSpent, got %v", err)
	}
}

func TestNotificationNudgeCapIsOnePerChapterPerSubject(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	raise := func(memberID string) error {
		_, err := s.Raise(ctx, models.Notification{
			MemberID:    memberID,
			ChapterID:   "c-1",
			Category:    models.CategorySurvey,
			Event:       models.EventSurveyNudge,
			SubjectKind: models.SubjectSurvey,
			SubjectID:   "survey-1",
		})
		return err
	}
	if err := raise("m-1"); err != nil {
		t.Fatalf("first nudge: %v", err)
	}
	// Same chapter+subject, different member: still capped, because the
	// nudge's cap is per (chapter, subject), not per member.
	if err := raise("m-2"); !errors.Is(err, models.ErrNotificationCapSpent) {
		t.Fatalf("second nudge: expected ErrNotificationCapSpent, got %v", err)
	}
}

func TestNotificationMarkReadIsScopedAndIdempotent(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	n1, _ := s.Raise(ctx, models.Notification{MemberID: "m-1", Category: models.CategoryResults, Event: models.EventResultsPublished, SubjectKind: models.SubjectSurvey, SubjectID: "s-1"})
	n2, _ := s.Raise(ctx, models.Notification{MemberID: "m-1", Category: models.CategoryResults, Event: models.EventResultsPublished, SubjectKind: models.SubjectSurvey, SubjectID: "s-2"})
	other, _ := s.Raise(ctx, models.Notification{MemberID: "m-2", Category: models.CategoryResults, Event: models.EventResultsPublished, SubjectKind: models.SubjectSurvey, SubjectID: "s-3"})

	marked, err := s.MarkRead(ctx, "m-1", []string{n1.ID, other.ID, "not-a-real-id"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if marked != 1 {
		t.Fatalf("MarkRead = %d, want 1 (only n1 is m-1's and unread)", marked)
	}
	unread, _ := s.UnreadCount(ctx, "m-1")
	if unread != 1 { // n2 still unread
		t.Fatalf("UnreadCount(m-1) = %d, want 1", unread)
	}
	// Retry is idempotent.
	marked, err = s.MarkRead(ctx, "m-1", []string{n1.ID}, time.Now().UTC())
	if err != nil || marked != 0 {
		t.Fatalf("retry MarkRead = %d, err %v, want 0", marked, err)
	}
	_ = n2
}

func TestNotificationMarkReadOverCapIsRefused(t *testing.T) {
	s := newNotificationStore(t)
	ids := make([]string, models.NotificationMaxMarkRead+1)
	_, err := s.MarkRead(context.Background(), "m-1", ids, time.Now().UTC())
	if !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid, got %v", err)
	}
}

func TestNotificationSetPreferenceRefusesExemptCategories(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	for _, c := range []models.NotificationCategory{models.CategorySurvey, models.CategorySecurity} {
		if _, err := s.SetPreference(ctx, "m-1", c, true); !errors.Is(err, models.ErrNotificationCategoryExempt) {
			t.Fatalf("SetPreference(%s): expected ErrNotificationCategoryExempt, got %v", c, err)
		}
	}
}

func TestNotificationSetPreferenceUpserts(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	p1, err := s.SetPreference(ctx, "m-1", models.CategoryChat, true)
	if err != nil {
		t.Fatalf("first SetPreference: %v", err)
	}
	muted, err := s.IsMuted(ctx, "m-1", models.CategoryChat)
	if err != nil || !muted {
		t.Fatalf("IsMuted = %v, err %v, want true", muted, err)
	}
	p2, err := s.SetPreference(ctx, "m-1", models.CategoryChat, false)
	if err != nil {
		t.Fatalf("second SetPreference: %v", err)
	}
	if p1.ID != p2.ID {
		t.Fatalf("upsert minted a new id: %q vs %q", p1.ID, p2.ID)
	}
	muted, err = s.IsMuted(ctx, "m-1", models.CategoryChat)
	if err != nil || muted {
		t.Fatalf("IsMuted after unmute = %v, err %v, want false", muted, err)
	}
	prefs, err := s.ListPreferences(ctx, "m-1")
	if err != nil || len(prefs) != 1 {
		t.Fatalf("ListPreferences = %+v, err %v, want 1 row", prefs, err)
	}
}

func TestNotificationIsMutedAbsentMeansOn(t *testing.T) {
	s := newNotificationStore(t)
	muted, err := s.IsMuted(context.Background(), "nobody-ever-set-a-preference", models.CategoryChat)
	if err != nil || muted {
		t.Fatalf("IsMuted for a member with no rows = %v, err %v, want false", muted, err)
	}
}
