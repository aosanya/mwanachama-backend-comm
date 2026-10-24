package mwanachamacomm_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/spec"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

const categoryConsultation models.NotificationCategory = "consultation"

func civicSpecWidenedWith(t *testing.T, extra models.NotificationCategory) *spec.Spec {
	t.Helper()
	raw, err := os.ReadFile("civic.comm.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, o := range doc["objects"].([]any) {
		obj := o.(map[string]any)
		if obj["role"] != "notification" && obj["role"] != "notification_preference" {
			continue
		}
		for _, f := range obj["fields"].([]any) {
			field := f.(map[string]any)
			if field["name"] == "category" {
				field["values"] = append(field["values"].([]any), string(extra))
			}
		}
	}
	widened, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	s, err := mwanachamacomm.ParseSpec(widened)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	return s
}

func notificationStoreOver(t *testing.T, s *spec.Spec) *mwanachamacomm.NotificationStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := mwanachamacomm.Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	store, err := mwanachamacomm.NewNotificationStore(db, s, time.Now)
	if err != nil {
		t.Fatalf("NewNotificationStore: %v", err)
	}
	return store
}

func TestASpecWideningCategoryLetsAMemberMuteTheNewCategory(t *testing.T) {
	store := notificationStoreOver(t, civicSpecWidenedWith(t, categoryConsultation))
	ctx := context.Background()
	p, err := store.SetPreference(ctx, "resident-1", categoryConsultation, true)
	if err != nil || !p.Muted || p.Category != categoryConsultation {
		t.Fatalf("SetPreference = %+v, %v", p, err)
	}
	muted, err := store.IsMuted(ctx, "resident-1", categoryConsultation)
	if err != nil || !muted {
		t.Fatalf("IsMuted = %v, %v", muted, err)
	}
}

func TestASpecWithoutTheCategoryRefusesAMuteOnIt(t *testing.T) {
	store := notificationStoreOver(t, civicSpec(t))
	_, err := store.SetPreference(context.Background(), "resident-1", categoryConsultation, true)
	if !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid, got %v", err)
	}
}

func TestAWidenedCategoryPassesTheDeclaredCheckOnARaise(t *testing.T) {
	widened := civicSpecWidenedWith(t, categoryConsultation)
	n := models.Notification{ActorID: "resident-1", Category: categoryConsultation, Event: models.EventSurveyReached, SubjectKind: models.SubjectSurvey, SubjectID: "form-1"}
	if err := mwanachamacomm.Check(widened, "notification", n); err != nil {
		t.Fatalf("widened spec refused the category: %v", err)
	}
	if err := mwanachamacomm.Check(civicSpec(t), "notification", n); !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("civic spec accepted the category: %v", err)
	}
}

func TestAWidenedCategoryCannotYetBeRaisedBecauseNoEventPairsWithIt(t *testing.T) {
	store := notificationStoreOver(t, civicSpecWidenedWith(t, categoryConsultation))
	for _, e := range models.NotificationEvents() {
		if c, _ := models.NotificationCategoryOf(e); c == categoryConsultation {
			t.Fatalf("event %q pairs with consultation in Go; CM60's raise half is unblocked, replace this pin with a raise test", e)
		}
	}
	_, err := store.Raise(context.Background(), models.Notification{
		ActorID: "resident-1", Category: categoryConsultation, Event: models.EventSurveyReached,
		SubjectKind: models.SubjectSurvey, SubjectID: "form-1",
	})
	if !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("expected ErrNotificationInvalid from the Go event-category pairing, got %v", err)
	}
}

func civicSpec(t *testing.T) *spec.Spec {
	t.Helper()
	s, err := mwanachamacomm.SpecFor("mwanachama")
	if err != nil {
		t.Fatal(err)
	}
	return s
}
