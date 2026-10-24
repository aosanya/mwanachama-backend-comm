package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
	"github.com/aosanya/mwanachama-backend-shared/spec"
	"gorm.io/gorm"
)

func newRulesManager(t *testing.T) (*mwanachamacomm.CommManager, *spec.Spec, *gorm.DB) {
	t.Helper()
	db, s := newTestDB(t)
	m, err := mwanachamacomm.NewCommManager(db, s, monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), nopActWriter{}, "")
	if err != nil {
		t.Fatal(err)
	}
	return m, s, db
}

func TestTheBlueprintDeclaresNotificationRulesDefaultingToNoRule(t *testing.T) {
	_, s, _ := newRulesManager(t)
	st, ok := s.Setting("notification_rules")
	if !ok || st.Type != spec.TypeJSON {
		t.Fatalf("notification_rules declared=%v type=%s", ok, st.Type)
	}
}

func TestWithNothingStoredEveryEventKeepsTheHostsOwnDefault(t *testing.T) {
	m, _, _ := newRulesManager(t)
	got, err := m.NotificationRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Layer != spec.LayerModule || len(got.Rules) != 0 {
		t.Fatalf("got %+v", got)
	}
	if _, ruled, err := m.NotificationRule(context.Background(), "thread_reply"); err != nil || ruled {
		t.Fatalf("ruled=%v err=%v", ruled, err)
	}
}

func TestTheDefinitionLayerIsReadAndAnOverrideOutranksItUntilCleared(t *testing.T) {
	m, s, db := newRulesManager(t)
	ctx := context.Background()
	definition := []any{map[string]any{"event": "thread_reply", "told": []any{"thread"}, "fallback": []any{}, "mutable": true}}
	if err := spec.PutDefinitionSettings(db, s, map[string]any{"notification_rules": definition}); err != nil {
		t.Fatal(err)
	}
	r, ruled, err := m.NotificationRule(ctx, "thread_reply")
	if err != nil || !ruled || !r.Mutable || len(r.Told) != 1 || r.Told[0] != "thread" {
		t.Fatalf("definition: %+v ruled=%v err=%v", r, ruled, err)
	}
	override := []any{map[string]any{"event": "thread_reply", "told": []any{"leads"}, "fallback": []any{"owners"}, "mutable": false}}
	if _, err := spec.StoreOverride(db, s, "notification_rules", override, "olu", "2026-10-24T08:00:00Z"); err != nil {
		t.Fatal(err)
	}
	got, err := m.NotificationRules(ctx)
	if err != nil || got.Layer != spec.LayerOverride || got.By != "olu" || got.Rules[0].Mutable || got.Rules[0].Fallback[0] != "owners" {
		t.Fatalf("override: %+v err=%v", got, err)
	}
	if _, _, err := spec.StoreClearOverride(db, s, "notification_rules", "olu", "2026-10-24T08:01:00Z"); err != nil {
		t.Fatal(err)
	}
	got, err = m.NotificationRules(ctx)
	if err != nil || got.Layer != spec.LayerDefinition {
		t.Fatalf("after clear: %+v err=%v", got, err)
	}
}

func TestARuleWithNoEventOrARepeatedEventIsRefused(t *testing.T) {
	for _, v := range []any{
		[]any{map[string]any{"told": []any{"thread"}}},
		[]any{map[string]any{"event": "a"}, map[string]any{"event": "a"}},
		[]any{map[string]any{"event": "a", "told": []any{""}}},
	} {
		if _, err := models.ParseNotificationRules(v); !errors.Is(err, models.ErrNotificationRuleInvalid) {
			t.Fatalf("%v: err=%v", v, err)
		}
	}
}
