package mwanachamacomm

import (
	"errors"
	"fmt"
	"github.com/aosanya/mwanachama-backend-shared/consent"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var ErrInvalidReference = errors.New("invalid reference")

var ErrConflict = errors.New("already exists")

type fieldError struct {
	err   error
	field string
}

func (e *fieldError) Error() string { return e.err.Error() + ": " + e.field }
func (e *fieldError) Unwrap() error { return e.err }

func Reference(field string) error {
	if field == "" {
		return ErrInvalidReference
	}
	return &fieldError{err: ErrInvalidReference, field: field}
}

func Conflict(field string) error {
	if field == "" {
		return ErrConflict
	}
	return &fieldError{err: ErrConflict, field: field}
}

const (
	sqlstateNotNullViolation    = "23502"
	sqlstateForeignKeyViolation = "23503"
	sqlstateUniqueViolation     = "23505"
	sqlstateCheckViolation      = "23514"
)

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

type flexTime struct {
	Time  time.Time
	Valid bool
}

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

var (
	ErrConsentRequired  = consent.ErrRequired
	ErrConsentWithdrawn = consent.ErrWithdrawn
)
