// moderation_dismissal_impl.go — G79's third outcome, on GORM. Its own file
// rather than more of moderation_impl.go, mirroring the gateway's original
// split (store_postgres_moderation_dismissal.go, [[file-length-limit]]).
package mwanachamacomm

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (s *ModerationStore) DismissReports(ctx context.Context, d models.Dismissal, wall string, actor models.Actor) (models.Dismissal, error) {
	if d.DismissedAt.IsZero() {
		d.DismissedAt = s.clock()
	}
	row := gormstore.DismissalToRow(d)
	var out models.Dismissal
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Table(s.tables.Removals)
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var existing gormstore.RemovalRow
		err := q.Where("message_id = ?", d.MessageID).First(&existing).Error
		switch {
		case err == nil:
			return models.ErrAlreadyRemoved
		case errors.Is(err, gorm.ErrRecordNotFound):
			// The ordinary case: the post is standing, which is what makes
			// leaving it standing meaningful.
		default:
			return err
		}

		if err := tx.Table(s.tables.Dismissals).Create(&row).Error; err != nil {
			return err
		}
		out = gormstore.DismissalFromRow(row)
		sqlTx, err := sqlTxFrom(tx)
		if err != nil {
			return err
		}
		return s.actWriter.WriteAct(ctx, sqlTx, models.ReportLeftStandingAct(out, wall, actor))
	})
	if err != nil {
		return models.Dismissal{}, classify(err)
	}
	return out, nil
}

// GetDismissalForMessage returns the dismissal naming messageID, if any.
func (s *ModerationStore) GetDismissalForMessage(ctx context.Context, messageID string) (models.Dismissal, error) {
	var row gormstore.DismissalRow
	err := s.db.WithContext(ctx).Table(s.tables.Dismissals).Where("message_id = ?", messageID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Dismissal{}, models.ErrModerationNotFound
	}
	if err != nil {
		return models.Dismissal{}, err
	}
	return gormstore.DismissalFromRow(row), nil
}

var _ models.ModerationRepository = (*ModerationStore)(nil)
