// Package models holds mwanachamacomm's domain types and the repository
// interfaces they're read and written through — Chat*, DM*, and
// moderation's Report/Removal/Dismissal/Dispute. Callers use models.ChatThread
// etc. directly; the root package holds the GORM-backed implementations
// (see gormstore/ for row shapes and doc.go for the split rationale).
package models

import (
	"context"
	"errors"
	"time"
)

// ErrChatNotFound is returned when a chat thread or message id has no record.
var ErrChatNotFound = errors.New("chat: not found")

// ChatThread is a tag_path conversation within a chapter room.
type ChatThread struct {
	ID        string    `json:"id"`
	ChapterID string    `json:"chapter_id"`
	TagPath   string    `json:"tag_path"`
	CreatedAt time.Time `json:"created_at"`
}

// ChatMessage is a post in a chapter room, optionally within a thread.
type ChatMessage struct {
	ID        string    `json:"id"`
	ChapterID string    `json:"chapter_id"`
	ThreadID  string    `json:"thread_id,omitempty"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// ChatRepository is the persistence boundary for the chat domain.
type ChatRepository interface {
	// MintThread creates (or returns the existing) thread for a tag_path in a
	// chapter room.
	MintThread(ctx context.Context, chapterID, tagPath string) (ChatThread, error)
	// ResolveThread returns the thread for a tag_path, if one exists.
	ResolveThread(ctx context.Context, chapterID, tagPath string) (ChatThread, error)
	ListThreads(ctx context.Context, chapterID string) ([]ChatThread, error)

	Post(ctx context.Context, m ChatMessage) (ChatMessage, error)
	// ListMessages returns messages in a chapter room; when threadID is set,
	// only that thread's messages.
	ListMessages(ctx context.Context, chapterID, threadID string) ([]ChatMessage, error)
	// GetMessage returns one message by id. ErrChatNotFound if unknown. Added
	// for DEV-1115 (moderation): a report freezes its excerpt off the message
	// body at that instant, and a removal resolves the message's chapter and
	// author — both need to read one message by id rather than list a room.
	GetMessage(ctx context.Context, id string) (ChatMessage, error)

	// ChatActivityReader adds the one read that is scoped to no room — see
	// chat_activity.go for what it deliberately does not return, and why the
	// restraint is the design rather than a first cut.
	ChatActivityReader
}
