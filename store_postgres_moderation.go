package mwanachamacomm

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ModerationPostgresStore is the Postgres implementation of
// ModerationRepository. Tables: comm_message_report, comm_message_removal,
// comm_removal_dispute, comm_message_report_dismissal (the last lives in
// store_postgres_moderation_dismissal.go, split for the same file-length
// reason the gateway's original split it).
//
// actWriter is the seam CreateRemoval/DismissReports/DecideDispute use to
// write the gateway's chapter act log in the same transaction as the
// moderation row — see moderation_act.go's package doc for why this store
// cannot write chapter_act_log_entry itself.
type ModerationPostgresStore struct {
	db        *sql.DB
	actWriter ActWriter
}

// NewModerationPostgresStore constructs a store over the given pool. db must
// be the SAME *sql.DB the gateway's custody store uses, so actWriter's
// insert lands in the same *sql.Tx as the moderation write.
func NewModerationPostgresStore(db *sql.DB, actWriter ActWriter) *ModerationPostgresStore {
	return &ModerationPostgresStore{db: db, actWriter: actWriter}
}

const reportColumns = `id, message_id, chapter_id, reported_by, reporter_role_class, reason, COALESCE(note,''), excerpt, reported_at`

// FileReport inserts a report row.
func (s *ModerationPostgresStore) FileReport(ctx context.Context, r Report) (Report, error) {
	const q = `INSERT INTO comm_message_report
	           (id, message_id, chapter_id, reported_by, reporter_role_class, reason, note, excerpt, reported_at)
	           VALUES (COALESCE(NULLIF($1,''), 'modreport-' || nextval('comm_message_report_seq')),
	                   $2, $3, $4, $5, $6, NULLIF($7,''), $8, COALESCE($9, now()))
	           RETURNING ` + reportColumns
	out, err := scanReport(s.db.QueryRowContext(ctx, q,
		r.ID, r.MessageID, r.ChapterID, r.ReportedBy, r.ReporterRoleClass, string(r.Reason), r.Note, r.Excerpt, nullTime(r.ReportedAt)))
	if err != nil {
		return Report{}, classify(err)
	}
	return out, nil
}

// ListReportsForMessage returns every report against one message, newest
// first.
func (s *ModerationPostgresStore) ListReportsForMessage(ctx context.Context, messageID string) ([]Report, error) {
	const q = `SELECT ` + reportColumns + ` FROM comm_message_report WHERE message_id = $1 ORDER BY reported_at DESC, id DESC`
	return queryReports(ctx, s.db, q, messageID)
}

// ListReportQueue returns every report filed against a message in
// chapterID, newest first.
func (s *ModerationPostgresStore) ListReportQueue(ctx context.Context, chapterID string) ([]Report, error) {
	const q = `SELECT ` + reportColumns + ` FROM comm_message_report WHERE chapter_id = $1 ORDER BY reported_at DESC, id DESC`
	return queryReports(ctx, s.db, q, chapterID)
}

