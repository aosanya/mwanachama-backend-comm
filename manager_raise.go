package mwanachamacomm

import (
	"context"
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type Raisable func(event models.NotificationEvent) bool

type RecipientRule func(ctx context.Context, event models.NotificationEvent, recipientID string) error

type RecipientRoster func(ctx context.Context, event models.NotificationEvent) ([]string, error)

func (m *CommManager) UseRecipientRoster(r RecipientRoster) { m.recipientRoster = r }

func (m *CommManager) ListRaiseRecipients(ctx context.Context, event string) ([]models.NotificationRecipient, error) {
	if m.recipientRoster == nil {
		return nil, fmt.Errorf("%w: %q", models.ErrNotificationNotRaisable, event)
	}
	ids, err := m.recipientRoster(ctx, models.NotificationEvent(event))
	if err != nil {
		return nil, err
	}
	out := make([]models.NotificationRecipient, 0, len(ids))
	for _, id := range ids {
		out = append(out, models.NotificationRecipient{ActorID: id})
	}
	return out, nil
}

func (m *CommManager) UseRaisable(r Raisable) { m.raisable = r }

func (m *CommManager) UseRecipientRule(r RecipientRule) { m.recipientRule = r }

func (m *CommManager) RaiseNotificationFor(ctx context.Context, callerID, recipientID string, category models.NotificationCategory, event models.NotificationEvent, subjectKind models.NotificationSubjectKind, subjectID string, names map[string]string) (models.Notification, error) {
	if m.raisable == nil || !m.raisable(event) {
		return models.Notification{}, fmt.Errorf("%w: %q", models.ErrNotificationNotRaisable, event)
	}
	if strings.TrimSpace(recipientID) == "" {
		return models.Notification{}, fmt.Errorf("%w: a notification names the person it is for", models.ErrNotificationInvalid)
	}
	if m.recipientRule != nil {
		if err := m.recipientRule(ctx, event, recipientID); err != nil {
			return models.Notification{}, fmt.Errorf("%w: %s", models.ErrNotificationNotRaisable, err.Error())
		}
	}
	if strings.TrimSpace(subjectID) == "" {
		return models.Notification{}, fmt.Errorf("%w: a notification names the record it is about", models.ErrNotificationInvalid)
	}
	return m.NotificationStore.Raise(ctx, models.Notification{
		ActorID:     recipientID,
		Category:    category,
		Event:       event,
		SubjectKind: subjectKind,
		SubjectID:   subjectID,
		Names:       names,
	})
}
