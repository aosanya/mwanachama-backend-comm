package mwanachamacomm

// Small helpers shared by the postgres store files, ported unchanged from
// mwanachama-backend-api-gateway's internal/store/postgres/{helpers,
// nullable_json,errors}.go and the itoa/rowScanner helpers scattered across
// that package. classify/Reference/Conflict/ErrInvalidReference/ErrConflict
// are comm's own copy of the gateway's internal/domain/domerr sentinels —
// this module cannot import a gateway-internal package, so the gateway's
// shared HTTP error mapping (internal/api/http/errors.go) checks both
// domerr's sentinels and these ones.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
// can, and returns it unchanged where it cannot.
func classify(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case sqlstateForeignKeyViolation, sqlstateNotNullViolation, sqlstateCheckViolation:
		return Reference(constraintOf(pgErr))
	case sqlstateUniqueViolation:
		return Conflict(constraintOf(pgErr))
	default:
		return err
	}
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

// nullStr maps Go's empty-string "unset" convention onto SQL NULL.
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// strOf reads a nullable text column back into Go's empty-string convention.
func strOf(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// nullString is nullStr's twin, kept as a second name to match the ported
// call sites (dm_device_key.published_by/device_id) exactly.
func nullString(s string) sql.NullString { return nullStr(s) }

// nullTime maps a zero time.Time (the store's "mint one for me" signal) onto
// SQL NULL, letting the column's DEFAULT now() fill it in.
func nullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// nullInt maps a nil *int onto SQL NULL.
func nullInt(n *int) sql.NullInt64 {
	if n == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*n), Valid: true}
}

// intPtr is nullInt's inverse.
func intPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// nullJSON maps an empty/nil JSON byte slice onto SQL NULL.
func nullJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}

// nullableJSON adapts a **nullable** `jsonb` column onto a `json.RawMessage`
// field (ported from DEV-1576's fix — see the gateway's nullable_json.go for
// the full incident writeup this guards against).
type nullableJSON struct{ dest *json.RawMessage }

func (n nullableJSON) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*n.dest = nil
	case []byte:
		b := make([]byte, len(v))
		copy(b, v)
		*n.dest = b
	case string:
		*n.dest = json.RawMessage(v)
	default:
		return fmt.Errorf("mwanachamacomm: cannot scan %T into json.RawMessage", src)
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// itoa avoids strconv just to keep this dependency-free, matching the
// gateway's own conn.go helper.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
