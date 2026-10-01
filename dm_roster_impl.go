package mwanachamacomm

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (s *DMStore) findParticipant(ctx context.Context, threadID, actorID string) (models.DMParticipant, bool, error) {
	var p models.DMParticipant
	q := s.store.Query(ctx, roleDMParticipant).
		Where("thread_id = ? AND actor_id = ?", threadID, actorID)
	err := s.store.Take(q, roleDMParticipant, &p, models.ErrDMNotFound)
	if errors.Is(err, models.ErrDMNotFound) {
		return models.DMParticipant{}, false, nil
	}
	if err != nil {
		return models.DMParticipant{}, false, err
	}
	return p, true, nil
}

func (s *DMStore) isAdmin(ctx context.Context, threadID, actorID string) (bool, error) {
	p, found, err := s.findParticipant(ctx, threadID, actorID)
	if err != nil || !found {
		return false, err
	}
	return p.IsAdmin && p.State == models.DMStateActive, nil
}

func (s *DMStore) Invite(ctx context.Context, threadID, actorID, by string) (models.DMParticipant, error) {
	ok, err := s.isAdmin(ctx, threadID, by)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !ok {
		return models.DMParticipant{}, models.ErrDMNotAdmin
	}
	existing, found, err := s.findParticipant(ctx, threadID, actorID)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if found && existing.State == models.DMStateActive {
		return models.DMParticipant{}, models.ErrDMAlreadyActive
	}
	return s.upsertParticipant(ctx, threadID, actorID)
}

func (s *DMStore) Accept(ctx context.Context, threadID, actorID string) (models.DMParticipant, error) {
	return s.setState(ctx, threadID, actorID, models.DMStateActive)
}

func (s *DMStore) Leave(ctx context.Context, threadID, actorID string) error {
	admin, err := s.isAdmin(ctx, threadID, actorID)
	if err != nil {
		return err
	}
	if admin {
		var activeAdmins, active int64
		base := s.store.Query(ctx, roleDMParticipant).
			Where("thread_id = ? AND state = ?", threadID, string(models.DMStateActive))
		if err := base.Session(&gorm.Session{}).Where("is_admin = ?", true).Count(&activeAdmins).Error; err != nil {
			return err
		}
		if err := base.Session(&gorm.Session{}).Count(&active).Error; err != nil {
			return err
		}
		if activeAdmins == 1 && active > 1 {
			return models.ErrDMLastAdmin
		}
	}
	_, err = s.setState(ctx, threadID, actorID, models.DMStateLeft)
	return err
}

func (s *DMStore) Kick(ctx context.Context, threadID, actorID, by string) error {
	if actorID == by {
		return models.ErrDMSelfKick
	}
	ok, err := s.isAdmin(ctx, threadID, by)
	if err != nil {
		return err
	}
	if !ok {
		return models.ErrDMNotAdmin
	}
	_, err = s.setState(ctx, threadID, actorID, models.DMStateKicked)
	return err
}

func (s *DMStore) Promote(ctx context.Context, threadID, actorID, by string) (models.DMParticipant, error) {
	ok, err := s.isAdmin(ctx, threadID, by)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !ok {
		return models.DMParticipant{}, models.ErrDMNotAdmin
	}
	res := s.store.Query(ctx, roleDMParticipant).
		Where("thread_id = ? AND actor_id = ?", threadID, actorID).
		Updates(map[string]any{"is_admin": true, "updated_at": stamp(s.clock())})
	if res.Error != nil {
		return models.DMParticipant{}, classify(res.Error)
	}
	if res.RowsAffected == 0 {
		return models.DMParticipant{}, models.ErrDMNotFound
	}
	return s.mustFindParticipant(ctx, threadID, actorID)
}

func (s *DMStore) ReEnable(ctx context.Context, threadID, actorID, by string) (models.DMParticipant, error) {
	_, found, err := s.findParticipant(ctx, threadID, actorID)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !found {
		return models.DMParticipant{}, models.ErrDMNotFound
	}
	if actorID == by {
		return s.claimEmptiedThread(ctx, threadID, actorID)
	}
	ok, err := s.isAdmin(ctx, threadID, by)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !ok {
		return models.DMParticipant{}, models.ErrDMNotAdmin
	}
	return s.setState(ctx, threadID, actorID, models.DMStateInvited)
}

func (s *DMStore) claimEmptiedThread(ctx context.Context, threadID, actorID string) (models.DMParticipant, error) {
	table := s.store.Table(roleDMParticipant)
	res := s.store.Query(ctx, roleDMParticipant).
		Where("thread_id = ? AND actor_id = ? AND state = ?", threadID, actorID, string(models.DMStateLeft)).
		Where("NOT EXISTS (SELECT 1 FROM "+table+" other WHERE other.thread_id = ? AND other.state = ?)",
			threadID, string(models.DMStateActive)).
		Updates(map[string]any{"state": string(models.DMStateActive), "updated_at": stamp(s.clock())})
	if res.Error != nil {
		return models.DMParticipant{}, classify(res.Error)
	}
	if res.RowsAffected == 0 {
		return models.DMParticipant{}, models.ErrDMNotLastToLeave
	}
	return s.mustFindParticipant(ctx, threadID, actorID)
}

func (s *DMStore) hasActive(ctx context.Context, threadID string) (bool, error) {
	var count int64
	err := s.store.Query(ctx, roleDMParticipant).
		Where("thread_id = ? AND state = ?", threadID, string(models.DMStateActive)).Count(&count).Error
	return count > 0, err
}

func (s *DMStore) setState(ctx context.Context, threadID, actorID string, state models.DMParticipantState) (models.DMParticipant, error) {
	res := s.store.Query(ctx, roleDMParticipant).
		Where("thread_id = ? AND actor_id = ?", threadID, actorID).
		Updates(map[string]any{"state": string(state), "updated_at": stamp(s.clock())})
	if res.Error != nil {
		return models.DMParticipant{}, classify(res.Error)
	}
	if res.RowsAffected == 0 {
		return models.DMParticipant{}, models.ErrDMNotFound
	}
	return s.mustFindParticipant(ctx, threadID, actorID)
}

func (s *DMStore) upsertParticipant(ctx context.Context, threadID, actorID string) (models.DMParticipant, error) {
	row, err := encode(s.store.Object(roleDMParticipant), models.DMParticipant{
		ThreadID: threadID, ActorID: actorID, State: models.DMStateInvited, UpdatedAt: s.clock(),
	})
	if err != nil {
		return models.DMParticipant{}, err
	}
	err = s.store.Query(ctx, roleDMParticipant).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "thread_id"}, {Name: "actor_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"state", "updated_at"}),
		}).Create(row).Error
	if err != nil {
		return models.DMParticipant{}, classify(err)
	}
	return s.mustFindParticipant(ctx, threadID, actorID)
}

func (s *DMStore) mustFindParticipant(ctx context.Context, threadID, actorID string) (models.DMParticipant, error) {
	p, found, err := s.findParticipant(ctx, threadID, actorID)
	if err != nil {
		return models.DMParticipant{}, err
	}
	if !found {
		return models.DMParticipant{}, models.ErrDMNotFound
	}
	return p, nil
}
