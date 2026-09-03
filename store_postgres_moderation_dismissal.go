package mwanachamacomm

// G79's third outcome, on Postgres — DEV-1351. Its own file rather than more
// of store_postgres_moderation.go, mirroring the gateway's own split (that
// file was at 291 of the 300 lines [[file-length-limit]] allows before this
// landed). The seam is the act: everything here belongs to a report being
// left standing, and nothing there does.

import (
	"context"
	"database/sql"
	"errors"
)

const dismissalColumns = `id, message_id, chapter_id, dismissed_by, actor_role_class, reason, COALESCE(note,''), dismissed_at`

// DismissReports inserts a dismissal row and writes the chapter's
// `report_left_standing` act-log row in the same transaction — DEV-1351,
// closing the fourth of the four moderation kinds DEV-1341 owed.
//
// **Both refusals return before the log is touched**, so the log holds
// exactly the dismissals that happened:
//
//   - Already withheld → ErrAlreadyRemoved, checked inside the transaction so
//     a removal committing between the check and the insert cannot slip past
//     it. `Removed` and `Left standing` are two states of one post.
//   - Already dismissed → the unique constraint, classified as a conflict,
//     the same way one-removal-per-message is.
func (s *ModerationPostgresStore) DismissReports(ctx context.Context, d Dismissal, wall string, actor Actor) (Dismissal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Dismissal{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// SELECT ... FOR UPDATE rather than a plain existence check: it takes the
	// removal row's lock inside this transaction, so a concurrent
	// CreateRemoval either lands before this check sees it or blocks until
	// this commits.
	var removalID string
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM comm_message_removal WHERE message_id = $1 FOR UPDATE`, d.MessageID).Scan(&removalID)
	switch {
	case err == nil:
		return Dismissal{}, ErrAlreadyRemoved
	case errors.Is(err, sql.ErrNoRows):
		// The ordinary case: the post is standing, which is what makes
		// leaving it standing meaningful.
	default:
		return Dismissal{}, err
	}

	const q = `INSERT INTO comm_message_report_dismissal
	           (id, message_id, chapter_id, dismissed_by, actor_role_class, reason, note, dismissed_at)
	           VALUES (COALESCE(NULLIF($1,''), 'moddismissal-' || nextval('comm_message_report_dismissal_seq')),
	                   $2, $3, $4, $5, $6, NULLIF($7,''), COALESCE($8, now()))
	           RETURNING ` + dismissalColumns
	out, err := scanDismissal(tx.QueryRowContext(ctx, q,
		d.ID, d.MessageID, d.ChapterID, d.DismissedBy, d.ActorRoleClass, string(d.Reason), d.Note, nullTime(d.DismissedAt)))
	if err != nil {
		return Dismissal{}, classify(err)
	}
	if err := s.actWriter.WriteAct(ctx, tx, ReportLeftStandingAct(out, wall, actor)); err != nil {
		return Dismissal{}, err
	}
	return out, tx.Commit()
}

// GetDismissalForMessage returns the dismissal naming messageID, if any.
func (s *ModerationPostgresStore) GetDismissalForMessage(ctx context.Context, messageID string) (Dismissal, error) {
	const q = `SELECT ` + dismissalColumns + ` FROM comm_message_report_dismissal WHERE message_id = $1`
	d, err := scanDismissal(s.db.QueryRowContext(ctx, q, messageID))
	if errors.Is(err, sql.ErrNoRows) {
		return Dismissal{}, ErrModerationNotFound
	}
	return d, err
}

// scanDismissal reads one row of dismissalColumns.
func scanDismissal(r rowScanner) (Dismissal, error) {
	var (
		d      Dismissal
		reason string
	)
	err := r.Scan(&d.ID, &d.MessageID, &d.ChapterID, &d.DismissedBy, &d.ActorRoleClass, &reason, &d.Note, &d.DismissedAt)
	if err != nil {
		return Dismissal{}, err
	}
	d.Reason = RemovalReason(reason)
	return d, nil
}