func queryReports(ctx context.Context, db *sql.DB, q, arg string) ([]Report, error) {
	rows, err := db.QueryContext(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Report{}
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const removalColumns = `id, message_id, chapter_id, removed_by, actor_role_class, reason, removed_at`

// CreateRemoval inserts a removal row and writes the chapter's act-log row in
// the same transaction — DEV-1341.
//
// The act is composed from the row the INSERT returned, not from the argument:
// the id and `removed_at` are minted by the server, and a log row composed
// from the request would name a removal time the database never agreed to.
//
// **The conflict returns before the log is touched**, so a second removal of
// an already-withheld post writes no second row.
func (s *ModerationPostgresStore) CreateRemoval(ctx context.Context, rem Removal, wall string, actor Actor) (Removal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Removal{}, err
	}
	defer func() { _ = tx.Rollback() }()

	const q = `INSERT INTO comm_message_removal
	           (id, message_id, chapter_id, removed_by, actor_role_class, reason, removed_at)
	           VALUES (COALESCE(NULLIF($1,''), 'modremoval-' || nextval('comm_message_removal_seq')),
	                   $2, $3, $4, $5, $6, COALESCE($7, now()))
	           RETURNING ` + removalColumns
	out, err := scanRemoval(tx.QueryRowContext(ctx, q,
		rem.ID, rem.MessageID, rem.ChapterID, rem.RemovedBy, rem.ActorRoleClass, string(rem.Reason), nullTime(rem.RemovedAt)))
	if err != nil {
		return Removal{}, classify(err)
	}
	if err := s.actWriter.WriteAct(ctx, tx, WithheldAct(out, wall, actor)); err != nil {
		return Removal{}, err
	}
	return out, tx.Commit()
}

// GetRemoval returns one removal by id.
func (s *ModerationPostgresStore) GetRemoval(ctx context.Context, id string) (Removal, error) {
	const q = `SELECT ` + removalColumns + ` FROM comm_message_removal WHERE id = $1`
	rem, err := scanRemoval(s.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Removal{}, ErrModerationNotFound
	}
	return rem, err
}

// GetRemovalForMessage returns the removal naming messageID, if any.
func (s *ModerationPostgresStore) GetRemovalForMessage(ctx context.Context, messageID string) (Removal, error) {
	const q = `SELECT ` + removalColumns + ` FROM comm_message_removal WHERE message_id = $1`
	rem, err := scanRemoval(s.db.QueryRowContext(ctx, q, messageID))
	if errors.Is(err, sql.ErrNoRows) {
		return Removal{}, ErrModerationNotFound
	}
	return rem, err
}

// ListRemovalsForChapter returns every removal at chapterID, newest first.
func (s *ModerationPostgresStore) ListRemovalsForChapter(ctx context.Context, chapterID string) ([]Removal, error) {
	const q = `SELECT ` + removalColumns + ` FROM comm_message_removal WHERE chapter_id = $1 ORDER BY removed_at DESC, id DESC`
	rows, err := s.db.QueryContext(ctx, q, chapterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Removal{}
	for rows.Next() {
		rem, err := scanRemoval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rem)
	}
	return out, rows.Err()
}

const disputeColumns = `id, removal_id, raised_by, statement, review_chapter_id, held_since, raised_at, state, COALESCE(decided_by,''), decided_at`

// CreateDispute inserts a dispute row in the Open state.
func (s *ModerationPostgresStore) CreateDispute(ctx context.Context, d Dispute) (Dispute, error) {
	const q = `INSERT INTO comm_removal_dispute
	           (id, removal_id, raised_by, statement, review_chapter_id, held_since, raised_at, state)
	           VALUES (COALESCE(NULLIF($1,''), 'moddispute-' || nextval('comm_removal_dispute_seq')),
	                   $2, $3, $4, $5, COALESCE($6, now()), COALESCE($7, now()), 'open')
	           RETURNING ` + disputeColumns
	out, err := scanDispute(s.db.QueryRowContext(ctx, q,
		d.ID, d.RemovalID, d.RaisedBy, d.Statement, d.ReviewChapterID, nullTime(d.HeldSince), nullTime(d.RaisedAt)))
	if err != nil {
		return Dispute{}, classify(err)
	}
	return out, nil
}

// GetDispute returns one dispute by id.
func (s *ModerationPostgresStore) GetDispute(ctx context.Context, id string) (Dispute, error) {
	const q = `SELECT ` + disputeColumns + ` FROM comm_removal_dispute WHERE id = $1`
	d, err := scanDispute(s.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Dispute{}, ErrModerationNotFound
	}
	return d, err
}

// GetDisputeForRemoval returns the dispute naming removalID, if any.
func (s *ModerationPostgresStore) GetDisputeForRemoval(ctx context.Context, removalID string) (Dispute, error) {
	const q = `SELECT ` + disputeColumns + ` FROM comm_removal_dispute WHERE removal_id = $1`
	d, err := scanDispute(s.db.QueryRowContext(ctx, q, removalID))
	if errors.Is(err, sql.ErrNoRows) {
		return Dispute{}, ErrModerationNotFound
	}
	return d, err
}

// DecideDispute moves a dispute from Open to outcome, enforcing G62
// (reviewer != remover) and single-shot decision inside one transaction so
// the read-then-write cannot race a second decide call.
func (s *ModerationPostgresStore) DecideDispute(ctx context.Context, id string, outcome DisputeState, decidedBy string, now time.Time, wall string, actor Actor) (Dispute, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Dispute{}, err
	}
	defer tx.Rollback()

	var (
		state     string
		removalID string
	)
	err = tx.QueryRowContext(ctx, `SELECT state, removal_id FROM comm_removal_dispute WHERE id = $1`, id).Scan(&state, &removalID)
	if errors.Is(err, sql.ErrNoRows) {
		return Dispute{}, ErrModerationNotFound
	}
	if err != nil {
		return Dispute{}, err
	}
	if state != string(DisputeOpen) {
		return Dispute{}, ErrAlreadyDecided
	}

	var removedBy string
	if err := tx.QueryRowContext(ctx, `SELECT removed_by FROM comm_message_removal WHERE id = $1`, removalID).Scan(&removedBy); err != nil {
		return Dispute{}, err
	}
	if removedBy == decidedBy {
		return Dispute{}, ErrReviewerIsRemover
	}

	const q = `UPDATE comm_removal_dispute SET state = $1, decided_by = $2, decided_at = $3 WHERE id = $4 RETURNING ` + disputeColumns
	out, err := scanDispute(tx.QueryRowContext(ctx, q, string(outcome), decidedBy, now, id))
	if err != nil {
		return Dispute{}, classify(err)
	}
	// DEV-1341 · the outcome's act-log row, in the same transaction as the
	// decision. Both refusals above return before this point, so only a
	// decision that happened is logged. The removal is read here rather than
	// passed in: the act names the post, and the caller has only the
	// dispute's id.
	rem, err := scanRemoval(tx.QueryRowContext(ctx,
		`SELECT `+removalColumns+` FROM comm_message_removal WHERE id = $1`, removalID))
	if err != nil {
		return Dispute{}, classify(err)
	}
	if err := s.actWriter.WriteAct(ctx, tx, DisputeOutcomeAct(rem, out, wall, actor)); err != nil {
		return Dispute{}, err
	}
	if err := tx.Commit(); err != nil {
		return Dispute{}, err
	}
	return out, nil
}

