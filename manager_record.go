package mwanachamacomm

import (
	"context"
	"fmt"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (m *CommManager) EndRecordAccess(ctx context.Context, recordID, actorID string) (models.Notification, error) {
	if recordID == "" || actorID == "" {
		return models.Notification{}, fmt.Errorf("%w: a record and the actor whose access ended are both required", models.ErrSubjectInvalid)
	}
	return m.NotificationStore.Raise(ctx, models.Notification{
		ActorID:     actorID,
		Category:    models.CategoryChat,
		Event:       models.EventRecordAccessEnded,
		SubjectKind: models.SubjectRecord,
		SubjectID:   recordID,
	})
}
