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
	row, err := s.store.Row(roleChatReadMarker, marker)
	if err != nil {
		return models.ChatReadMarker{}, err
	}
	table := s.store.Table(roleChatReadMarker)
	err = s.store.Query(ctx, roleChatReadMarker).
		Clauses(clause.OnConflict{
			Columns: conflictOn(s.store, "thread_id", "reader_id"),
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

const UnreadCap = 100

func (s *ChatStore) UnreadCounts(ctx context.Context, structureID, readerID string) (map[string]int, error) {
	markerScope, markerArgs := runClause(s.store, "r")
	messageScope, messageArgs := runClause(s.store, "m")
	threadScope, threadArgs := runClause(s.store, "t")
	query := `
	SELECT t.id, (
	  SELECT count(*) FROM (
	    SELECT 1
	      FROM ` + s.store.Table(roleChatMessage) + ` m
	      LEFT JOIN ` + s.store.Table(roleChatReadMarker) + ` r
	        ON r.thread_id = m.thread_id AND r.reader_id = ?` + markerScope + `
	     WHERE m.thread_id = t.id AND m.author_id <> ?` + messageScope + `
	       AND (r.read_at IS NULL OR m.created_at > r.read_at)
	     LIMIT ?
	  ) counted
	)
	  FROM ` + s.store.Table(roleChatThread) + ` t
	 WHERE t.structure_id = ?` + threadScope
	args := []any{readerID}
	args = append(args, markerArgs...)
	args = append(args, readerID)
	args = append(args, messageArgs...)
	args = append(args, UnreadCap, structureID)
	args = append(args, threadArgs...)
	rows, err := s.store.Query(ctx, roleChatThread).Raw(query, args...).Rows()
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
		if n > 0 {
			counts[threadID] = n
		}
	}
	return counts, rows.Err()
}

func (s *ChatStore) LatestPosts(ctx context.Context, structureID string) (map[string]time.Time, error) {
	messageScope, messageArgs := runClause(s.store, "m")
	threadScope, threadArgs := runClause(s.store, "t")
	query := `
	SELECT t.id, (
	  SELECT max(m.created_at) FROM ` + s.store.Table(roleChatMessage) + ` m WHERE m.thread_id = t.id` + messageScope + `
	)
	  FROM ` + s.store.Table(roleChatThread) + ` t
	 WHERE t.structure_id = ?` + threadScope
	args := append(messageArgs, structureID)
	args = append(args, threadArgs...)
	rows, err := s.store.Query(ctx, roleChatThread).Raw(query, args...).Rows()
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
