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

type ChatThread struct {
	ID          string    `json:"id"`
	StructureID string    `json:"structure_id"`
	TagPath     string    `json:"tag_path"`
	CreatedAt   time.Time `json:"created_at"`
}

type ChatMessage struct {
	ID          string    `json:"id"`
	StructureID string    `json:"structure_id"`
	ThreadID    string    `json:"thread_id,omitempty"`
	AuthorID    string    `json:"author_id"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChatRepository is the persistence boundary for the chat domain.
type ChatRepository interface {
	MintThread(ctx context.Context, structureID, tagPath string) (ChatThread, error)
	// ResolveThread returns the thread for a tag_path, if one exists.
	ResolveThread(ctx context.Context, structureID, tagPath string) (ChatThread, error)
	ListThreads(ctx context.Context, structureID string) ([]ChatThread, error)

	Post(ctx context.Context, m ChatMessage) (ChatMessage, error)
	ListMessages(ctx context.Context, structureID, threadID string) ([]ChatMessage, error)
	GetMessage(ctx context.Context, id string) (ChatMessage, error)

	// ChatActivityReader adds the one read that is scoped to no room — see
	// chat_activity.go for what it deliberately does not return, and why the
	// restraint is the design rather than a first cut.
	ChatActivityReader
}
