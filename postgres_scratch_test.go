//go:build postgres

// postgres_scratch_test.go exercises the postgres store files against a real
// Postgres, mirroring mwanachama-backend-taskmanager's
// postgres_integration_test.go: skip unless POSTGRES_URL is set, stand up a
// scratch schema, drop it on cleanup. Unlike taskmanager's generic
// entitygraph DDL generator, comm's tables are bespoke SQL, so the scratch
// schema comes from the embedded schema.sql fixture instead — see that
// file's header for the two ways it deliberately diverges from production.
//
// The extensive per-backend unit tests already cover business logic against
// the memory store; this file's job is narrower — prove the real Postgres
// wiring (transactions, jsonb round-trips, nullable columns, sequences)
// works end-to-end, and that Postgres and memory agree on ordering.
package mwanachamacomm

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var scratchSchema string

// scratchDB opens POSTGRES_URL, applies schema.sql, and drops every comm_
// table on cleanup. Skips the calling test if POSTGRES_URL is unset.
func scratchDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping Postgres scratch test (see Makefile's test-pg target)")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), scratchSchema); err != nil {
		t.Fatalf("applying schema.sql: %v", err)
	}
	t.Cleanup(func() {
		const drop = `DROP TABLE IF EXISTS
			comm_test_act_log, comm_message_report_dismissal, comm_removal_dispute,
			comm_message_removal, comm_message_report, comm_dm_message_reaction,
			comm_dm_device_key, comm_dm_message, comm_dm_participant, comm_dm_thread,
			comm_chat_message, comm_chat_thread CASCADE`
		_, _ = db.ExecContext(context.Background(), drop)
	})
	return db
}

func TestChatMintThreadIsIdempotentLive(t *testing.T) {
	db := scratchDB(t)
	s := NewChatPostgresStore(db)
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

func TestDMLifecycleLive(t *testing.T) {
	db := scratchDB(t)
	s := NewDMPostgresStore(db)
	ctx := context.Background()

	th, err := s.CreateThread(ctx, DMThread{Title: "board", CreatedBy: "m-1"}, []string{"m-2"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Invite(ctx, th.ID, "m-3", "m-1"); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if _, err := s.Accept(ctx, th.ID, "m-2"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if _, err := s.Post(ctx, DMMessage{ThreadID: th.ID, SenderID: "m-1", PayloadCiphertext: "hi"}); err != nil {
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

// testTxActWriter writes into the test-only comm_test_act_log table within
// the caller's transaction, standing in for the gateway's real
// chapter_act_log_entry adapter.
type testTxActWriter struct{ fail bool }

func (w testTxActWriter) WriteAct(ctx context.Context, tx *sql.Tx, e ActEntry) error {
	if w.fail {
		return errors.New("simulated act-log failure")
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO comm_test_act_log (chapter_id, kind, actor_id, subject_id) VALUES ($1, $2, $3, $4)`,
		e.ChapterID, string(e.Kind), e.ActorID, e.SubjectID)
	return err
}

// TestModerationActLogWrittenInSameTransactionLive proves CreateRemoval's
// act-log write lands in the same transaction as the removal row: a
// removal succeeds and is logged together, and an ActWriter failure rolls
// back the removal too, leaving neither row behind.
func TestModerationActLogWrittenInSameTransactionLive(t *testing.T) {
	db := scratchDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx,
		`INSERT INTO comm_chat_message (id, chapter_id, author_id, body) VALUES ('msg-1', 'ward-1', 'author-1', 'hello')`); err != nil {
		t.Fatalf("seed message: %v", err)
	}

	okStore := NewModerationPostgresStore(db, testTxActWriter{})
	rem, err := okStore.CreateRemoval(ctx, Removal{
		MessageID: "msg-1", ChapterID: "ward-1", RemovedBy: "mod-1",
		ActorRoleClass: "Ward coordinator", Reason: RemovalReasonAbuse,
	}, "Ward wall", Actor{ID: "mod-1", ChapterID: "ward-1"})
	if err != nil {
		t.Fatalf("CreateRemoval: %v", err)
	}
	var actCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM comm_test_act_log WHERE subject_id = $1`, rem.MessageID).Scan(&actCount); err != nil {
		t.Fatalf("counting act log: %v", err)
	}
	if actCount != 1 {
		t.Fatalf("expected exactly 1 act-log row committed with the removal, got %d", actCount)
	}

	// A second message, removed by a store whose ActWriter always fails: the
	// removal must not survive the rollback either.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO comm_chat_message (id, chapter_id, author_id, body) VALUES ('msg-2', 'ward-1', 'author-1', 'hello again')`); err != nil {
		t.Fatalf("seed message 2: %v", err)
	}
	failStore := NewModerationPostgresStore(db, testTxActWriter{fail: true})
	if _, err := failStore.CreateRemoval(ctx, Removal{
		MessageID: "msg-2", ChapterID: "ward-1", RemovedBy: "mod-1",
		ActorRoleClass: "Ward coordinator", Reason: RemovalReasonAbuse,
	}, "Ward wall", Actor{ID: "mod-1", ChapterID: "ward-1"}); err == nil {
		t.Fatal("expected CreateRemoval to fail when the act log write fails")
	}
	if _, err := okStore.GetRemovalForMessage(ctx, "msg-2"); !errors.Is(err, ErrModerationNotFound) {
		t.Fatalf("expected the removal to have rolled back with the failed act write, got %v", err)
	}
}

// TestParticipantOrderMatchesAcrossBackendsLive proves Postgres's
// `ORDER BY updated_at, member_id` and the memory store's sort produce the
// same participant order for the same sequence of roster changes.
func TestParticipantOrderMatchesAcrossBackendsLive(t *testing.T) {
	db := scratchDB(t)
	pg := NewDMPostgresStore(db)
	mem := NewDMMemoryStore(NewIDGen(), monotonicClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	ctx := context.Background()

	pgTh, err := pg.CreateThread(ctx, DMThread{CreatedBy: "a"}, []string{"b", "c"})
	if err != nil {
		t.Fatalf("pg create: %v", err)
	}
	memTh, err := mem.CreateThread(ctx, DMThread{CreatedBy: "a"}, []string{"b", "c"})
	if err != nil {
		t.Fatalf("mem create: %v", err)
	}
	if _, err := pg.Accept(ctx, pgTh.ID, "b"); err != nil {
		t.Fatalf("pg accept: %v", err)
	}
	if _, err := mem.Accept(ctx, memTh.ID, "b"); err != nil {
		t.Fatalf("mem accept: %v", err)
	}

	pgParts, err := pg.ListParticipants(ctx, pgTh.ID)
	if err != nil {
		t.Fatalf("pg list: %v", err)
	}
	memParts, err := mem.ListParticipants(ctx, memTh.ID)
	if err != nil {
		t.Fatalf("mem list: %v", err)
	}
	if len(pgParts) != len(memParts) {
		t.Fatalf("participant count mismatch: pg=%d mem=%d", len(pgParts), len(memParts))
	}
	for i := range pgParts {
		if pgParts[i].MemberID != memParts[i].MemberID {
			t.Fatalf("order mismatch at %d: pg=%q mem=%q", i, pgParts[i].MemberID, memParts[i].MemberID)
		}
	}
}
