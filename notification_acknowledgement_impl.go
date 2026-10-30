package mwanachamacomm

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (s *NotificationStore) AcknowledgeNotification(ctx context.Context, actorID, notificationID string) (models.NotificationAcknowledgement, error) {
	if actorID == "" || notificationID == "" {
		return models.NotificationAcknowledgement{}, models.ErrNotificationNotFound
	}
	var own models.Notification
	q := s.store.Query(ctx, roleNotification).Where("id = ? AND actor_id = ?", notificationID, actorID)
	if err := s.store.Take(q, roleNotification, &own, models.ErrNotificationNotFound); err != nil {
		return models.NotificationAcknowledgement{}, classify(err)
	}
	if held, found, err := s.acknowledgementOf(ctx, notificationID); err != nil || found {
		return held, err
	}
	ack := models.NotificationAcknowledgement{
		ID: newID(), NotificationID: notificationID, ActorID: actorID, AcknowledgedAt: s.clock(),
	}
	if err := Check(s.store.Spec(), roleNotificationAck, ack); err != nil {
		return models.NotificationAcknowledgement{}, err
	}
	err := classifyNotificationWrite(s.store.Insert(ctx, roleNotificationAck, ack))
	if errors.Is(err, models.ErrNotificationCapSpent) {
		held, _, readErr := s.acknowledgementOf(ctx, notificationID)
		return held, readErr
	}
	if err != nil {
		return models.NotificationAcknowledgement{}, err
	}
	return ack, nil
}

func (s *NotificationStore) acknowledgementOf(ctx context.Context, notificationID string) (models.NotificationAcknowledgement, bool, error) {
	q := s.store.Query(ctx, roleNotificationAck).Where("notification_id = ?", notificationID).Limit(1)
	out, err := specstore.List[models.NotificationAcknowledgement](s.store, q, roleNotificationAck)
	if err != nil {
		return models.NotificationAcknowledgement{}, false, classify(err)
	}
	if len(out) == 0 {
		return models.NotificationAcknowledgement{}, false, nil
	}
	return out[0], true, nil
}

func (s *NotificationStore) NotificationReceipts(ctx context.Context, subjectKind models.NotificationSubjectKind, subjectID string, event models.NotificationEvent, after string, limit int) ([]models.NotificationReceipt, error) {
	if strings.TrimSpace(string(subjectKind)) == "" || strings.TrimSpace(subjectID) == "" {
		return nil, models.ErrNotificationInvalid
	}
	if limit <= 0 {
		limit = models.NotificationReceiptsDefaultPage
	}
	if limit > models.NotificationReceiptsMaxPage {
		limit = models.NotificationReceiptsMaxPage
	}
	q := s.store.Query(ctx, roleNotification).
		Where("subject_kind = ? AND subject_id = ?", string(subjectKind), subjectID)
	if event != "" {
		q = q.Where("event = ?", string(event))
	}
	if after != "" {
		q = q.Where("id > ?", after)
	}
	sent, err := specstore.List[models.Notification](s.store, q.Order("id").Limit(limit), roleNotification)
	if err != nil {
		return nil, classify(err)
	}
	acknowledgedAt, err := s.acknowledgementTimes(ctx, sent)
	if err != nil {
		return nil, err
	}
	out := make([]models.NotificationReceipt, len(sent))
	for i, n := range sent {
		out[i] = models.NotificationReceipt{
			NotificationID: n.ID, ActorID: n.ActorID, Event: n.Event,
			CreatedAt: n.CreatedAt, ReadAt: n.ReadAt, AcknowledgedAt: acknowledgedAt[n.ID],
		}
	}
	return out, nil
}

func (s *NotificationStore) acknowledgementTimes(ctx context.Context, sent []models.Notification) (map[string]*time.Time, error) {
	times := map[string]*time.Time{}
	if len(sent) == 0 {
		return times, nil
	}
	ids := make([]string, len(sent))
	for i, n := range sent {
		ids[i] = n.ID
	}
	q := s.store.Query(ctx, roleNotificationAck).Where("notification_id IN ?", ids)
	acks, err := specstore.List[models.NotificationAcknowledgement](s.store, q, roleNotificationAck)
	if err != nil {
		return nil, classify(err)
	}
	for _, a := range acks {
		at := a.AcknowledgedAt
		times[a.NotificationID] = &at
	}
	return times, nil
}
