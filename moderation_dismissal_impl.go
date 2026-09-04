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

// DismissReports inserts a dismissal row and writes the chapter's
// `report_left_standing` act-log row in the same transaction — DEV-1351.
//
// **Already-withheld is checked with a locking read on Postgres**
// (`SELECT ... FOR UPDATE`), taking the removal row's lock inside this
// transaction so a concurrent CreateRemoval either lands before this check
// sees it or blocks until this commits — same guarantee the original
// Postgres store's FOR UPDATE gave. sqlite (this store's other dialect,
// tests only) has no FOR UPDATE syntax at all and needs none: nothing in
// this repo's test suite runs DismissReports and CreateRemoval
// concurrently against the same sqlite connection, so a plain read is
// equivalent there.
//
// **Already-dismissed** is the unique(message_id) index, classified as a
// conflict the same way one-removal-per-message is. Both refusals return
// before the log is touched, so the log holds exactly the dismissals that
// happened.
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
