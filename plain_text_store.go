package mwanachamacomm

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-shared/plaintext"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

type store struct {
	*specstore.Store
}

func (st *store) ForRun(runID string) (*store, error) {
	inner, err := st.Store.ForRun(runID)
	if err != nil {
		return nil, err
	}
	return &store{Store: inner}, nil
}

func (st *store) Row(role string, v any) (map[string]any, error) {
	row, err := st.Store.Row(role, v)
	if err != nil {
		return nil, err
	}
	if err := refuseControlText(st, role, row); err != nil {
		return nil, err
	}
	return row, nil
}

func (st *store) Insert(ctx context.Context, role string, v any) error {
	row, err := st.Row(role, v)
	if err != nil {
		return err
	}
	return st.Query(ctx, role).Create(row).Error
}

func (st *store) InsertOnce(ctx context.Context, role string, v any) error {
	row, err := st.Row(role, v)
	if err != nil {
		return err
	}
	return st.Query(ctx, role).Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error
}

func refuseControlText(st *store, role string, row map[string]any) error {
	for _, f := range st.Object(role).Fields {
		text, isText := row[f.Name].(string)
		if !isText {
			continue
		}
		if err := refuseControlCharacters(role, f.Name, text); err != nil {
			return err
		}
	}
	return nil
}

func refuseControlCharacters(role, field, text string) error {
	if err := plaintext.CheckText(field, text); err != nil {
		return fmt.Errorf("%w: %v", invalid(role), err)
	}
	return nil
}

func list[T any](st *store, q *gorm.DB, role string) ([]T, error) {
	return specstore.List[T](st.Store, q, role)
}
