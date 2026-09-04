// dm_roster_impl.go — GORM-backed DMRepository implementation, roster-write
// half: Invite/Accept/Leave/Kick/Promote/ReEnable. See dm_impl.go's package
// doc for why this port's single GORM store needs a three-way rather than
// the original module's two-way file split.
package mwanachamacomm

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

var errNotAdmin = errors.New("directmessage: not admin")

// findParticipant returns the roster row for (threadID, memberID), if one
// exists.
func (s *DMStore) findParticipant(ctx context.Context, threadID, memberID string) (gormstore.DMParticipantRow, bool, error) {
	var row gormstore.DMParticipantRow
	err := s.db.WithContext(ctx).Table(s.tables.DMParticipants).
		Where("thread_id = ? AND member_id = ?", threadID, memberID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return gormstore.DMParticipantRow{}, false, nil
	}
	if err != nil {
		return gormstore.DMParticipantRow{}, false, err
	}
	return row, true, nil
}

func (s *DMStore) isAdmin(ctx context.Context, threadID, memberID string) (bool, error) {
	row, found, err := s.findParticipant(ctx, threadID, memberID)
	if err != nil || !found {
		return false, err
	}
	return row.IsAdmin && row.State == string(models.DMStateActive), nil
}

// Invite adds an invited member (admin-only). Re-inviting somebody still
// pending flips state back to invited and stamps updated_at; re-inviting
// somebody who has already accepted is refused (ErrDMAlreadyActive) rather
// than silently taking their acceptance away.
func (s *DMStore) Invite(ctx context.Context, threadID, memberID, by string) (models.DMParticipant, error) {
	ok, err := s.isAdmin(ctx, threadID, by)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !ok {
		return models.DMParticipant{}, errNotAdmin
	}
	existing, found, err := s.findParticipant(ctx, threadID, memberID)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if found && existing.State == string(models.DMStateActive) {
		return models.DMParticipant{}, models.ErrDMAlreadyActive
	}
	return s.upsertParticipant(ctx, threadID, memberID)
}

// Accept turns an invite into active membership.
func (s *DMStore) Accept(ctx context.Context, threadID, memberID string) (models.DMParticipant, error) {
	return s.setState(ctx, threadID, memberID, models.DMStateActive)
}

// Leave marks the member as having left.
func (s *DMStore) Leave(ctx context.Context, threadID, memberID string) error {
	_, err := s.setState(ctx, threadID, memberID, models.DMStateLeft)
	return err
}

// Kick marks the member as removed by an admin.
func (s *DMStore) Kick(ctx context.Context, threadID, memberID, by string) error {
	ok, err := s.isAdmin(ctx, threadID, by)
	if err != nil {
		return err
	}
	if !ok {
		return errNotAdmin
	}
	_, err = s.setState(ctx, threadID, memberID, models.DMStateKicked)
	return err
}

// Promote makes the target an admin (admin-only). A plain UPDATE, not an
// upsert: promoting somebody who was never on the roster is ErrDMNotFound,
// not a silently-created row — matches the memory store's original
// contract (the old Postgres store's RETURNING-only Promote had drifted
// from this, returning a bare sql.ErrNoRows on the same case; this port
// picks the memory store's behaviour since it's the one every business-rule
// test already assumes).
func (s *DMStore) Promote(ctx context.Context, threadID, memberID, by string) (models.DMParticipant, error) {
	ok, err := s.isAdmin(ctx, threadID, by)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !ok {
		return models.DMParticipant{}, errNotAdmin
	}
	res := s.db.WithContext(ctx).Table(s.tables.DMParticipants).
		Where("thread_id = ? AND member_id = ?", threadID, memberID).
		Updates(map[string]any{"is_admin": true, "updated_at": s.clock()})
	if res.Error != nil {
		return models.DMParticipant{}, classify(res.Error)
	}
	if res.RowsAffected == 0 {
		return models.DMParticipant{}, models.ErrDMNotFound
	}
	row, found, err := s.findParticipant(ctx, threadID, memberID)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !found {
		return models.DMParticipant{}, models.ErrDMNotFound
	}
	return gormstore.DMParticipantFromRow(row), nil
}

// ReEnable is the two acts documented on [models.DMRepository.ReEnable]: the
// member who was last to leave bringing the thread back active (memberID ==
// by, nobody else active, their own row `left`), and an admin restoring
// somebody else to `invited`.
func (s *DMStore) ReEnable(ctx context.Context, threadID, memberID, by string) (models.DMParticipant, error) {
	row, found, err := s.findParticipant(ctx, threadID, memberID)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !found {
		return models.DMParticipant{}, models.ErrDMNotFound
	}
	if memberID == by {
		active, err := s.hasActive(ctx, threadID)
		if err != nil {
			return models.DMParticipant{}, err
		}
		if row.State != string(models.DMStateLeft) || active {
			return models.DMParticipant{}, models.ErrDMNotLastToLeave
		}
		return s.setState(ctx, threadID, memberID, models.DMStateActive)
	}
	ok, err := s.isAdmin(ctx, threadID, by)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !ok {
		return models.DMParticipant{}, errNotAdmin
	}
	return s.setState(ctx, threadID, memberID, models.DMStateInvited)
}

// hasActive reports whether anybody is currently active on the thread.
func (s *DMStore) hasActive(ctx context.Context, threadID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Table(s.tables.DMParticipants).
		Where("thread_id = ? AND state = ?", threadID, string(models.DMStateActive)).Count(&count).Error
	return count > 0, err
}

// setState updates a participant's state, returning the fresh row (or
// ErrDMNotFound if there was nothing to update).
func (s *DMStore) setState(ctx context.Context, threadID, memberID string, state models.DMParticipantState) (models.DMParticipant, error) {
	res := s.db.WithContext(ctx).Table(s.tables.DMParticipants).
		Where("thread_id = ? AND member_id = ?", threadID, memberID).
		Updates(map[string]any{"state": string(state), "updated_at": s.clock()})
	if res.Error != nil {
		return models.DMParticipant{}, classify(res.Error)
	}
	if res.RowsAffected == 0 {
		return models.DMParticipant{}, models.ErrDMNotFound
	}
	row, found, err := s.findParticipant(ctx, threadID, memberID)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !found {
		return models.DMParticipant{}, models.ErrDMNotFound
	}
	return gormstore.DMParticipantFromRow(row), nil
}

// upsertParticipant inserts a new invite, or — if the pair already has a
// row (Invite's own caller already refused the ErrDMAlreadyActive case) —
// flips it back to invited without touching is_admin, preserving whatever
// admin flag the row already carried across a re-invite.
func (s *DMStore) upsertParticipant(ctx context.Context, threadID, memberID string) (models.DMParticipant, error) {
	row := gormstore.DMParticipantRow{ThreadID: threadID, MemberID: memberID, State: string(models.DMStateInvited), UpdatedAt: s.clock()}
	err := s.db.WithContext(ctx).Table(s.tables.DMParticipants).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "thread_id"}, {Name: "member_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"state", "updated_at"}),
		}).Create(&row).Error
	if err != nil {
		return models.DMParticipant{}, classify(err)
	}
	updated, found, err := s.findParticipant(ctx, threadID, memberID)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !found {
		return models.DMParticipant{}, models.ErrDMNotFound
	}
	return gormstore.DMParticipantFromRow(updated), nil
}
