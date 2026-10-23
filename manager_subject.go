package mwanachamacomm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type Standing func(ctx context.Context, actorID, subjectKind, subjectID string) (models.SubjectStanding, error)

func (m *CommManager) UseStanding(s Standing) { m.standing = s }

type Answerers func(ctx context.Context, subjectKind, subjectID string) ([]string, error)

func (m *CommManager) UseAnswerers(a Answerers) { m.answerers = a }

func (m *CommManager) standingOn(ctx context.Context, callerID, kind, id string) (models.SubjectStanding, error) {
	if m.standing == nil || callerID == "" || kind == "" || id == "" {
		return models.StandingNone, models.ErrSubjectNotFound
	}
	st, err := m.standing(ctx, callerID, kind, id)
	if err != nil {
		return models.StandingNone, err
	}
	if st != models.StandingAudience && st != models.StandingAnswerer {
		return models.StandingNone, models.ErrSubjectNotFound
	}
	return st, nil
}

func (m *CommManager) readableThread(ctx context.Context, threadID, callerID string) (models.SubjectThread, models.SubjectStanding, error) {
	t, err := m.subjects.Thread(ctx, threadID)
	if err != nil {
		return models.SubjectThread{}, models.StandingNone, err
	}
	st, err := m.standingOn(ctx, callerID, t.SubjectKind, t.SubjectID)
	if err != nil {
		return models.SubjectThread{}, models.StandingNone, err
	}
	if !t.ReadableBy(callerID, st) {
		return models.SubjectThread{}, models.StandingNone, models.ErrSubjectNotFound
	}
	return t, st, nil
}

func (m *CommManager) OpenSubjectThreadFor(ctx context.Context, kind, id, callerID string, audience models.SubjectAudience) (models.SubjectThreadView, error) {
	st, err := m.standingOn(ctx, callerID, kind, id)
	if err != nil {
		return models.SubjectThreadView{}, err
	}
	owner := ""
	switch {
	case audience == models.AudiencePrivate && st == models.StandingAudience:
		owner = callerID
	case audience == models.AudiencePrivate:
		return models.SubjectThreadView{}, fmt.Errorf("%w: a private thread is opened by the one it is with, not by an answerer", models.ErrSubjectInvalid)
	case audience != models.AudienceGroup:
		return models.SubjectThreadView{}, fmt.Errorf("%w: audience must be private or group", models.ErrSubjectInvalid)
	}
	t, err := m.subjects.Open(ctx, kind, id, audience, owner)
	if err != nil {
		return models.SubjectThreadView{}, err
	}
	views, err := m.subjects.Views(ctx, []models.SubjectThread{t})
	if err != nil {
		return models.SubjectThreadView{}, err
	}
	return views[0], nil
}

func (m *CommManager) ListSubjectThreadsFor(ctx context.Context, kind, id, callerID string) ([]models.SubjectThreadView, error) {
	st, err := m.standingOn(ctx, callerID, kind, id)
	if err != nil {
		return nil, err
	}
	all, err := m.subjects.Threads(ctx, kind, id)
	if err != nil {
		return nil, err
	}
	readable := make([]models.SubjectThread, 0, len(all))
	for _, t := range all {
		if t.ReadableBy(callerID, st) {
			readable = append(readable, t)
		}
	}
	return m.subjects.Views(ctx, readable)
}

func (m *CommManager) ListSubjectMessagesFor(ctx context.Context, threadID, callerID string) ([]models.SubjectThreadMessage, error) {
	_, st, err := m.readableThread(ctx, threadID, callerID)
	if err != nil {
		return nil, err
	}
	msgs, err := m.subjects.Messages(ctx, threadID)
	if err != nil {
		return nil, err
	}
	for i := range msgs {
		msgs[i] = msgs[i].For(st)
	}
	return msgs, nil
}

