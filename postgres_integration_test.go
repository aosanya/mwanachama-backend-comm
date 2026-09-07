//go:build postgres

// postgres_integration_test.go exercises the GORM stores against a real
// Postgres, mirroring mwanachama-backend-actor's postgres_integration_test.go
// and this module's own former postgres_scratch_test.go (deleted along with
// schema.sql — gormstore.Migrate is now the schema source for both
// dialects, so there is nothing left for a checked-in fixture to do). Skips
// unless POSTGRES_URL is set. The fast sqlite-backed tests elsewhere in
// this package already exhaustively cover business logic; this file's job
// is narrower — prove the real Postgres wiring (sequence-minted ids,
// transactions, jsonb round-trips, the *sql.Tx bridge DEV-1341's act-log
// write depends on) works end-to-end.
package mwanachamacomm_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-shared/postgres"
)

// newPostgresDB opens POSTGRES_URL, migrates a unique-enough table prefix,
// and returns a ready-to-use *gorm.DB plus the table set. Skips the calling
// test if POSTGRES_URL is unset. Tables are dropped on cleanup.
func newPostgresDB(t *testing.T) (*gorm.DB, mwanachamacomm.TableNames) {
	t.Helper()
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping Postgres integration test (see Makefile's test-pg target)")
	}

	ctx := context.Background()
	sqlDB, err := postgres.Open(ctx, postgres.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("postgres.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	tables := mwanachamacomm.TableNames{
		ChatThreads: "commi_chat_thread", ChatMessages: "commi_chat_message",
		DMThreads: "commi_dm_thread", DMParticipants: "commi_dm_participant",
		DMMessages: "commi_dm_message", DMDeviceKeys: "commi_dm_device_key", DMReactions: "commi_dm_message_reaction",
		Reports: "commi_message_report", Removals: "commi_message_removal",
		Disputes: "commi_removal_dispute", Dismissals: "commi_message_report_dismissal",
	}
	if err := mwanachamacomm.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(
			tables.Dismissals, tables.Disputes, tables.Removals, tables.Reports,
			tables.DMReactions, tables.DMDeviceKeys, tables.DMMessages, tables.DMParticipants, tables.DMThreads,
			tables.ChatMessages, tables.ChatThreads,
		)
		_ = db.Exec("DROP TABLE IF EXISTS commi_test_act_log").Error
	})
	return db, tables
}

func TestPostgres_ChatMintThreadIsIdempotentLive(t *testing.T) {
	db, tables := newPostgresDB(t)
	s, err := mwanachamacomm.NewChatStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	ctx := context.Background()
	th1, err := s.MintThread(ctx, "c-1", "/announcements/first")
	if err != nil {
		t.Fatalf("mint 1: %v", err)
	}
	th2, err := s.MintThread(ctx, "c-1", "/announcements/first")
	if err != nil {
		t.Fatalf("mint 2: %v", err)
	}
	if th1.ID != th2.ID {
		t.Fatalf("expected idempotent mint, got %q and %q", th1.ID, th2.ID)
	}
}