// scanReport reads one row of reportColumns.
func scanReport(r rowScanner) (Report, error) {
	var (
		rep    Report
		reason string
	)
	err := r.Scan(&rep.ID, &rep.MessageID, &rep.ChapterID, &rep.ReportedBy, &rep.ReporterRoleClass,
		&reason, &rep.Note, &rep.Excerpt, &rep.ReportedAt)
	if err != nil {
		return Report{}, err
	}
	rep.Reason = ReportReason(reason)
	return rep, nil
}

// scanRemoval reads one row of removalColumns.
func scanRemoval(r rowScanner) (Removal, error) {
	var (
		rem    Removal
		reason string
	)
	err := r.Scan(&rem.ID, &rem.MessageID, &rem.ChapterID, &rem.RemovedBy, &rem.ActorRoleClass, &reason, &rem.RemovedAt)
	if err != nil {
		return Removal{}, err
	}
	rem.Reason = RemovalReason(reason)
	return rem, nil
}

// scanDispute reads one row of disputeColumns.
func scanDispute(r rowScanner) (Dispute, error) {
	var (
		d         Dispute
		state     string
		decidedBy string
		decidedAt sql.NullTime
	)
	err := r.Scan(&d.ID, &d.RemovalID, &d.RaisedBy, &d.Statement, &d.ReviewChapterID,
		&d.HeldSince, &d.RaisedAt, &state, &decidedBy, &decidedAt)
	if err != nil {
		return Dispute{}, err
	}
	d.State = DisputeState(state)
	d.DecidedBy = decidedBy
	if decidedAt.Valid {
		t := decidedAt.Time
		d.DecidedAt = &t
	}
	return d, nil
}

var _ ModerationRepository = (*ModerationPostgresStore)(nil)
