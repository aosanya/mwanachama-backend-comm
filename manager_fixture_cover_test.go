package mwanachamacomm

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-comm/models"
	"github.com/aosanya/mwanachama-backend-shared/consent"
)

type cover13Acts struct {
	mu      sync.Mutex
	entries []models.ActEntry
}

func (a *cover13Acts) WriteAct(_ context.Context, _ *sql.Tx, e models.ActEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, e)
	return nil
}

func (a *cover13Acts) written() []models.ActEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]models.ActEntry(nil), a.entries...)
}

type cover13Clock struct {
	mu sync.Mutex
	at time.Time
}

func (c *cover13Clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(time.Second)
	return c.at
}

type cover13Fixture struct {
	m     *CommManager
	db    *gorm.DB
	acts  *cover13Acts
	clock *cover13Clock
}

func cover13Setup(t *testing.T) *cover13Fixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	s, err := SpecFor("cover13")
	if err != nil {
		t.Fatalf("SpecFor: %v", err)
	}
	if err := Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	clock := &cover13Clock{at: time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)}
	acts := &cover13Acts{}
	m, err := NewCommManager(db, s, clock.now, acts, "")
	if err != nil {
		t.Fatalf("NewCommManager: %v", err)
	}
	m.UseConsentGate(cover13Gate{})
	return &cover13Fixture{m: m, db: db, acts: acts, clock: clock}
}

type cover13Gate struct{}

func (cover13Gate) Check(context.Context, string) error { return nil }

var _ consent.Gate = cover13Gate{}

func (f *cover13Fixture) message(t *testing.T, structure, thread, author, body string) models.ChatMessage {
	t.Helper()
	msg, err := f.m.ChatStore.Post(context.Background(), models.ChatMessage{
		StructureID: structure, ThreadID: thread, AuthorID: author, Body: body,
	})
	if err != nil {
		t.Fatalf("post %q: %v", body, err)
	}
	return msg
}
