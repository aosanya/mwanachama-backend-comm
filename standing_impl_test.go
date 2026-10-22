package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func newStandingStore(t *testing.T) *mwanachamacomm.StandingStore {
	t.Helper()
	db, s := newTestDB(t)
	st, err := mwanachamacomm.NewStandingStore(db, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func monthlyStanding(start time.Time) models.StandingMessage {
	return models.StandingMessage{
		StructureID: "g1",
		AuthorID:    "a1",
		Body:        "Bring your questions to the monthly meeting.",
		Rule:        "FREQ=MONTHLY",
		Zone:        "Africa/Nairobi",
		StartsAt:    start,
	}
}

func TestAStandingMessageIsSentOncePerOccurrence(t *testing.T) {
	ctx := context.Background()
	st := newStandingStore(t)
	start := time.Date(2026, 1, 5, 6, 0, 0, 0, time.UTC)
	if _, err := st.CreateStanding(ctx, monthlyStanding(start)); err != nil {
		t.Fatal(err)
	}

	sent, err := st.SendDueStandingMessages(ctx, start.Add(time.Hour))
	if err != nil || len(sent) != 1 || sent[0].StructureID != "g1" {
		t.Fatalf("first occurrence: %v, %+v", err, sent)
	}
	again, err := st.SendDueStandingMessages(ctx, start.Add(2*time.Hour))
	if err != nil || len(again) != 0 {
		t.Fatalf("an occurrence went out twice: %v, %+v", err, again)
	}
	next, err := st.SendDueStandingMessages(ctx, start.AddDate(0, 1, 0).Add(time.Minute))
	if err != nil || len(next) != 1 {
		t.Fatalf("second occurrence: %v, %+v", err, next)
	}
}

func TestAStoppedStandingMessageSendsNothingMore(t *testing.T) {
	ctx := context.Background()
	st := newStandingStore(t)
	start := time.Date(2026, 1, 5, 6, 0, 0, 0, time.UTC)
	m, err := st.CreateStanding(ctx, monthlyStanding(start))
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := st.StopStanding(ctx, "g1", m.ID, "a2")
	if err != nil || !stopped.Stopped() || stopped.StoppedBy != "a2" {
		t.Fatalf("stop: %v, %+v", err, stopped)
	}
	if _, err := st.StopStanding(ctx, "g1", m.ID, "a2"); !errors.Is(err, models.ErrStandingStopped) {
		t.Fatalf("second stop: %v", err)
	}
	sent, err := st.SendDueStandingMessages(ctx, start.Add(time.Hour))
	if err != nil || len(sent) != 0 {
		t.Fatalf("a stopped message was sent: %v, %+v", err, sent)
	}
}

func TestAStandingMessageEndsAtItsEnd(t *testing.T) {
	ctx := context.Background()
	st := newStandingStore(t)
	start := time.Date(2026, 1, 5, 6, 0, 0, 0, time.UTC)
	m := monthlyStanding(start)
	end := start.Add(24 * time.Hour)
	m.EndsAt = &end
	if _, err := st.CreateStanding(ctx, m); err != nil {
		t.Fatal(err)
	}
	if sent, _ := st.SendDueStandingMessages(ctx, start.Add(time.Hour)); len(sent) != 1 {
		t.Fatalf("first occurrence: %+v", sent)
	}
	if sent, _ := st.SendDueStandingMessages(ctx, start.AddDate(0, 2, 0)); len(sent) != 0 {
		t.Fatalf("sent after its end: %+v", sent)
	}
}

func TestAStandingMessageRefusesABadSchedule(t *testing.T) {
	st := newStandingStore(t)
	m := monthlyStanding(time.Now())
	m.Rule = "FREQ=SOMETIMES"
	if _, err := st.CreateStanding(context.Background(), m); !errors.Is(err, models.ErrStandingInvalid) {
		t.Fatalf("a bad rule was accepted: %v", err)
	}
	m = monthlyStanding(time.Now())
	m.Zone = "Nowhere/Atlantis"
	if _, err := st.CreateStanding(context.Background(), m); !errors.Is(err, models.ErrStandingInvalid) {
		t.Fatalf("a bad zone was accepted: %v", err)
	}
}
