package models

import (
	"context"
	"time"
)

type ChatActivityQuery struct {
	StructureID string `query:"structure_id"`
	Limit       *int   `query:"limit,min=1"`
	Offset      int    `query:"offset,min=0"`
}

type ChatActivityRow struct {
	StructureID string `json:"structure_id"`

	Threads int `json:"threads"`

	Messages int `json:"messages"`

	LastPostAt *time.Time `json:"last_post_at,omitempty"`
}

type ChatActivityPage struct {
	Rows       []ChatActivityRow `json:"rows"`
	Total      int               `json:"total"`
	Threads    int               `json:"threads"`
	Messages   int               `json:"messages"`
	LastPostAt *time.Time        `json:"last_post_at,omitempty"`
}

type ChatActivityReader interface {
	Activity(ctx context.Context, q ChatActivityQuery) (ChatActivityPage, error)
}
