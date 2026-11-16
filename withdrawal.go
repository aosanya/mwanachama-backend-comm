package mwanachamacomm

import (
	"context"
	"fmt"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (s *PushStore) ReleaseActor(ctx context.Context, actorID string) error {
	if actorID == "" {
		return fmt.Errorf("%w: a subscription belongs to somebody", ErrPushSubscriptionInvalid)
	}
	return classify(deleteWhere(ctx, s.store, rolePushSubscription, "actor_id = ?", actorID))
}

func (m *CommManager) HideActorMessages(ctx context.Context, actorID string) error {
	if actorID == "" {
		return fmt.Errorf("%w: a message belongs to somebody", models.ErrChatInvalid)
	}
	at := stamp(m.ChatStore.clock())
	chat := m.ChatStore.store.Query(ctx, roleChatMessage).Where("author_id = ? AND withdrawn_at = ''", actorID)
	if err := chat.Updates(map[string]any{"withdrawn_at": at}).Error; err != nil {
		return classify(err)
	}
	dm := m.DMStore.store.Query(ctx, roleDMMessage).Where("sender_id = ? AND withdrawn_at = ''", actorID)
	return classify(dm.Updates(map[string]any{"withdrawn_at": at}).Error)
}
