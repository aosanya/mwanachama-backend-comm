package mwanachamacomm

import (
	"context"
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type Belonging func(ctx context.Context, actorID, structureID string) (bool, error)

func (m *CommManager) UseBelonging(b Belonging) { m.belonging = b }

func (m *CommManager) inStructure(ctx context.Context, structureID, callerID string) error {
	if m.belonging == nil || callerID == "" || structureID == "" {
		return models.ErrChatNotFound
	}
	ok, err := m.belonging(ctx, callerID, structureID)
	if err != nil {
		return err
	}
	if !ok {
		return models.ErrChatNotFound
	}
	return nil
}

func (m *CommManager) inThread(ctx context.Context, structureID, threadID, callerID string) error {
	if err := m.inStructure(ctx, structureID, callerID); err != nil {
		return err
	}
	_, err := m.ChatStore.threadIn(ctx, structureID, threadID)
	return err
}

func (m *CommManager) ListChatThreadsFor(ctx context.Context, structureID, callerID string) ([]models.ChatThread, error) {
	if err := m.inStructure(ctx, structureID, callerID); err != nil {
		return nil, err
	}
	return m.ChatStore.ListThreads(ctx, structureID)
}

func (m *CommManager) ListChatMessagesFor(ctx context.Context, structureID, threadID, callerID string) ([]models.ChatMessage, error) {
	if err := m.inThread(ctx, structureID, threadID, callerID); err != nil {
		return nil, err
	}
	return m.ChatStore.ListMessages(ctx, structureID, threadID)
}

func (m *CommManager) PostChatMessageFor(ctx context.Context, structureID, threadID, callerID, body string) (models.ChatMessage, error) {
	if err := m.inThread(ctx, structureID, threadID, callerID); err != nil {
		return models.ChatMessage{}, err
	}
	if strings.TrimSpace(body) == "" {
		return models.ChatMessage{}, fmt.Errorf("%w: body is required", models.ErrChatInvalid)
	}
	return m.ChatStore.Post(ctx, models.ChatMessage{
		StructureID: structureID, ThreadID: threadID, AuthorID: callerID, Body: body,
	})
}