func TestPostgres_DMLifecycleLive(t *testing.T) {
	db, tables := newPostgresDB(t)
	s, err := mwanachamacomm.NewDMStore(db, tables, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	ctx := context.Background()
	th, err := s.CreateThread(ctx, mwanachamacomm.DMThread{Title: "board", CreatedBy: "m-1"}, []string{"m-2"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Invite(ctx, th.ID, "m-3", "m-1"); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if _, err := s.Accept(ctx, th.ID, "m-2"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if _, err := s.Post(ctx, mwanachamacomm.DMMessage{ThreadID: th.ID, SenderID: "m-1", PayloadCiphertext: "hi"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	msgs, err := s.ListMessages(ctx, th.ID)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("ListMessages = %+v, err %v, want 1 message", msgs, err)
	}
	parts, err := s.ListParticipants(ctx, th.ID)
	if err != nil || len(parts) != 3 {
		t.Fatalf("ListParticipants = %+v, err %v, want 3", parts, err)
	}
}

// pgTxActWriter mirrors this package's own txActWriter, against a
// Postgres-only test table (commi_test_act_log) created ad hoc — there is
// no checked-in schema.sql fixture any more (see this file's package doc).
type pgTxActWriter struct{ fail bool }

func (w pgTxActWriter) WriteAct(ctx context.Context, tx *sql.Tx, e mwanachamacomm.ActEntry) error {
	if w.fail {
		return errors.New("simulated act-log failure")
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO commi_test_act_log (chapter_id, kind, actor_id, subject_id) VALUES ($1, $2, $3, $4)`,
		e.ChapterID, string(e.Kind), e.ActorID, e.SubjectID)
	return err
}

// TestPostgres_ModerationActLogWrittenInSameTransactionLive proves
// CreateRemoval's act-log write lands in the same transaction as the
// removal row via the GORM/*sql.Tx bridge (sqlTxFrom in moderation_impl.go):
// a removal succeeds and is logged together, and an ActWriter failure rolls
// back the removal too, leaving neither row behind.
func TestPostgres_ModerationActLogWrittenInSameTransactionLive(t *testing.T) {
	db, tables := newPostgresDB(t)
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS commi_test_act_log (
		id bigserial PRIMARY KEY, chapter_id text NOT NULL, kind text NOT NULL,
		actor_id text, subject_id text, occurred_at timestamptz NOT NULL DEFAULT now())`).Error; err != nil {
		t.Fatalf("create commi_test_act_log: %v", err)
	}
	ctx := context.Background()

	okStore, err := mwanachamacomm.NewModerationStore(db, tables, nil, pgTxActWriter{})
	if err != nil {
		t.Fatalf("NewModerationStore: %v", err)
	}
	rem, err := okStore.CreateRemoval(ctx, mwanachamacomm.Removal{
		MessageID: "msg-1", ChapterID: "ward-1", RemovedBy: "mod-1",
		ActorRoleClass: "Ward coordinator", Reason: mwanachamacomm.RemovalReasonAbuse,
	}, "Ward wall", mwanachamacomm.Actor{ID: "mod-1", ChapterID: "ward-1"})
	if err != nil {
		t.Fatalf("CreateRemoval: %v", err)
	}
	var actCount int
	if err := db.Raw(`SELECT count(*) FROM commi_test_act_log WHERE subject_id = ?`, rem.MessageID).Scan(&actCount).Error; err != nil {
		t.Fatalf("counting act log: %v", err)
	}
	if actCount != 1 {
		t.Fatalf("expected exactly 1 act-log row committed with the removal, got %d", actCount)
	}

	failStore, err := mwanachamacomm.NewModerationStore(db, tables, nil, pgTxActWriter{fail: true})
	if err != nil {
		t.Fatalf("NewModerationStore(fail): %v", err)
	}
	if _, err := failStore.CreateRemoval(ctx, mwanachamacomm.Removal{
		MessageID: "msg-2", ChapterID: "ward-1", RemovedBy: "mod-1",
		ActorRoleClass: "Ward coordinator", Reason: mwanachamacomm.RemovalReasonAbuse,
	}, "Ward wall", mwanachamacomm.Actor{ID: "mod-1", ChapterID: "ward-1"}); err == nil {
		t.Fatal("expected CreateRemoval to fail when the act log write fails")
	}
	if _, err := okStore.GetRemovalForMessage(ctx, "msg-2"); !errors.Is(err, mwanachamacomm.ErrModerationNotFound) {
		t.Fatalf("expected the removal to have rolled back with the failed act write, got %v", err)
	}
}

// TestPostgres_ParticipantOrderMatchesSqliteLive proves Postgres and
// sqlite (this package's other dialect) agree on `ORDER BY updated_at,
// member_id` for the same sequence of roster changes — the parity the old
// store_postgres_dm.go/store_memory_dm.go split needed a dedicated test to
// prove; a single GORM store removes the question by construction, but this
// keeps a live check that Postgres's actual column ordering behaves as
// expected under real timestamps.
func TestPostgres_ParticipantOrderMatchesSqliteLive(t *testing.T) {
	pgDB, pgTables := newPostgresDB(t)
	pg, err := mwanachamacomm.NewDMStore(pgDB, pgTables, nil)
	if err != nil {
		t.Fatalf("NewDMStore(pg): %v", err)
	}
	sqliteDB, sqliteTables := newTestDB(t)
	sq, err := mwanachamacomm.NewDMStore(sqliteDB, sqliteTables, nil)
	if err != nil {
		t.Fatalf("NewDMStore(sqlite): %v", err)
	}
	ctx := context.Background()

	pgTh, err := pg.CreateThread(ctx, mwanachamacomm.DMThread{CreatedBy: "a"}, []string{"b", "c"})
	if err != nil {
		t.Fatalf("pg create: %v", err)
	}
	sqTh, err := sq.CreateThread(ctx, mwanachamacomm.DMThread{CreatedBy: "a"}, []string{"b", "c"})
	if err != nil {
		t.Fatalf("sqlite create: %v", err)
	}
	time.Sleep(2 * time.Millisecond) // keep updated_at strictly ordered on both dialects
	if _, err := pg.Accept(ctx, pgTh.ID, "b"); err != nil {
		t.Fatalf("pg accept: %v", err)
	}
	if _, err := sq.Accept(ctx, sqTh.ID, "b"); err != nil {
		t.Fatalf("sqlite accept: %v", err)
	}

	pgParts, err := pg.ListParticipants(ctx, pgTh.ID)
	if err != nil {
		t.Fatalf("pg list: %v", err)
	}
	sqParts, err := sq.ListParticipants(ctx, sqTh.ID)
	if err != nil {
		t.Fatalf("sqlite list: %v", err)
	}
	if len(pgParts) != len(sqParts) {
		t.Fatalf("participant count mismatch: pg=%d sqlite=%d", len(pgParts), len(sqParts))
	}
	for i := range pgParts {
		if pgParts[i].MemberID != sqParts[i].MemberID {
			t.Fatalf("order mismatch at %d: pg=%q sqlite=%q", i, pgParts[i].MemberID, sqParts[i].MemberID)
		}
	}
}