func (m *CommManager) PostSubjectMessageFor(ctx context.Context, threadID, callerID, body, regardingID string) (models.SubjectThreadMessage, error) {
	t, st, err := m.readableThread(ctx, threadID, callerID)
	if err != nil {
		return models.SubjectThreadMessage{}, err
	}
	if strings.TrimSpace(body) == "" {
		return models.SubjectThreadMessage{}, fmt.Errorf("%w: body is required", models.ErrSubjectInvalid)
	}
	answerers, err := m.answerersToTell(ctx, t, callerID, st)
	if err != nil {
		return models.SubjectThreadMessage{}, err
	}
	var (
		posted models.SubjectThreadMessage
		raised []models.Notification
	)
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		subjects, err := NewSubjectStore(tx, m.spec, m.subjects.clock)
		if err != nil {
			return err
		}
		notices, err := NewNotificationStore(tx, m.spec, m.subjects.clock)
		if err != nil {
			return err
		}
		recipients, err := replyRecipients(ctx, subjects, t, callerID, st, answerers)
		if err != nil {
			return err
		}
		posted, err = subjects.Post(ctx, models.SubjectThreadMessage{
			ThreadID: t.ID, AuthorID: callerID, ByAnswerer: st == models.StandingAnswerer,
			RegardingID: regardingID, Body: body,
		})
		if err != nil {
			return err
		}
		recipients, err = m.recipientsFor(ctx, replyEvent(t), RaiseRoom{ThreadID: t.ID, SubjectKind: t.SubjectKind, SubjectID: t.SubjectID}, recipients)
		if err != nil {
			return err
		}
		raised, err = raiseReplies(ctx, notices, t, recipients)
		return err
	})
	if err != nil {
		return models.SubjectThreadMessage{}, err
	}
	if m.NotificationStore.onRaised != nil {
		for _, n := range raised {
			m.NotificationStore.onRaised(ctx, n)
		}
	}
	return posted.For(st), nil
}

func replyRecipients(ctx context.Context, subjects *SubjectStore, t models.SubjectThread, posterID string, poster models.SubjectStanding, answerers []string) ([]string, error) {
	if t.Audience == models.AudiencePrivate {
		if poster == models.StandingAnswerer && t.OwnerID != posterID {
			return []string{t.OwnerID}, nil
		}
		return answerers, nil
	}
	authors, err := subjects.AudienceAuthors(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(authors))
	for _, a := range authors {
		if a != posterID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *CommManager) answerersToTell(ctx context.Context, t models.SubjectThread, posterID string, poster models.SubjectStanding) ([]string, error) {
	if m.answerers == nil || t.Audience != models.AudiencePrivate || poster != models.StandingAudience {
		return nil, nil
	}
	named, err := m.answerers(ctx, t.SubjectKind, t.SubjectID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{posterID: true, "": true}
	out := make([]string, 0, len(named))
	for _, a := range named {
		if seen[a] {
			continue
		}
		seen[a] = true
		st, err := m.standingOn(ctx, a, t.SubjectKind, t.SubjectID)
		if errors.Is(err, models.ErrSubjectNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if st == models.StandingAnswerer {
			out = append(out, a)
		}
	}
	return out, nil
}

func replyEvent(t models.SubjectThread) models.NotificationEvent {
	if t.SubjectKind == string(models.SubjectRecord) && t.Audience == models.AudiencePrivate {
		return models.EventRecordMessage
	}
	return models.EventThreadReply
}

func raiseReplies(ctx context.Context, notices *NotificationStore, t models.SubjectThread, recipients []string) ([]models.Notification, error) {
	threadID := t.ID
	event := replyEvent(t)
	raised := make([]models.Notification, 0, len(recipients))
	for _, actorID := range recipients {
		n, err := notices.Raise(ctx, models.Notification{
			ActorID:     actorID,
			Category:    models.CategoryChat,
			Event:       event,
			SubjectKind: models.SubjectSubjectThread,
			SubjectID:   threadID,
		})
		switch {
		case errors.Is(err, models.ErrNotificationMuted):
		case err != nil:
			return nil, err
		default:
			raised = append(raised, n)
		}
	}
	return raised, nil
}
