package mwanachamacomm

import (
	"context"
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type Belonging func(ctx context.Context, actorID, structureID string) (bool, error)

func (m *CommManager) UseBelonging(b Belonging) { m.belonging = b }

type Leading func(ctx context.Context, actorID, structureID string) (bool, error)

func (m *CommManager) UseLeading(l Leading) { m.leading = l }

func (m *CommManager) leads(ctx context.Context, structureID, callerID string) (bool, error) {
	if m.leading == nil || callerID == "" || structureID == "" {
		return false, nil
	}
	return m.leading(ctx, callerID, structureID)
}

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
	t, err := m.ChatStore.threadIn(ctx, structureID, threadID)
	if err != nil {
		return err
	}
	if t.Audience != models.ChatAudienceLeads {
		return nil
	}
	leads, err := m.leads(ctx, structureID, callerID)
	if err != nil {
		return err
	}
	if !t.ReadableBy(callerID, leads) {
		return models.ErrChatNotFound
	}
	return nil
}

func (m *CommManager) asLead(ctx context.Context, structureID, callerID string) error {
	if err := m.inStructure(ctx, structureID, callerID); err != nil {
		return err
	}
	leads, err := m.leads(ctx, structureID, callerID)
	if err != nil {
		return err
	}
	if !leads {
		return models.ErrChatNotFound
	}
	return nil
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
	latest, err := m.ChatStore.LatestPosts(ctx, structureID)
	if err != nil {
		return nil, err
	}
	leads, err := m.leads(ctx, structureID, callerID)
	if err != nil {
		return nil, err
	}
	out := make([]models.ChatThreadView, 0, len(threads))
	for _, t := range threads {
		if !t.ReadableBy(callerID, leads) {
			continue
		}
		view := models.ChatThreadView{ChatThread: t, Unread: unread[t.ID]}
		if at, ok := latest[t.ID]; ok {
			view.LastMessageAt = &at
		}
		out = append(out, view)
	}
	return out, nil
}

func (m *CommManager) OpenChatThreadFor(ctx context.Context, structureID, threadID, callerID string) (models.ChatThread, error) {
	if err := m.asLead(ctx, structureID, callerID); err != nil {
		return models.ChatThread{}, err
	}
	return m.ChatStore.OpenThread(ctx, structureID, threadID)
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
