package models

import (
	"context"
	"time"
)

// ChatActivityQuery is one filtered page of the chat activity summary.
type ChatActivityQuery struct {
	StructureID string
	Limit       *int
	Offset      int
}

type ChatActivityRow struct {
	StructureID string `json:"structure_id"`
	// Threads is how many conversations have been opened in this room.
	Threads int `json:"threads"`
	// Messages is how many posts were made — including any since removed, for
	// the reason in the package comment above.
	Messages int `json:"messages"`
	// LastPostAt is when the room last heard anything, and is the figure the
	// "gone quiet" reading rests on. Nil for a room holding threads and no
	// posts, which is a real state and not a missing value.
	LastPostAt *time.Time `json:"last_post_at,omitempty"`
}

type ChatActivityPage struct {
	Rows       []ChatActivityRow `json:"rows"`
	Total      int               `json:"total"`
	Threads    int               `json:"threads"`
	Messages   int               `json:"messages"`
	LastPostAt *time.Time        `json:"last_post_at,omitempty"`
}

// ChatActivityReader is the summary half of ChatRepository, its own interface
// so a caller that only summarises cannot reach a room's contents.
type ChatActivityReader interface {
	// Activity returns one page of the chat activity summary plus the totals
	// over the whole filtered set. Authorisation is the HTTP layer's, as it is
	// for every other read in this domain.
	Activity(ctx context.Context, q ChatActivityQuery) (ChatActivityPage, error)
}
