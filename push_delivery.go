package mwanachamacomm

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

const (
	pushDeliveryTimeout = 30 * time.Second
	pushConcurrency     = 16
)

type pushPayload struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Event       string `json:"event"`
	SubjectKind string `json:"subject_kind,omitempty"`
	SubjectID   string `json:"subject_id,omitempty"`
	StructureID string `json:"structure_id,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func payloadOf(n models.Notification) ([]byte, error) {
	return json.Marshal(pushPayload{
		ID:          n.ID,
		Category:    string(n.Category),
		Event:       string(n.Event),
		SubjectKind: string(n.SubjectKind),
		SubjectID:   n.SubjectID,
		StructureID: n.StructureID,
		CreatedAt:   n.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
}

type pushDelivery struct {
	subscriptions *PushStore
	sender        PushSender
	slots         chan struct{}
	inFlight      sync.WaitGroup
	failed        func(error)
}

func newPushDelivery(subs *PushStore, sender PushSender) *pushDelivery {
	return &pushDelivery{
		subscriptions: subs,
		sender:        sender,
		slots:         make(chan struct{}, pushConcurrency),
		failed:        func(error) {},
	}
}

func (d *pushDelivery) raised(ctx context.Context, n models.Notification) {
	detached := context.WithoutCancel(ctx)
	d.inFlight.Add(1)
	go func() {
		defer d.inFlight.Done()
		d.slots <- struct{}{}
		defer func() { <-d.slots }()
		bounded, cancel := context.WithTimeout(detached, pushDeliveryTimeout)
		defer cancel()
		d.deliver(bounded, n)
	}()
}

func (d *pushDelivery) deliver(ctx context.Context, n models.Notification) {
	subs, err := d.subscriptions.ListPushSubscriptions(ctx, n.ActorID)
	if err != nil {
		d.failed(err)
		return
	}
	if len(subs) == 0 {
		return
	}
	payload, err := payloadOf(n)
	if err != nil {
		d.failed(err)
		return
	}
	for _, sub := range subs {
		err := d.sender.Send(ctx, sub, payload)
		state := models.PushDeliverySent
		switch {
		case errors.Is(err, models.ErrPushSubscriptionGone):
			state = models.PushDeliveryGone
			if err := d.subscriptions.DropPushSubscription(ctx, sub.ID); err != nil {
				d.failed(err)
			}
		case err != nil:
			state = models.PushDeliveryFailed
			d.failed(err)
		}
		d.record(ctx, n, sub, state)
	}
}

func (d *pushDelivery) record(ctx context.Context, n models.Notification, sub models.PushSubscription, state models.PushDeliveryState) {
	_, err := d.subscriptions.RecordPushDelivery(ctx, models.PushDelivery{
		NotificationID: n.ID,
		ActorID:        n.ActorID,
		SubscriptionID: sub.ID,
		State:          state,
	})
	if err != nil {
		d.failed(err)
	}
}

func (d *pushDelivery) drain() { d.inFlight.Wait() }
