package mwanachamacomm

// Sentinel errors and the classify() driver-error translation every store
// file uses. Moved here from the old store_postgres_helpers.go — the
// storage swap to GORM changed how these get produced (GORM wraps the same
// underlying pgx driver error), not what they mean.

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// ErrInvalidReference means the caller named a row that does not exist.
var ErrInvalidReference = errors.New("invalid reference")

// ErrConflict means the row the caller asked to create already exists — a
// duplicate primary key or a violated unique constraint.
var ErrConflict = errors.New("already exists")

type fieldError struct {
	err   error
	field string
}

func (e *fieldError) Error() string { return e.err.Error() + ": " + e.field }
func (e *fieldError) Unwrap() error { return e.err }

// Reference wraps ErrInvalidReference with the column at fault.
func Reference(field string) error {
	if field == "" {
		return ErrInvalidReference
	}
	return &fieldError{err: ErrInvalidReference, field: field}
}

// Conflict wraps ErrConflict with the field at fault.
func Conflict(field string) error {
	if field == "" {
		return ErrConflict
	}
	return &fieldError{err: ErrConflict, field: field}
}

// SQLSTATE class 23 is "integrity constraint violation".
const (
	sqlstateNotNullViolation    = "23502"
	sqlstateForeignKeyViolation = "23503"
	sqlstateUniqueViolation     = "23505"
	sqlstateCheckViolation      = "23514"
)

// classify maps a driver error onto ErrInvalidReference/ErrConflict where it
// can, on either dialect this package runs against. Postgres reports a
// structured pgconn.PgError with a SQLSTATE; sqlite (tests only) reports a
// plain-text driver error with no structured code at all, so that branch
// matches on the constraint-violation phrase modernc.org/sqlite's error
// text always contains. Returns err unchanged where neither matches.
func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case sqlstateForeignKeyViolation, sqlstateNotNullViolation, sqlstateCheckViolation:
			return Reference(constraintOf(pgErr))
		case sqlstateUniqueViolation:
			return Conflict(constraintOf(pgErr))
		default:
			return err
		}
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "UNIQUE constraint failed"):
		return Conflict(sqliteConstraintOf(msg))
	case strings.Contains(msg, "FOREIGN KEY constraint failed"),
		strings.Contains(msg, "NOT NULL constraint failed"),
		strings.Contains(msg, "CHECK constraint failed"):
		return Reference(sqliteConstraintOf(msg))
	default:
		return err
	}
}

// sqliteConstraintOf pulls the "table.column[, table.column...]" tail
// modernc.org/sqlite appends after "constraint failed: <KIND> constraint
// failed: " — the closest sqlite equivalent of pgconn.PgError's constraint
// name, good enough for a Reference/Conflict field label.
func sqliteConstraintOf(msg string) string {
	if i := strings.LastIndex(msg, "failed: "); i >= 0 {
		return msg[i+len("failed: "):]
	}
	return msg
}

func constraintOf(pgErr *pgconn.PgError) string {
	if pgErr.ConstraintName != "" {
		return pgErr.ConstraintName
	}
	if pgErr.ColumnName != "" {
		return pgErr.ColumnName
	}
	return pgErr.TableName
}

// flexTime scans a nullable timestamp column out of a raw (non-GORM-mapped)
// query result, where GORM's own dialector-aware scanning doesn't apply.
// Postgres's driver returns a real time.Time for this shape; sqlite (this
// package's other dialect, tests only) returns a plain string instead —
// flexTime accepts either, so Activity's hand-written aggregate query
// (chat_impl.go) works unchanged on both.
type flexTime struct {
	Time  time.Time
	Valid bool
}

// sqliteTimeLayouts covers the string shapes modernc.org/sqlite (via
// glebarez/sqlite) has been observed returning a time.Time value as when
// there is no declared column type for the scanner to key off of (true of
// any aggregate expression, e.g. Activity's max(created_at)).
var sqliteTimeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999Z07:00",
}

func (f *flexTime) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		f.Valid = false
		return nil
	case time.Time:
		f.Time, f.Valid = v, true
		return nil
	case string:
		return f.scanString(v)
	case []byte:
		return f.scanString(string(v))
	default:
		return fmt.Errorf("flexTime: unsupported Scan source %T", src)
	}
}

func (f *flexTime) scanString(s string) error {
	var lastErr error
	for _, layout := range sqliteTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			f.Time, f.Valid = t, true
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("flexTime: parsing %q: %w", s, lastErr)
}
