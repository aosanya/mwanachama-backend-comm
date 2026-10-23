package mwanachamacomm

import (
	"context"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (m *CommManager) UseWebPush(sender PushSender, onFailure func(error)) {
	if sender == nil {
		m.push = nil
		m.NotificationStore.onRaised = nil
		return
	}
	d := newPushDelivery(m.PushStore, sender)
	if onFailure != nil {
		d.failed = onFailure
	}
	m.push = d
	m.NotificationStore.onRaised = d.raised
}

func (m *CommManager) DrainPushes() {
	if m.push != nil {
		m.push.drain()
	}
}

func (m *CommManager) SubscribePushFor(ctx context.Context, callerID, deviceID, endpoint string, keys models.PushKeys) (models.PushSubscription, error) {
	return m.PushStore.SubscribePush(ctx, models.PushSubscription{
		ActorID:    callerID,
		DeviceID:   deviceID,
		Endpoint:   endpoint,
		PublicKey:  keys.P256dh,
		AuthSecret: keys.Auth,
	})
}

func (m *CommManager) UnsubscribePushFor(ctx context.Context, callerID, endpoint string) error {
	return m.PushStore.UnsubscribePush(ctx, callerID, endpoint)
}

func (m *CommManager) PushDeliveriesOf(ctx context.Context, notificationID string) ([]models.PushDelivery, error) {
	return m.PushStore.ListPushDeliveries(ctx, notificationID)
}
