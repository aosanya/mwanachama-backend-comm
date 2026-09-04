// moderation_impl.go — GORM-backed ModerationRepository implementation:
// report/removal/dispute. Was store_postgres_moderation.go +
// store_memory_moderation.go; DismissReports lives in
// moderation_dismissal_impl.go (300-line split, mirrors the original file
// split — that file was already at the line-length limit before this
// landed).
package mwanachamacomm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

// ModerationStore is the GORM implementation of [models.ModerationRepository].
//
// actWriter is the seam CreateRemoval/DismissReports/DecideDispute use to
// write the gateway's chapter act log in the same transaction as the
// moderation row — see models/moderation_act.go's package doc for why this
// store cannot write chapter_act_log_entry itself. Its contract
// (WriteAct(ctx, *sql.Tx, ActEntry)) predates this store's move to GORM and
// is unaffected by it: sqlTxFrom recovers the *sql.Tx a GORM transaction
// wraps so actWriter never needs to know GORM is involved.
type ModerationStore struct {
	db        *gorm.DB
	tables    TableNames
	clock     Clock
	actWriter models.ActWriter
}

// NewModerationStore constructs a ModerationStore backed by db. actWriter
// must be non-nil: a store built without one withholds posts silently.
func NewModerationStore(db *gorm.DB, t TableNames, clock Clock, actWriter models.ActWriter) (*ModerationStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewModerationStore: db must not be nil")
	}
	if actWriter == nil {
		return nil, fmt.Errorf("NewModerationStore: actWriter must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	return &ModerationStore{db: db, tables: t, clock: clock, actWriter: actWriter}, nil
}

// sqlTxFrom recovers the *sql.Tx a `*gorm.DB.Transaction` callback wraps —
// GORM abstracts the connection as its own Statement.ConnPool, but every SQL
// dialect this repo uses (Postgres, and sqlite in tests) backs it with
// database/sql underneath, so the type assertion holds for both.
func sqlTxFrom(tx *gorm.DB) (*sql.Tx, error) {
	sqlTx, ok := tx.Statement.ConnPool.(*sql.Tx)
	if !ok {
		return nil, fmt.Errorf("mwanachamacomm: GORM transaction's ConnPool is %T, not *sql.Tx", tx.Statement.ConnPool)
	}
	return sqlTx, nil
}

// FileReport inserts a report row.
func (s *ModerationStore) FileReport(ctx context.Context, r models.Report) (models.Report, error) {
	if r.ReportedAt.IsZero() {
		r.ReportedAt = s.clock()
	}
	row := gormstore.ReportToRow(r)
	if err := s.db.WithContext(ctx).Table(s.tables.Reports).Create(&row).Error; err != nil {
		return models.Report{}, classify(err)
	}
	return gormstore.ReportFromRow(row), nil
}

// ListReportsForMessage returns every report against one message, newest
// first.
func (s *ModerationStore) ListReportsForMessage(ctx context.Context, messageID string) ([]models.Report, error) {
	var rows []gormstore.ReportRow
	err := s.db.WithContext(ctx).Table(s.tables.Reports).
		Where("message_id = ?", messageID).Order("reported_at DESC, id DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]models.Report, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.ReportFromRow(r))
	}
	return out, nil
}

// ListReportQueue returns every report filed against a message in
// chapterID, newest first.
func (s *ModerationStore) ListReportQueue(ctx context.Context, chapterID string) ([]models.Report, error) {
	var rows []gormstore.ReportRow
	err := s.db.WithContext(ctx).Table(s.tables.Reports).
		Where("chapter_id = ?", chapterID).Order("reported_at DESC, id DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]models.Report, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.ReportFromRow(r))
	}
	return out, nil
}

// CreateRemoval inserts a removal row and writes the chapter's act-log row
// in the same transaction — DEV-1341. The conflict (unique message_id)
// returns before the log is touched, since the failed Create aborts the
// transaction before the act write runs.
func (s *ModerationStore) CreateRemoval(ctx context.Context, rem models.Removal, wall string, actor models.Actor) (models.Removal, error) {
	if rem.RemovedAt.IsZero() {
		rem.RemovedAt = s.clock()
	}
	row := gormstore.RemovalToRow(rem)
	var out models.Removal
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(s.tables.Removals).Create(&row).Error; err != nil {
			return err
		}
		out = gormstore.RemovalFromRow(row)
		sqlTx, err := sqlTxFrom(tx)
		if err != nil {
			return err
		}
		return s.actWriter.WriteAct(ctx, sqlTx, models.WithheldAct(out, wall, actor))
	})
	if err != nil {
		return models.Removal{}, classify(err)
	}
	return out, nil
}

// GetRemoval returns one removal by id.
func (s *ModerationStore) GetRemoval(ctx context.Context, id string) (models.Removal, error) {
	var row gormstore.RemovalRow
	err := s.db.WithContext(ctx).Table(s.tables.Removals).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Removal{}, models.ErrModerationNotFound
	}
	if err != nil {
		return models.Removal{}, err
	}
	return gormstore.RemovalFromRow(row), nil
}

