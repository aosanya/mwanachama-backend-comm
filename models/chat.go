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

var ErrChatInvalid = errors.New("chat: invalid")

var ErrChatNotDeclinable = errors.New("chat: only a thread somebody asked for, still with the leads, can be declined")

type ChatThread struct {
	ID          string       `json:"id"`
	StructureID string       `json:"structure_id"`
	TagPath     string       `json:"tag_path"`
	Label       string       `json:"label"`
	Audience    ChatAudience `json:"audience"`
	RequestedBy string       `json:"requested_by,omitempty"`
	DeclinedAt  *time.Time   `json:"declined_at,omitempty"`
	DeclinedBy  string       `json:"declined_by,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

type ChatAudience string

const (
	ChatAudienceLeads ChatAudience = "leads"
	ChatAudienceGroup ChatAudience = "group"
)

func (t ChatThread) Declined() bool { return t.DeclinedAt != nil }

func (t ChatThread) Declinable() bool {
	return !t.Declined() && t.Audience == ChatAudienceLeads && t.RequestedBy != ""
}

func (t ChatThread) ReadableBy(actorID string, leads bool) bool {
	switch {
	case t.Declined():
		return false
	case t.Audience != ChatAudienceLeads:
		return true
	case leads:
		return true
	default:
		return actorID != "" && actorID == t.RequestedBy
	}
}

type ChatMessage struct {
	ID          string    `json:"id"`
	StructureID string    `json:"structure_id"`
	ThreadID    string    `json:"thread_id,omitempty"`
	AuthorID    string    `json:"author_id"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type ChatReadMarker struct {
	ID          string    `json:"id"`
	StructureID string    `json:"structure_id"`
	ThreadID    string    `json:"thread_id"`
	ReaderID    string    `json:"reader_id"`
	ReadAt      time.Time `json:"read_at"`
	MarkedAt    time.Time `json:"marked_at"`
}

type ChatThreadView struct {
	ChatThread
	Unread int `json:"unread"`
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
