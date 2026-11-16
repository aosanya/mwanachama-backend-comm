package mwanachamacomm

import (
	"context"
	"fmt"
)

func (s *PushStore) ReleaseActor(ctx context.Context, actorID string) error {
	if actorID == "" {
		return fmt.Errorf("%w: a subscription belongs to somebody", ErrPushSubscriptionInvalid)
	}
	return classify(deleteWhere(ctx, s.store, rolePushSubscription, "actor_id = ?", actorID))
}
