package mwanachamacomm_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	comm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
	"github.com/aosanya/mwanachama-backend-shared/consent"
	"github.com/aosanya/mwanachama-backend-shared/spec"
)

type noActs struct{}

func (noActs) WriteAct(context.Context, *sql.Tx, models.ActEntry) error { return nil }

type gateFor map[string]error

func (g gateFor) Check(_ context.Context, actorID string) error { return g[actorID] }

func gatedManager(t *testing.T, g consent.Gate) *comm.CommManager {
	t.Helper()
	m, _, _ := gatedManagerOn(t, g)
	return m
}

func gatedManagerOn(t *testing.T, g consent.Gate) (*comm.CommManager, *gorm.DB, *spec.Spec) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	s, err := comm.SpecFor("test")
	if err != nil {
		t.Fatalf("SpecFor: %v", err)
	}
	if err := comm.Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	m, err := comm.NewCommManager(db, s, nil, noActs{}, "")
	if err != nil {
		t.Fatalf("NewCommManager: %v", err)
	}
	m.UseConsentGate(g)
	return m, db, s
}

func TestAChatPostNeedsALiveConsent(t *testing.T) {
	m := gatedManager(t, gateFor{"mem-2": consent.ErrRequired, "mem-3": consent.ErrWithdrawn})
	ctx := context.Background()
	if _, err := m.PostChatMessage(ctx, models.ChatMessage{StructureID: "s-1", AuthorID: "mem-2", Body: "hello"}); !errors.Is(err, comm.ErrConsentRequired) {
		t.Fatalf("post without consent = %v, want ErrConsentRequired", err)
	}
	if _, err := m.PostChatMessage(ctx, models.ChatMessage{StructureID: "s-1", AuthorID: "mem-3", Body: "hello"}); !errors.Is(err, comm.ErrConsentWithdrawn) {
		t.Fatalf("post after withdrawal = %v, want ErrConsentWithdrawn", err)
	}
	if _, err := m.PostChatMessage(ctx, models.ChatMessage{StructureID: "s-1", AuthorID: "mem-1", Body: "hello"}); errors.Is(err, comm.ErrConsentRequired) || errors.Is(err, comm.ErrConsentWithdrawn) {
		t.Fatalf("consenting post refused for consent: %v", err)
	}
}

func TestADirectMessageNeedsALiveConsent(t *testing.T) {
	m := gatedManager(t, gateFor{"mem-2": consent.ErrRequired})
	_, err := m.PostDMMessage(context.Background(), models.DMMessage{ThreadID: "t-1", SenderID: "mem-2", PayloadCiphertext: "x"})
	if !errors.Is(err, comm.ErrConsentRequired) {
		t.Fatalf("DM without consent = %v, want ErrConsentRequired", err)
	}
}

func TestAChatReadMarkerNeedsALiveConsent(t *testing.T) {
	m := gatedManager(t, gateFor{"mem-2": consent.ErrWithdrawn})
	_, err := m.MarkChatThreadReadFor(context.Background(), "s-1", "t-1", "mem-2", "")
	if !errors.Is(err, comm.ErrConsentWithdrawn) {
		t.Fatalf("read marker after withdrawal = %v, want ErrConsentWithdrawn", err)
	}
}
