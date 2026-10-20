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

func (m *CommManager) ListChatThreadsFor(ctx context.Context, structureID, callerID string) ([]models.ChatThreadView, error) {
	if err := m.inStructure(ctx, structureID, callerID); err != nil {
		return nil, err
	}
	threads, err := m.ChatStore.ListThreads(ctx, structureID)
	if err != nil {
		return nil, err
	}
	unread, err := m.ChatStore.UnreadCounts(ctx, structureID, callerID)
	if err != nil {
		return nil, err
	}
	out := make([]models.ChatThreadView, len(threads))
	for i, t := range threads {
		out[i] = models.ChatThreadView{ChatThread: t, Unread: unread[t.ID]}
	}
	return out, nil
}

func (m *CommManager) StartChatThreadFor(ctx context.Context, structureID, callerID, label string) (models.ChatThread, error) {
	if err := m.inStructure(ctx, structureID, callerID); err != nil {
		return models.ChatThread{}, err
	}
	return m.ChatStore.StartThread(ctx, structureID, label)
}

func (m *CommManager) MarkChatThreadReadFor(ctx context.Context, structureID, threadID, readerID, messageID string) (models.ChatReadMarker, error) {
	if err := m.inThread(ctx, structureID, threadID, readerID); err != nil {
		return models.ChatReadMarker{}, err
	}
	readAt := m.ChatStore.clock()
	if messageID != "" {
		seen, err := m.ChatStore.GetMessage(ctx, messageID)
		if err != nil {
			return models.ChatReadMarker{}, err
		}
		if seen.StructureID != structureID || seen.ThreadID != threadID {
			return models.ChatReadMarker{}, models.ErrChatNotFound
		}
		readAt = seen.CreatedAt
	}
	return m.ChatStore.MarkRead(ctx, structureID, threadID, readerID, readAt)
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
