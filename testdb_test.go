package mwanachamacomm_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/spec"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func newTestDB(t *testing.T) (*gorm.DB, *spec.Spec) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB(): %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	s, err := mwanachamacomm.SpecFor("mwanachama")
	if err != nil {
		t.Fatalf("SpecFor: %v", err)
	}
	if err := mwanachamacomm.Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	return db, s
}

// monotonicClock returns a [mwanachamacomm.Clock] that advances by 1ms on
// every call starting from start — deterministic ordering for tests that
// need to distinguish "created before" from "created in the same instant",
// mirroring the old store_memory_*_test.go files' monotonicClock helper.
func monotonicClock(start time.Time) mwanachamacomm.Clock {
	t := start
	return func() time.Time {
		t = t.Add(time.Millisecond)
		return t
	}
}

const testActorsTable = "test_member_actors"

func createTestActors(t *testing.T, db *gorm.DB) {
	t.Helper()
	ddl := `CREATE TABLE IF NOT EXISTS ` + testActorsTable + ` (
		id TEXT PRIMARY KEY,
		display_name TEXT
	)`
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatalf("createTestActors: %v", err)
	}
}

func insertTestActor(t *testing.T, db *gorm.DB, id, displayName string) {
	t.Helper()
	err := db.Exec(`INSERT INTO `+testActorsTable+` (id, display_name) VALUES (?, ?)`, id, displayName).Error
	if err != nil {
		t.Fatalf("insertTestActor: %v", err)
	}
}

func createTestActLog(t *testing.T, db *gorm.DB) {
	t.Helper()
	const ddl = `CREATE TABLE IF NOT EXISTS test_act_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		structure_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		actor_id TEXT,
		subject_id TEXT
	)`
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatalf("createTestActLog: %v", err)
	}
}

// testActLogCount counts test_act_log rows naming subjectID — used to prove
// an act-log write did (or, on a simulated failure, did not) survive a
// transaction alongside the moderation row it was written with.
func testActLogCount(t *testing.T, db *gorm.DB, subjectID string) int {
	t.Helper()
	var n int64
	if err := db.Raw(`SELECT count(*) FROM test_act_log WHERE subject_id = ?`, subjectID).Scan(&n).Error; err != nil {
		t.Fatalf("testActLogCount: %v", err)
	}
	return int(n)
}

// txActWriter is a [mwanachamacomm.ActWriter] that writes into test_act_log using
// the *sql.Tx it's handed — the seam CreateRemoval/DismissReports/
// DecideDispute use, standing in for the gateway's real custody adapter.
// fail simulates the act write itself failing, to prove the moderation row
// rolls back with it.
type txActWriter struct{ fail bool }

func (w txActWriter) WriteAct(ctx context.Context, tx *sql.Tx, e mwanachamacomm.ActEntry) error {
	if w.fail {
		return errors.New("simulated act-log failure")
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO test_act_log (structure_id, kind, actor_id, subject_id) VALUES (?, ?, ?, ?)`,
		e.StructureID, string(e.Kind), e.ActorID, e.SubjectID)
	return err
}
