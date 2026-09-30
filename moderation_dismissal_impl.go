package mwanachamacomm

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (s *ModerationStore) DismissReports(ctx context.Context, d models.Dismissal, wall string, actor models.Actor) (models.Dismissal, error) {
	if d.DismissedAt.IsZero() {
		d.DismissedAt = s.clock()
	}
	if d.ID == "" {
		d.ID = mintID(prefixDismissal)
	}
	row, err := encode(s.store.Object(roleDismissal), d)
	if err != nil {
		return models.Dismissal{}, err
	}
	err = s.store.Query(ctx, roleDismissal).Session(&gorm.Session{}).Transaction(func(tx *gorm.DB) error {
		q := tx.Table(s.store.Table(roleRemoval))
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var rows []map[string]any
		if err := q.Where("message_id = ?", d.MessageID).Limit(1).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			return models.ErrAlreadyRemoved
		}

		if err := tx.Table(s.store.Table(roleDismissal)).Create(row).Error; err != nil {
			return err
		}
		sqlTx, err := sqlTxFrom(tx)
		if err != nil {
			return err
		}
		return s.actWriter.WriteAct(ctx, sqlTx, models.ReportLeftStandingAct(d, wall, actor))
	})
	if err != nil {
		return models.Dismissal{}, classify(err)
	}
	return d, nil
}

func (s *ModerationStore) GetDismissalForMessage(ctx context.Context, messageID string) (models.Dismissal, error) {
	var d models.Dismissal
	q := s.store.Query(ctx, roleDismissal).Where("message_id = ?", messageID)
	if err := s.store.Take(q, roleDismissal, &d, models.ErrModerationNotFound); err != nil {
		return models.Dismissal{}, err
	}
	return d, nil
}

var _ models.ModerationRepository = (*ModerationStore)(nil)
