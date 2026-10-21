package mwanachamacomm

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (s *ChatStore) MarkRead(ctx context.Context, structureID, threadID, readerID string, readAt time.Time) (models.ChatReadMarker, error) {
	marker := models.ChatReadMarker{
		ID:          mintID(prefixChatReadMarker),
		StructureID: structureID,
		ThreadID:    threadID,
		ReaderID:    readerID,
		ReadAt:      readAt.UTC(),
		MarkedAt:    s.clock(),
	}
	if err := Check(s.store.Spec(), roleChatReadMarker, marker); err != nil {
		return models.ChatReadMarker{}, err
	}
	row, err := encode(s.store.Object(roleChatReadMarker), marker)
	if err != nil {
		return models.ChatReadMarker{}, err
	}
	table := s.store.Table(roleChatReadMarker)
	err = s.store.Query(ctx, roleChatReadMarker).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "thread_id"}, {Name: "reader_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"read_at":   gorm.Expr("CASE WHEN excluded.read_at > " + table + ".read_at THEN excluded.read_at ELSE " + table + ".read_at END"),
				"marked_at": gorm.Expr("excluded.marked_at"),
			}),
		}).Create(row).Error
	if err != nil {
		return models.ChatReadMarker{}, classify(err)
	}
	return s.readMarker(ctx, threadID, readerID)
}

func (s *ChatStore) readMarker(ctx context.Context, threadID, readerID string) (models.ChatReadMarker, error) {
	var out models.ChatReadMarker
	q := s.store.Query(ctx, roleChatReadMarker).Where("thread_id = ? AND reader_id = ?", threadID, readerID)
	if err := s.store.Take(q, roleChatReadMarker, &out, models.ErrChatNotFound); err != nil {
		return models.ChatReadMarker{}, err
	}
	return out, nil
}

func (s *ChatStore) UnreadCounts(ctx context.Context, structureID, readerID string) (map[string]int, error) {
	query := `
	SELECT m.thread_id, count(*)
	  FROM ` + s.store.Table(roleChatMessage) + ` m
	  LEFT JOIN ` + s.store.Table(roleChatReadMarker) + ` r
	    ON r.thread_id = m.thread_id AND r.reader_id = ?
	 WHERE m.structure_id = ? AND m.thread_id <> '' AND m.author_id <> ?
	   AND (r.read_at IS NULL OR m.created_at > r.read_at)
	 GROUP BY m.thread_id`
	rows, err := s.store.Query(ctx, roleChatMessage).Raw(query, readerID, structureID, readerID).Rows()
	if err != nil {
		return nil, classify(err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var (
			threadID string
			n        int
		)
		if err := rows.Scan(&threadID, &n); err != nil {
			return nil, err
		}
		counts[threadID] = n
	}
	return counts, rows.Err()
}

func (s *ChatStore) LatestPosts(ctx context.Context, structureID string) (map[string]time.Time, error) {
	query := `
	SELECT thread_id, max(created_at)
	  FROM ` + s.store.Table(roleChatMessage) + `
	 WHERE structure_id = ? AND thread_id <> ''
	 GROUP BY thread_id`
	rows, err := s.store.Query(ctx, roleChatMessage).Raw(query, structureID).Rows()
	if err != nil {
		return nil, classify(err)
	}
	defer rows.Close()
	latest := map[string]time.Time{}
	for rows.Next() {
		var (
			threadID string
			at       flexTime
		)
		if err := rows.Scan(&threadID, &at); err != nil {
			return nil, err
		}
		if at.Valid {
			latest[threadID] = at.Time.UTC()
		}
	}
	return latest, rows.Err()
}
