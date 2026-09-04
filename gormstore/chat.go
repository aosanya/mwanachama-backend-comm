package gormstore

import (
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// ChatThreadRow is the GORM row for a [models.ChatThread].
type ChatThreadRow struct {
	ID        string `gorm:"primaryKey"`
	ChapterID string `gorm:"uniqueIndex:comm_chat_thread_chapter_tag_path;index"`
	TagPath   string `gorm:"uniqueIndex:comm_chat_thread_chapter_tag_path"`
	CreatedAt time.Time
}

func (r *ChatThreadRow) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		id, err := mintID(tx, "cthread", "comm_chat_thread_seq")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

// ChatThreadToRow converts a domain ChatThread to its row shape.
func ChatThreadToRow(t models.ChatThread) ChatThreadRow {
	return ChatThreadRow{ID: t.ID, ChapterID: t.ChapterID, TagPath: t.TagPath, CreatedAt: t.CreatedAt}
}

// ChatThreadFromRow converts a row back to the domain ChatThread.
func ChatThreadFromRow(r ChatThreadRow) models.ChatThread {
	return models.ChatThread{ID: r.ID, ChapterID: r.ChapterID, TagPath: r.TagPath, CreatedAt: r.CreatedAt}
}

// ChatMessageRow is the GORM row for a [models.ChatMessage]. ThreadID is
// nullable — a room-level post carries no thread.
type ChatMessageRow struct {
	ID        string  `gorm:"primaryKey"`
	ChapterID string  `gorm:"index:comm_chat_message_chapter_idx,priority:1"`
	ThreadID  *string `gorm:"index:comm_chat_message_thread_idx,priority:1"`
	AuthorID  string
	Body      string
	CreatedAt time.Time `gorm:"index:comm_chat_message_chapter_idx,priority:2;index:comm_chat_message_thread_idx,priority:2"`
}

func (r *ChatMessageRow) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		id, err := mintID(tx, "msg", "comm_chat_message_seq")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

// ChatMessageToRow converts a domain ChatMessage to its row shape.
func ChatMessageToRow(m models.ChatMessage) ChatMessageRow {
	return ChatMessageRow{
		ID:        m.ID,
		ChapterID: m.ChapterID,
		ThreadID:  StringToNullable(m.ThreadID),
		AuthorID:  m.AuthorID,
		Body:      m.Body,
		CreatedAt: m.CreatedAt,
	}
}

// ChatMessageFromRow converts a row back to the domain ChatMessage.
func ChatMessageFromRow(r ChatMessageRow) models.ChatMessage {
	return models.ChatMessage{
		ID:        r.ID,
		ChapterID: r.ChapterID,
		ThreadID:  NullableToString(r.ThreadID),
		AuthorID:  r.AuthorID,
		Body:      r.Body,
		CreatedAt: r.CreatedAt,
	}
}

// StringToNullable maps a domain "" (unset) to a nil *string, the nullable
// column spelling of "no value" — mirrors actor's gormstore.StringToNullable.
func StringToNullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// NullableToString is StringToNullable's inverse.
func NullableToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
