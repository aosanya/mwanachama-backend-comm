package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func datedStanding(start time.Time, dates ...string) models.StandingMessage {
	m := monthlyStanding(start)
	m.Rule = ""
	m.Dates = dates
	return m
}

func TestAStandingMessageOnAListOfDatesIsSentOnEachDateOnly(t *testing.T) {
	ctx := context.Background()
	st := newStandingStore(t)
	start := time.Date(2026, 1, 5, 6, 0, 0, 0, time.UTC)
	created, err := st.CreateStanding(ctx, datedStanding(start, "2026-01-05", "2026-05-04", "2026-09-07"))
	if err != nil {
		t.Fatal(err)
	}
	read, err := st.GetStanding(ctx, "g1", created.ID)
	if err != nil || len(read.Dates) != 3 || read.Dates[1] != "2026-05-04" || read.Rule != "" {
		t.Fatalf("dates did not round-trip: %v, %+v", err, read)
	}

	if sent, err := st.SendDueStandingMessages(ctx, start.Add(time.Hour)); err != nil || len(sent) != 1 {
		t.Fatalf("first date: %v, %+v", err, sent)
	}
	if sent, err := st.SendDueStandingMessages(ctx, start.AddDate(0, 2, 0)); err != nil || len(sent) != 0 {
		t.Fatalf("a month between dates sent something: %v, %+v", err, sent)
	}
	may := time.Date(2026, 5, 4, 6, 0, 0, 0, time.UTC)
	if sent, err := st.SendDueStandingMessages(ctx, may.Add(time.Minute)); err != nil || len(sent) != 1 {
		t.Fatalf("second date: %v, %+v", err, sent)
	}
	if sent, err := st.SendDueStandingMessages(ctx, may.Add(time.Hour)); err != nil || len(sent) != 0 {
		t.Fatalf("a date went out twice: %v, %+v", err, sent)
	}
	if sent, err := st.SendDueStandingMessages(ctx, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil || len(sent) != 1 {
		t.Fatalf("last date: %v, %+v", err, sent)
	}
	if sent, err := st.SendDueStandingMessages(ctx, time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil || len(sent) != 0 {
		t.Fatalf("sent past the last date: %v, %+v", err, sent)
	}
}

func TestAStandingScheduleIsARuleOrDatesExactlyOne(t *testing.T) {
	st := newStandingStore(t)
	start := time.Date(2026, 1, 5, 6, 0, 0, 0, time.UTC)
	both := datedStanding(start, "2026-01-05")
	both.Rule = "FREQ=MONTHLY"
	neither := datedStanding(start)
	notADate := datedStanding(start, "2026-02-30")
	malformed := datedStanding(start, "5 May")
	for name, m := range map[string]models.StandingMessage{"both": both, "neither": neither, "not a date": notADate, "malformed": malformed} {
		if _, err := st.CreateStanding(context.Background(), m); !errors.Is(err, models.ErrStandingInvalid) {
			t.Fatalf("%s: err = %v, want ErrStandingInvalid", name, err)
		}
	}
}
