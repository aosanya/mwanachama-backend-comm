package mwanachamacomm

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func TestReferenceAndConflictNameTheirField(t *testing.T) {
	ref := Reference("thread_id")
	if !errors.Is(ref, ErrInvalidReference) || ref.Error() != "invalid reference: thread_id" {
		t.Fatalf("reference: %v", ref)
	}
	conflict := Conflict("address")
	if !errors.Is(conflict, ErrConflict) || conflict.Error() != "already exists: address" {
		t.Fatalf("conflict: %v", conflict)
	}
	if Reference("") != ErrInvalidReference || Conflict("") != ErrConflict {
		t.Fatalf("an unnamed field should give the bare sentinel")
	}
}

func TestClassifyTurnsPostgresConstraintsIntoReferenceOrConflict(t *testing.T) {
	cases := []struct {
		code     string
		want     error
		wantName string
	}{
		{"23503", ErrInvalidReference, "fk_thread"},
		{"23502", ErrInvalidReference, "fk_thread"},
		{"23514", ErrInvalidReference, "fk_thread"},
		{"23505", ErrConflict, "fk_thread"},
	}
	for _, c := range cases {
		got := classify(&pgconn.PgError{Code: c.code, ConstraintName: "fk_thread"})
		if !errors.Is(got, c.want) || got.Error() != c.want.Error()+": "+c.wantName {
			t.Errorf("code %s gave %v", c.code, got)
		}
	}
}

func TestClassifyNamesTheColumnOrTableWhenTheConstraintHasNoName(t *testing.T) {
	byColumn := classify(&pgconn.PgError{Code: "23502", ColumnName: "body"})
	if byColumn.Error() != "invalid reference: body" {
		t.Fatalf("by column: %v", byColumn)
	}
	byTable := classify(&pgconn.PgError{Code: "23503", TableName: "messages"})
	if byTable.Error() != "invalid reference: messages" {
		t.Fatalf("by table: %v", byTable)
	}
}

func TestClassifyLeavesOtherErrorsAlone(t *testing.T) {
	other := &pgconn.PgError{Code: "40001"}
	if classify(other) != error(other) {
		t.Fatalf("a serialisation failure was reclassified")
	}
	if classify(nil) != nil {
		t.Fatalf("nil became an error")
	}
	if classify(gorm.ErrRecordNotFound) != gorm.ErrRecordNotFound {
		t.Fatalf("not found must stay not found")
	}
	plain := errors.New("disk full")
	if classify(plain) != plain {
		t.Fatalf("a plain error was changed")
	}
}

func TestClassifyReadsSQLiteConstraintMessages(t *testing.T) {
	conflict := classify(errors.New("UNIQUE constraint failed: comm_address.public_address"))
	if !errors.Is(conflict, ErrConflict) || conflict.Error() != "already exists: comm_address.public_address" {
		t.Fatalf("unique: %v", conflict)
	}
	for _, msg := range []string{
		"FOREIGN KEY constraint failed",
		"NOT NULL constraint failed: comm_message.body",
		"CHECK constraint failed: audience",
	} {
		if got := classify(errors.New(msg)); !errors.Is(got, ErrInvalidReference) {
			t.Errorf("%q gave %v", msg, got)
		}
	}
	if got := classify(errors.New("NOT NULL constraint failed: comm_message.body")); got.Error() != "invalid reference: comm_message.body" {
		t.Fatalf("name not taken from the message: %v", got)
	}
}

func TestClassifySeesThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("saving: %w", &pgconn.PgError{Code: "23505", ConstraintName: "uq_addr"})
	if !errors.Is(classify(wrapped), ErrConflict) {
		t.Fatalf("a wrapped unique violation was missed")
	}
	if !errors.Is(classify(fmt.Errorf("saving: %w", gorm.ErrRecordNotFound)), gorm.ErrRecordNotFound) {
		t.Fatalf("a wrapped not found was lost")
	}
}

func TestFlexTimeReadsEverySpellingTheDatabasesWrite(t *testing.T) {
	want := time.Date(2026, 11, 9, 10, 30, 15, 0, time.UTC)
	for _, src := range []any{
		want,
		"2026-11-09T10:30:15Z",
		[]byte("2026-11-09T10:30:15Z"),
		"2026-11-09 11:30:15+01:00",
		"2026-11-09 10:30:15Z",
	} {
		var f flexTime
		if err := f.Scan(src); err != nil || !f.Valid || !f.Time.Equal(want) {
			t.Errorf("%v (%T): %v %+v", src, src, err, f)
		}
	}
}

func TestFlexTimeNullAndBadInput(t *testing.T) {
	f := flexTime{Valid: true}
	if err := f.Scan(nil); err != nil || f.Valid {
		t.Fatalf("null should clear: %v %+v", err, f)
	}
	if err := f.Scan("yesterday-ish"); err == nil {
		t.Fatalf("a nonsense time parsed")
	}
	if err := f.Scan(42); err == nil {
		t.Fatalf("an integer was accepted as a time")
	}
}

func TestSystemClockIsUTC(t *testing.T) {
	if SystemClock().Location() != time.UTC {
		t.Fatalf("the clock is not in UTC")
	}
}
