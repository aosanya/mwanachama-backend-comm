package models

import (
	"errors"
	"time"
)

var ErrSubjectNotFound = errors.New("subject thread: not found")

var ErrSubjectInvalid = errors.New("subject thread: invalid")

type SubjectAudience string

const (
	AudiencePrivate SubjectAudience = "private"
	AudienceGroup   SubjectAudience = "group"
)

func (a SubjectAudience) Valid() bool { return a == AudiencePrivate || a == AudienceGroup }

type SubjectStanding int

const (
	StandingNone SubjectStanding = iota
	StandingAudience
	StandingAnswerer
)

type SubjectThread struct {
	ID          string          `json:"id"`
	SubjectKind string          `json:"subject_kind"`
	SubjectID   string          `json:"subject_id"`
	Audience    SubjectAudience `json:"audience"`
	OwnerID     string          `json:"owner_id,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

type SubjectThreadView struct {
	SubjectThread
	MessageCount  int        `json:"message_count"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
}

type SubjectThreadMessage struct {
	ID          string    `json:"id"`
	ThreadID    string    `json:"thread_id"`
	AuthorID    string    `json:"author_id,omitempty"`
	ByAnswerer  bool      `json:"by_answerer"`
	RegardingID string    `json:"regarding_id,omitempty"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

func (m SubjectThreadMessage) For(reader SubjectStanding) SubjectThreadMessage {
	if m.ByAnswerer && reader != StandingAnswerer {
		m.AuthorID = ""
	}
	return m
}

func (t SubjectThread) ReadableBy(actorID string, standing SubjectStanding) bool {
	switch {
	case actorID == "" || standing == StandingNone:
		return false
	case standing == StandingAnswerer:
		return true
	case t.Audience == AudiencePrivate:
		return t.OwnerID == actorID
	default:
		return t.Audience == AudienceGroup
	}
}
