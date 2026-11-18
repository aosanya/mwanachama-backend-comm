package mwanachamacomm

import (
	"context"
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type Raisable func(event models.NotificationEvent) bool

func (m *CommManager) UseRaisable(r Raisable) { m.raisable = r }

func (m *CommManager) RaiseNotificationFor(ctx context.Context, callerID, recipientID string, category models.NotificationCategory, event models.NotificationEvent, subjectKind models.NotificationSubjectKind, subjectID string, names map[string]string) (models.Notification, error) {
	if m.raisable == nil || !m.raisable(event) {
		return models.Notification{}, fmt.Errorf("%w: %q", models.ErrNotificationNotRaisable, event)
	}
	if strings.TrimSpace(recipientID) == "" {
		return models.Notification{}, fmt.Errorf("%w: a notification names the person it is for", models.ErrNotificationInvalid)
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
