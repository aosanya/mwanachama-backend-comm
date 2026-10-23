package models

import (
	"errors"
	"time"
)

var (
	ErrStandingInvalid  = errors.New("standing message: invalid")
	ErrStandingNotFound = errors.New("standing message: not found")
	ErrStandingStopped  = errors.New("standing message: already stopped")
)

type StandingMessage struct {
	ID          string     `json:"id"`
	StructureID string     `json:"structure_id"`
	ThreadID    string     `json:"thread_id,omitempty"`
	AuthorID    string     `json:"author_id"`
	Body        string     `json:"body"`
	Rule        string     `json:"rule,omitempty"`
	Dates       []string   `json:"dates,omitempty"`
	Zone        string     `json:"zone"`
	StartsAt    time.Time  `json:"start"`
	EndsAt      *time.Time `json:"end,omitempty"`
	LastSentAt  *time.Time `json:"last_sent_at,omitempty"`
	StoppedAt   *time.Time `json:"stopped_at,omitempty"`
	StoppedBy   string     `json:"stopped_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (m StandingMessage) Stopped() bool { return m.StoppedAt != nil }