// GetRemovalForMessage returns the removal naming messageID, if any.
func (s *ModerationStore) GetRemovalForMessage(ctx context.Context, messageID string) (models.Removal, error) {
	var row gormstore.RemovalRow
	err := s.db.WithContext(ctx).Table(s.tables.Removals).Where("message_id = ?", messageID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Removal{}, models.ErrModerationNotFound
	}
	if err != nil {
		return models.Removal{}, err
	}
	return gormstore.RemovalFromRow(row), nil
}

// ListRemovalsForChapter returns every removal at chapterID, newest first.
func (s *ModerationStore) ListRemovalsForChapter(ctx context.Context, chapterID string) ([]models.Removal, error) {
	var rows []gormstore.RemovalRow
	err := s.db.WithContext(ctx).Table(s.tables.Removals).
		Where("chapter_id = ?", chapterID).Order("removed_at DESC, id DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]models.Removal, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.RemovalFromRow(r))
	}
	return out, nil
}

// CreateDispute inserts a dispute row in the Open state. Checks the removal
// exists first (ErrInvalidReference if not) rather than relying on a
// database foreign key — see gormstore's GroupRow doc (actor) for why a row
// whose table name is runtime-configurable doesn't declare a GORM
// association.
func (s *ModerationStore) CreateDispute(ctx context.Context, d models.Dispute) (models.Dispute, error) {
	if _, err := s.GetRemoval(ctx, d.RemovalID); err != nil {
		if errors.Is(err, models.ErrModerationNotFound) {
			return models.Dispute{}, Reference("removal_id")
		}
		return models.Dispute{}, err
	}
	now := s.clock()
	if d.RaisedAt.IsZero() {
		d.RaisedAt = now
	}
	if d.HeldSince.IsZero() {
		d.HeldSince = now
	}
	d.State = models.DisputeOpen
	d.DecidedBy = ""
	d.DecidedAt = nil
	row := gormstore.DisputeToRow(d)
	if err := s.db.WithContext(ctx).Table(s.tables.Disputes).Create(&row).Error; err != nil {
		return models.Dispute{}, classify(err)
	}
	return gormstore.DisputeFromRow(row), nil
}

// GetDispute returns one dispute by id.
func (s *ModerationStore) GetDispute(ctx context.Context, id string) (models.Dispute, error) {
	var row gormstore.DisputeRow
	err := s.db.WithContext(ctx).Table(s.tables.Disputes).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Dispute{}, models.ErrModerationNotFound
	}
	if err != nil {
		return models.Dispute{}, err
	}
	return gormstore.DisputeFromRow(row), nil
}

// GetDisputeForRemoval returns the dispute naming removalID, if any.
func (s *ModerationStore) GetDisputeForRemoval(ctx context.Context, removalID string) (models.Dispute, error) {
	var row gormstore.DisputeRow
	err := s.db.WithContext(ctx).Table(s.tables.Disputes).Where("removal_id = ?", removalID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Dispute{}, models.ErrModerationNotFound
	}
	if err != nil {
		return models.Dispute{}, err
	}
	return gormstore.DisputeFromRow(row), nil
}

// DecideDispute moves a dispute from Open to outcome, enforcing G62
// (reviewer != remover) and single-shot decision inside one transaction,
// and writes the outcome's act-log row in the same transaction. Both
// refusals return before the log is touched. No explicit row lock (the
// original Postgres store's DecideDispute had none either, unlike
// DismissReports' FOR UPDATE — carried over unchanged, not a new race).
func (s *ModerationStore) DecideDispute(ctx context.Context, id string, outcome models.DisputeState, decidedBy string, now time.Time, wall string, actor models.Actor) (models.Dispute, error) {
	var out models.Dispute
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var disputeRow gormstore.DisputeRow
		if err := tx.Table(s.tables.Disputes).Where("id = ?", id).First(&disputeRow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return models.ErrModerationNotFound
			}
			return err
		}
		if disputeRow.State != string(models.DisputeOpen) {
			return models.ErrAlreadyDecided
		}
		var removalRow gormstore.RemovalRow
		if err := tx.Table(s.tables.Removals).Where("id = ?", disputeRow.RemovalID).First(&removalRow).Error; err != nil {
			return err
		}
		if removalRow.RemovedBy == decidedBy {
			return models.ErrReviewerIsRemover
		}
		if err := tx.Table(s.tables.Disputes).Where("id = ?", id).
			Updates(map[string]any{"state": string(outcome), "decided_by": decidedBy, "decided_at": now}).Error; err != nil {
			return err
		}
		var updated gormstore.DisputeRow
		if err := tx.Table(s.tables.Disputes).Where("id = ?", id).First(&updated).Error; err != nil {
			return err
		}
		out = gormstore.DisputeFromRow(updated)

		sqlTx, err := sqlTxFrom(tx)
		if err != nil {
			return err
		}
		return s.actWriter.WriteAct(ctx, sqlTx, models.DisputeOutcomeAct(gormstore.RemovalFromRow(removalRow), out, wall, actor))
	})
	if err != nil {
		return models.Dispute{}, err
	}
	return out, nil
}
