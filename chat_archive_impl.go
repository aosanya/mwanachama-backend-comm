package mwanachamacomm

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (s *ChatStore) Archive(ctx context.Context, structureID, threadID, readerID string) (models.ChatArchive, error) {
	archive := models.ChatArchive{
		ID:          mintID(prefixChatArchive),
		StructureID: structureID,
		ThreadID:    threadID,
		ReaderID:    readerID,
		ArchivedAt:  s.clock(),
	}
	if err := Check(s.store.Spec(), roleChatArchive, archive); err != nil {
		return models.ChatArchive{}, err
	}
	row, err := encode(s.store.Object(roleChatArchive), archive)
	if err != nil {
		return models.ChatArchive{}, err
	}
	err = s.store.Query(ctx, roleChatArchive).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "thread_id"}, {Name: "reader_id"}},
			DoNothing: true,
		}).Create(row).Error
	if err != nil {
		return models.ChatArchive{}, classify(err)
	}
	var out models.ChatArchive
	q := s.store.Query(ctx, roleChatArchive).Where("thread_id = ? AND reader_id = ?", threadID, readerID)
	if err := s.store.Take(q, roleChatArchive, &out, models.ErrChatNotFound); err != nil {
		return models.ChatArchive{}, err
	}
	return out, nil
}

func (s *ChatStore) Unarchive(ctx context.Context, threadID, readerID string) error {
	return classify(deleteWhere(ctx, s.store, roleChatArchive, "thread_id = ? AND reader_id = ?", threadID, readerID))
}

func (s *ChatStore) ArchivedThreads(ctx context.Context, structureID, readerID string) (map[string]bool, error) {
	var threadIDs []string
	err := s.store.Query(ctx, roleChatArchive).
		Where("structure_id = ? AND reader_id = ?", structureID, readerID).
		Pluck("thread_id", &threadIDs).Error
	if err != nil {
		return nil, classify(err)
	}
	archived := make(map[string]bool, len(threadIDs))
	for _, id := range threadIDs {
		archived[id] = true
	}
	return archived, nil
}

func (m *CommManager) ArchiveChatThreadFor(ctx context.Context, structureID, threadID, readerID string) (models.ChatArchive, error) {
	if err := m.inThread(ctx, structureID, threadID, readerID); err != nil {
		return models.ChatArchive{}, err
	}
	return m.ChatStore.Archive(ctx, structureID, threadID, readerID)
}

func (m *CommManager) UnarchiveChatThreadFor(ctx context.Context, structureID, threadID, readerID string) error {
	if err := m.inThread(ctx, structureID, threadID, readerID); err != nil {
		return err
	}
	return m.ChatStore.Unarchive(ctx, threadID, readerID)
}
