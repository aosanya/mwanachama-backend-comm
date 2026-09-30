package mwanachamacomm

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

func stamp(at time.Time) string { return at.UTC().Format(specstore.TimeLayout) }

func deleteWhere(ctx context.Context, st *store, role, where string, args ...any) error {
	return st.Query(ctx, role).Exec("delete from "+st.Table(role)+" where "+where, args...).Error
}

type store = specstore.Store

func newStore(db *gorm.DB, s *spec.Spec, carriers map[string]any) (*store, error) {
	return specstore.New(db, s, carriers)
}

func newID() string { return specstore.NewID() }

func mintID(prefix string) string { return prefix + "-" + specstore.NewID() }

const (
	prefixChatThread  = "cthread"
	prefixChatMessage = "msg"
	prefixDMThread    = "dm"
	prefixDMMessage   = "dmmsg"
	prefixDMDeviceKey = "dkey"
	prefixReport      = "modreport"
	prefixRemoval     = "modremoval"
	prefixDismissal   = "moddismissal"
	prefixDispute     = "moddispute"
)

func columnName(field string) string { return specstore.ColumnName(field) }

func encode(o spec.Object, v any) (map[string]any, error) { return specstore.Encode(o, v) }

func decode(o spec.Object, row map[string]any, out any) error {
	return specstore.Decode(o, row, out)
}
