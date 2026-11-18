package mwanachamacomm

import (
	"context"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (s *ChatStore) GetThread(ctx context.Context, threadID string) (models.ChatThread, error) {
	var t models.ChatThread
	q := s.store.Query(ctx, roleChatThread).Where("id = ?", threadID)
	if err := s.store.Take(q, roleChatThread, &t, models.ErrChatNotFound); err != nil {
		return models.ChatThread{}, err
	}
	return t, nil
}
