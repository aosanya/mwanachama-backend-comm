package mwanachamacomm

import (
	"context"
	"fmt"

	"github.com/aosanya/mwanachama-backend-comm/models"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
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

type HiddenMessages struct {
	Chat   []models.ChatMessage `json:"chat"`
	Direct []models.DMMessage   `json:"direct"`
}

func (m *CommManager) RestoreActorMessages(ctx context.Context, actorID string) error {
	if actorID == "" {
		return fmt.Errorf("%w: a message belongs to somebody", models.ErrChatInvalid)
	}
	shown := map[string]any{"withdrawn_at": ""}
	chat := m.ChatStore.store.Query(ctx, roleChatMessage).Where("author_id = ? AND withdrawn_at <> ''", actorID)
	if err := chat.Updates(shown).Error; err != nil {
		return classify(err)
	}
	dm := m.DMStore.store.Query(ctx, roleDMMessage).Where("sender_id = ? AND withdrawn_at <> ''", actorID)
	return classify(dm.Updates(shown).Error)
}

func (m *CommManager) ListHiddenMessages(ctx context.Context, actorID string) (HiddenMessages, error) {
	if actorID == "" {
		return HiddenMessages{}, fmt.Errorf("%w: a message belongs to somebody", models.ErrChatInvalid)
	}
	chatQuery := m.ChatStore.store.Query(ctx, roleChatMessage).Where("author_id = ? AND withdrawn_at <> ''", actorID).Order("created_at, id")
	chat, err := specstore.List[models.ChatMessage](m.ChatStore.store, chatQuery, roleChatMessage)
	if err != nil {
		return HiddenMessages{}, classify(err)
	}
	dmQuery := m.DMStore.store.Query(ctx, roleDMMessage).Where("sender_id = ? AND withdrawn_at <> ''", actorID).Order("created_at, id")
	direct, err := specstore.List[models.DMMessage](m.DMStore.store, dmQuery, roleDMMessage)
	if err != nil {
		return HiddenMessages{}, classify(err)
	}
	return HiddenMessages{Chat: chat, Direct: direct}, nil
}
