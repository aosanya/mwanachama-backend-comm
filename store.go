package mwanachamacomm

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

func stamp(at time.Time) string { return at.UTC().Format(specstore.TimeLayout) }

func runClause(st *store, alias string) (string, []any) {
	if st.RunID() == "" {
		return "", nil
	}
	column := spec.RunIDColumn
	if alias != "" {
		column = alias + "." + spec.RunIDColumn
	}
	return " AND " + column + " = ?", []any{st.RunID()}
}

func conflictOn(st *store, columns ...string) []clause.Column {
	out := make([]clause.Column, 0, len(columns)+1)
	if st.RunID() != "" {
		out = append(out, clause.Column{Name: spec.RunIDColumn})
	}
	for _, c := range columns {
		out = append(out, clause.Column{Name: c})
	}
	return out
}

func inTx(tx *gorm.DB, st *store, role string) *gorm.DB {
	q := tx.Table(st.Table(role))
	if st.RunID() != "" {
		q = q.Where(spec.RunIDColumn+" = ?", st.RunID())
	}
	return q
}

func deleteWhere(ctx context.Context, st *store, role, where string, args ...any) error {
	scope, scopeArgs := runClause(st, "")
	return st.Query(ctx, role).Exec("delete from "+st.Table(role)+" where "+where+scope, append(args, scopeArgs...)...).Error
}

type store = specstore.Store

func newStore(db *gorm.DB, s *spec.Spec, carriers map[string]any) (*store, error) {
	return specstore.New(db, s, carriers)
}

func newID() string { return specstore.NewID() }

func mintID(prefix string) string { return prefix + "-" + specstore.NewID() }

const (
	prefixChatThread     = "cthread"
	prefixChatMessage    = "msg"
	prefixChatReadMarker = "cread"
	prefixChatArchive    = "carch"
	prefixSubjectThread  = "sthread"
	prefixSubjectMessage = "smsg"
	prefixDMThread       = "dm"
	prefixDMMessage      = "dmmsg"
	prefixDMDeviceKey    = "dkey"
	prefixReport         = "modreport"
	prefixRemoval        = "modremoval"
	prefixDismissal      = "moddismissal"
	prefixDispute        = "moddispute"
)

func columnName(field string) string { return specstore.ColumnName(field) }

func encode(o spec.Object, v any) (map[string]any, error) { return specstore.Encode(o, v) }

func decode(o spec.Object, row map[string]any, out any) error {
	return specstore.Decode(o, row, out)
}
