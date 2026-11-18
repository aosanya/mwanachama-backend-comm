package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func onlyReportHandOvers(e models.NotificationEvent) bool {
	return e == models.EventReportHandedOver
}

func handOver(m *mwanachamacomm.CommManager, caller, recipient string) (models.Notification, error) {
	return m.RaiseNotificationFor(context.Background(), caller, recipient, models.CategoryReport, models.EventReportHandedOver, models.SubjectReport, "report-1", map[string]string{"report": "Annual report"})
}

func TestAReportHandedOverIsKeptForOnePersonAndNobodyElsesList(t *testing.T) {
	m := newPushManager(t)
	m.UseRaisable(onlyReportHandOvers)
	ctx := context.Background()
	if _, err := handOver(m, "officer-1", "councillor-1"); err != nil {
		t.Fatalf("hand over: %v", err)
	}
	mine, err := m.NotificationStore.List(ctx, "councillor-1", 0)
	if err != nil || len(mine) != 1 || mine[0].Event != models.EventReportHandedOver || mine[0].SubjectKind != models.SubjectReport || mine[0].SubjectID != "report-1" || mine[0].Names["report"] != "Annual report" {
		t.Fatalf("recipient list: %+v %v", mine, err)
	}
	for _, other := range []string{"officer-1", "councillor-2"} {
		got, err := m.NotificationStore.List(ctx, other, 0)
		if err != nil || len(got) != 0 {
			t.Fatalf("%s list: %+v %v", other, got, err)
		}
	}
}

func TestAManagerGivenNoRaisableEventRefusesEveryHandOver(t *testing.T) {
	m := newPushManager(t)
	_, err := handOver(m, "officer-1", "councillor-1")
	if !errors.Is(err, mwanachamacomm.ErrNotificationNotRaisable) {
		t.Fatalf("expected ErrNotificationNotRaisable, got %v", err)
	}
}

func TestAnEventTheHostDoesNotNameIsRefusedEvenWhenItIsDeclared(t *testing.T) {
	m := newPushManager(t)
	m.UseRaisable(onlyReportHandOvers)
	_, err := m.RaiseNotificationFor(context.Background(), "officer-1", "councillor-1", models.CategorySecurity, models.EventDeviceSignedOut, models.SubjectDevice, "device-1", nil)
	if !errors.Is(err, mwanachamacomm.ErrNotificationNotRaisable) {
		t.Fatalf("expected ErrNotificationNotRaisable, got %v", err)
	}
	got, _ := m.NotificationStore.List(context.Background(), "councillor-1", 0)
	if len(got) != 0 {
		t.Fatalf("a refused hand over left %+v", got)
	}
}

func TestAHandOverNamesARecipientAndASubject(t *testing.T) {
	m := newPushManager(t)
	m.UseRaisable(onlyReportHandOvers)
	if _, err := handOver(m, "officer-1", ""); !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("no recipient: expected ErrNotificationInvalid, got %v", err)
	}
	_, err := m.RaiseNotificationFor(context.Background(), "officer-1", "councillor-1", models.CategoryReport, models.EventReportHandedOver, models.SubjectReport, "", nil)
	if !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("no subject: expected ErrNotificationInvalid, got %v", err)
	}
}

func TestMutingReportSilencesAHandOver(t *testing.T) {
	m := newPushManager(t)
	m.UseRaisable(onlyReportHandOvers)
	if _, err := m.NotificationStore.SetPreference(context.Background(), "councillor-1", models.CategoryReport, true); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	if _, err := handOver(m, "officer-1", "councillor-1"); !errors.Is(err, mwanachamacomm.ErrNotificationMuted) {
		t.Fatalf("expected ErrNotificationMuted, got %v", err)
	}
}
