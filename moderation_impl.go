package mwanachamacomm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type ModerationStore struct {
	store     *store
	clock     Clock
	actWriter models.ActWriter
}

func NewModerationStore(db *gorm.DB, s *spec.Spec, clock Clock, actWriter models.ActWriter) (*ModerationStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewModerationStore: db must not be nil")
	}
	if actWriter == nil {
		return nil, fmt.Errorf("NewModerationStore: actWriter must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	st, err := newStore(db, s, map[string]any{
		roleReport:    models.Report{},
		roleRemoval:   models.Removal{},
		roleDismissal: models.Dismissal{},
		roleDispute:   models.Dispute{},
	})
	if err != nil {
		return nil, err
	}
	return &ModerationStore{store: st, clock: clock, actWriter: actWriter}, nil
}

func sqlTxFrom(tx *gorm.DB) (*sql.Tx, error) {
	sqlTx, ok := tx.Statement.ConnPool.(*sql.Tx)
	if !ok {
		return nil, fmt.Errorf("mwanachamacomm: GORM transaction's ConnPool is %T, not *sql.Tx", tx.Statement.ConnPool)
	}
	return sqlTx, nil
}

func (s *ModerationStore) FileReport(ctx context.Context, r models.Report) (models.Report, error) {
	if r.ReportedAt.IsZero() {
		r.ReportedAt = s.clock()
	}
	if r.ID == "" {
		r.ID = mintID(prefixReport)
	}
	if err := s.store.Insert(ctx, roleReport, r); err != nil {
		return models.Report{}, classify(err)
	}
	return r, nil
}

func (s *ModerationStore) ListReportsForMessage(ctx context.Context, messageID string) ([]models.Report, error) {
	q := s.store.Query(ctx, roleReport).
		Where("message_id = ?", messageID).Order("reported_at DESC, id DESC")
	return specstore.List[models.Report](s.store, q, roleReport)
}

func (s *ModerationStore) ListReportQueue(ctx context.Context, structureID string) ([]models.Report, error) {
	q := s.store.Query(ctx, roleReport).
		Where("structure_id = ?", structureID).Order("reported_at DESC, id DESC")
	return specstore.List[models.Report](s.store, q, roleReport)
}

func (s *ModerationStore) CreateRemoval(ctx context.Context, rem models.Removal, wall string, actor models.Actor) (models.Removal, error) {
	if rem.RemovedAt.IsZero() {
		rem.RemovedAt = s.clock()
	}
	if rem.ID == "" {
		rem.ID = mintID(prefixRemoval)
	}
	row, err := encode(s.store.Object(roleRemoval), rem)
	if err != nil {
		return models.Removal{}, err
	}

	err = s.store.Query(ctx, roleRemoval).Session(&gorm.Session{}).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(s.store.Table(roleRemoval)).Create(row).Error; err != nil {
			return err
		}
		sqlTx, err := sqlTxFrom(tx)
		if err != nil {
			return err
		}
		return s.actWriter.WriteAct(ctx, sqlTx, models.WithheldAct(rem, wall, actor))
	})
	if err != nil {
		return models.Removal{}, classify(err)
	}
	return rem, nil
}

func (s *ModerationStore) GetRemoval(ctx context.Context, id string) (models.Removal, error) {
	var rem models.Removal
	q := s.store.Query(ctx, roleRemoval).Where("id = ?", id)
	if err := s.store.Take(q, roleRemoval, &rem, models.ErrModerationNotFound); err != nil {
		return models.Removal{}, err
	}
	return rem, nil
}

func (s *ModerationStore) GetRemovalForMessage(ctx context.Context, messageID string) (models.Removal, error) {
	var rem models.Removal
	q := s.store.Query(ctx, roleRemoval).Where("message_id = ?", messageID)
	if err := s.store.Take(q, roleRemoval, &rem, models.ErrModerationNotFound); err != nil {
		return models.Removal{}, err
	}
	return rem, nil
}

func (s *ModerationStore) ListRemovalsForStructure(ctx context.Context, structureID string) ([]models.Removal, error) {
	q := s.store.Query(ctx, roleRemoval).
		Where("structure_id = ?", structureID).Order("removed_at DESC, id DESC")
	return specstore.List[models.Removal](s.store, q, roleRemoval)
}

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
	if d.ID == "" {
		d.ID = mintID(prefixDispute)
	}
	d.State = models.DisputeOpen
	d.DecidedBy = ""
	d.DecidedAt = nil

	if err := s.store.Insert(ctx, roleDispute, d); err != nil {
		return models.Dispute{}, classify(err)
	}
	return d, nil
}

func (s *ModerationStore) GetDispute(ctx context.Context, id string) (models.Dispute, error) {
	var d models.Dispute
	q := s.store.Query(ctx, roleDispute).Where("id = ?", id)
	if err := s.store.Take(q, roleDispute, &d, models.ErrModerationNotFound); err != nil {
		return models.Dispute{}, err
	}
	return d, nil
}

func (s *ModerationStore) GetDisputeForRemoval(ctx context.Context, removalID string) (models.Dispute, error) {
	var d models.Dispute
	q := s.store.Query(ctx, roleDispute).Where("removal_id = ?", removalID)
	if err := s.store.Take(q, roleDispute, &d, models.ErrModerationNotFound); err != nil {
		return models.Dispute{}, err
	}
	return d, nil
}

func (s *ModerationStore) DecideDispute(ctx context.Context, id string, outcome models.DisputeState, decidedBy string, now time.Time, wall string, actor models.Actor) (models.Dispute, error) {
	var out models.Dispute
	disputes, removals := s.store.Object(roleDispute), s.store.Object(roleRemoval)

	err := s.store.Query(ctx, roleDispute).Session(&gorm.Session{}).Transaction(func(tx *gorm.DB) error {
		var d models.Dispute
		if err := takeIn(tx, s.store, roleDispute, disputes, &d,
			models.ErrModerationNotFound, "id = ?", id); err != nil {
			return err
		}
		if d.State != models.DisputeOpen {
			return models.ErrAlreadyDecided
		}

		var rem models.Removal
		if err := takeIn(tx, s.store, roleRemoval, removals, &rem,
			models.ErrModerationNotFound, "id = ?", d.RemovalID); err != nil {
			return err
		}
		if rem.RemovedBy == decidedBy {
			return models.ErrReviewerIsRemover
		}

		if err := tx.Table(s.store.Table(roleDispute)).Where("id = ?", id).
			Updates(map[string]any{
				"state": string(outcome), "decided_by": decidedBy, "decided_at": stamp(now),
			}).Error; err != nil {
			return err
		}

		if err := takeIn(tx, s.store, roleDispute, disputes, &out,
			models.ErrModerationNotFound, "id = ?", id); err != nil {
			return err
		}

		sqlTx, err := sqlTxFrom(tx)
		if err != nil {
			return err
		}
		return s.actWriter.WriteAct(ctx, sqlTx, models.DisputeOutcomeAct(rem, out, wall, actor))
	})
	if err != nil {
		return models.Dispute{}, err
	}
	return out, nil
}

func takeIn(tx *gorm.DB, st *store, role string, o spec.Object, out any, notFound error, where string, args ...any) error {
	var rows []map[string]any
	if err := tx.Table(st.Table(role)).Where(where, args...).Limit(1).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return notFound
	}
	return decode(o, rows[0], out)
}
