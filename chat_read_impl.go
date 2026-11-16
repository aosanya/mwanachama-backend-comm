package mwanachamacomm

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aosanya/mwanachama-backend-comm/models"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
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
	     WHERE m.thread_id = t.id AND m.author_id <> ? AND m.withdrawn_at = ''` + messageScope + `
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

func (s *ChatStore) LatestPosts(ctx context.Context, structureID string) (map[string]models.ChatMessage, error) {
	messageScope, messageArgs := runClause(s.store, "m")
	threadScope, threadArgs := runClause(s.store, "t")
	query := `
	SELECT t.id, (
	  SELECT m.id FROM ` + s.store.Table(roleChatMessage) + ` m WHERE m.thread_id = t.id AND m.withdrawn_at = ''` + messageScope + `
	   ORDER BY m.created_at DESC, m.id DESC LIMIT 1
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
	threadOf := map[string]string{}
	messageIDs := []string{}
	for rows.Next() {
		var (
			threadID  string
			messageID sql.NullString
		)
		if err := rows.Scan(&threadID, &messageID); err != nil {
			return nil, err
		}
		if messageID.Valid {
			threadOf[messageID.String] = threadID
			messageIDs = append(messageIDs, messageID.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	latest := map[string]models.ChatMessage{}
	if len(messageIDs) == 0 {
		return latest, nil
	}
	posts, err := specstore.List[models.ChatMessage](s.store, s.store.Query(ctx, roleChatMessage).Where("id IN ?", messageIDs), roleChatMessage)
	if err != nil {
		return nil, err
	}
	for _, post := range posts {
		post.CreatedAt = post.CreatedAt.UTC()
		latest[threadOf[post.ID]] = post
	}
	return latest, nil
}
