package mwanachamacomm

import (
	"context"
	"errors"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (m *CommManager) GetSubjectThreadFor(ctx context.Context, threadID, callerID string) (models.SubjectThread, error) {
	t, _, err := m.readableThread(ctx, threadID, callerID)
	return t, err
}

func (m *CommManager) GetChatMessageFor(ctx context.Context, messageID, callerID string) (models.ChatMessage, error) {
	if callerID == "" {
		return models.ChatMessage{}, models.ErrChatNotFound
	}
	msg, err := m.ChatStore.GetMessage(ctx, messageID)
	if err != nil {
		return models.ChatMessage{}, err
	}
	if msg.AuthorID == callerID {
		if err := m.inStructure(ctx, msg.StructureID, callerID); err != nil {
			return models.ChatMessage{}, err
		}
		return msg, nil
	}
	if err := m.inThread(ctx, msg.StructureID, msg.ThreadID, callerID); err != nil {
		return models.ChatMessage{}, err
	}
	if msg.WithdrawnAt != "" {
		return models.ChatMessage{}, models.ErrChatNotFound
	}
	removed, err := m.messageRemoved(ctx, messageID)
	if err != nil {
		return models.ChatMessage{}, err
	}
	if removed {
		return models.ChatMessage{}, models.ErrChatNotFound
	}
	return msg, nil
}

func (m *CommManager) GetChatThreadFor(ctx context.Context, threadID, callerID string) (models.ChatThread, error) {
	if callerID == "" {
		return models.ChatThread{}, models.ErrChatNotFound
	}
	t, err := m.ChatStore.GetThread(ctx, threadID)
	if err != nil {
		return models.ChatThread{}, err
	}
	if err := m.inThread(ctx, t.StructureID, t.ID, callerID); err != nil {
		return models.ChatThread{}, err
	}
	return t, nil
}

func (m *CommManager) messageRemoved(ctx context.Context, messageID string) (bool, error) {
	_, err := m.ModerationStore.GetRemovalForMessage(ctx, messageID)
	if errors.Is(err, models.ErrModerationNotFound) {
		return false, nil
	}
	return err == nil, err
}
