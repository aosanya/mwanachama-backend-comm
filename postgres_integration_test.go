//go:build integration

package mwanachamacomm_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/postgres"
	"github.com/aosanya/mwanachama-backend-shared/spec"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func newPostgresDB(t *testing.T) (*gorm.DB, *spec.Spec) {
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

	s, err := mwanachamacomm.SpecFor("commi")
	if err != nil {
		t.Fatalf("SpecFor: %v", err)
	}
	if err := mwanachamacomm.Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	t.Cleanup(func() {
		for _, o := range s.Objects {
			_ = db.Migrator().DropTable(s.TableFor(o))
		}
		_ = db.Migrator().DropTable(s.NameRegistryTable())
		_ = db.Exec("DROP TABLE IF EXISTS commi_test_act_log").Error
	})
	return db, s
}

func TestPostgres_ChatMintThreadIsIdempotentLive(t *testing.T) {
	db, dspec := newPostgresDB(t)
	s, err := mwanachamacomm.NewChatStore(db, dspec, nil)
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
	db, dspec := newPostgresDB(t)
	s, err := mwanachamacomm.NewDMStore(db, dspec, nil)
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
		`INSERT INTO commi_test_act_log (structure_id, kind, actor_id, subject_id) VALUES ($1, $2, $3, $4)`,
		e.StructureID, string(e.Kind), e.ActorID, e.SubjectID)
	return err
}

// TestPostgres_ModerationActLogWrittenInSameTransactionLive proves
// CreateRemoval's act-log write lands in the same transaction as the
// removal row via the GORM/*sql.Tx bridge (sqlTxFrom in moderation_impl.go):
// a removal succeeds and is logged together, and an ActWriter failure rolls
// back the removal too, leaving neither row behind.
func TestPostgres_ModerationActLogWrittenInSameTransactionLive(t *testing.T) {
	db, dspec := newPostgresDB(t)
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS commi_test_act_log (
		id bigserial PRIMARY KEY, structure_id text NOT NULL, kind text NOT NULL,
		actor_id text, subject_id text, occurred_at timestamptz NOT NULL DEFAULT now())`).Error; err != nil {
		t.Fatalf("create commi_test_act_log: %v", err)
	}
	ctx := context.Background()

	okStore, err := mwanachamacomm.NewModerationStore(db, dspec, nil, pgTxActWriter{})
	if err != nil {
		t.Fatalf("NewModerationStore: %v", err)
	}
	rem, err := okStore.CreateRemoval(ctx, mwanachamacomm.Removal{
		MessageID: "msg-1", StructureID: "ward-1", RemovedBy: "mod-1",
		ActorRoleClass: "Ward coordinator", Reason: mwanachamacomm.RemovalReasonAbuse,
	}, "Ward wall", mwanachamacomm.Actor{ID: "mod-1", StructureID: "ward-1"})
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

	failStore, err := mwanachamacomm.NewModerationStore(db, dspec, nil, pgTxActWriter{fail: true})
	if err != nil {
		t.Fatalf("NewModerationStore(fail): %v", err)
	}
	if _, err := failStore.CreateRemoval(ctx, mwanachamacomm.Removal{
		MessageID: "msg-2", StructureID: "ward-1", RemovedBy: "mod-1",
		ActorRoleClass: "Ward coordinator", Reason: mwanachamacomm.RemovalReasonAbuse,
	}, "Ward wall", mwanachamacomm.Actor{ID: "mod-1", StructureID: "ward-1"}); err == nil {
		t.Fatal("expected CreateRemoval to fail when the act log write fails")
	}
	if _, err := okStore.GetRemovalForMessage(ctx, "msg-2"); !errors.Is(err, mwanachamacomm.ErrModerationNotFound) {
		t.Fatalf("expected the removal to have rolled back with the failed act write, got %v", err)
	}
}

func TestPostgres_ParticipantOrderMatchesSqliteLive(t *testing.T) {
	pgDB, pgSpec := newPostgresDB(t)
	pg, err := mwanachamacomm.NewDMStore(pgDB, pgSpec, nil)
	if err != nil {
		t.Fatalf("NewDMStore(pg): %v", err)
	}
	sqliteDB, sqliteSpec := newTestDB(t)
	sq, err := mwanachamacomm.NewDMStore(sqliteDB, sqliteSpec, nil)
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
		if pgParts[i].ActorID != sqParts[i].ActorID {
			t.Fatalf("order mismatch at %d: pg=%q sqlite=%q", i, pgParts[i].ActorID, sqParts[i].ActorID)
		}
	}
}

func TestPostgres_JSONColumnsRoundTripThroughJSONBLive(t *testing.T) {
	db, dspec := newPostgresDB(t)
	dm, err := mwanachamacomm.NewDMStore(db, dspec, nil)
	if err != nil {
		t.Fatalf("NewDMStore: %v", err)
	}
	ctx := context.Background()

	sealed := json.RawMessage(`{"alg":"x25519","ct":"abc"}`)
	th, err := dm.CreateThread(ctx, mwanachamacomm.DMThread{
		CreatedBy: "m-1", SentFromAddressSealed: sealed,
	}, []string{"m-2"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	back, err := dm.GetThread(ctx, th.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	var got, want map[string]any
	if err := json.Unmarshal(back.SentFromAddressSealed, &got); err != nil {
		t.Fatalf("sealed came back as %q: %v", back.SentFromAddressSealed, err)
	}
	if err := json.Unmarshal(sealed, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sealed round tripped as %v, want %v", got, want)
	}

	keys := map[string]string{"dkey-1": "wrapped-1", "dkey-2": "wrapped-2"}
	msg, err := dm.Post(ctx, mwanachamacomm.DMMessage{
		ThreadID: th.ID, SenderID: "m-1", PayloadCiphertext: "ct", PerRecipientKeys: keys,
	})
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	fetched, err := dm.GetMessage(ctx, msg.ID)
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if !reflect.DeepEqual(fetched.PerRecipientKeys, keys) {
		t.Fatalf("per_recipient_keys round tripped as %v, want %v", fetched.PerRecipientKeys, keys)
	}
}

// A declared timestamp is a text column, so ORDER BY over it is a string
// comparison. SQLite cannot show a divergence here because it stores
// whatever it is handed; Postgres will.
func TestPostgres_TimestampOrderingIsChronologicalLive(t *testing.T) {
	db, dspec := newPostgresDB(t)
	chat, err := mwanachamacomm.NewChatStore(db, dspec, nil)
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	ctx := context.Background()

	for i := 0; i < 12; i++ {
		if _, err := chat.Post(ctx, mwanachamacomm.ChatMessage{
			StructureID: "ward-1", AuthorID: "a-1", Body: fmt.Sprintf("m%02d", i),
		}); err != nil {
			t.Fatalf("post %d: %v", i, err)
		}
	}

	msgs, err := chat.ListMessages(ctx, "ward-1", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 12 {
		t.Fatalf("got %d messages, want 12", len(msgs))
	}
	for i := 1; i < len(msgs); i++ {
		if msgs[i].CreatedAt.Before(msgs[i-1].CreatedAt) {
			t.Fatalf("message %d (%s) sorts before %d (%s) — the stored layout is not order-preserving",
				i, msgs[i].CreatedAt, i-1, msgs[i-1].CreatedAt)
		}
	}
}
