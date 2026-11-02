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

type RoomsOf func(ctx context.Context, actorID string) ([]string, error)

func (m *CommManager) UseRoomsOf(r RoomsOf) { m.roomsOf = r }

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
	return m.threadViews(ctx, structureID, callerID, false)
}

func (m *CommManager) ListArchivedChatThreadsFor(ctx context.Context, structureID, callerID string) ([]models.ChatThreadView, error) {
	return m.threadViews(ctx, structureID, callerID, true)
}

func (m *CommManager) threadViews(ctx context.Context, structureID, callerID string, archivedOnes bool) ([]models.ChatThreadView, error) {
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
	archived, err := m.ChatStore.ArchivedThreads(ctx, structureID, callerID)
	if err != nil {
		return nil, err
	}
	for _, t := range threads {
		if archived[t.ID] != archivedOnes || !t.ReadableBy(callerID, leads) {
			continue
		}
		view := models.ChatThreadView{ChatThread: t, Unread: unread[t.ID]}
		if post, ok := latest[t.ID]; ok {
			at := post.CreatedAt
			last := post.Last()
			view.LastMessageAt = &at
			view.LastMessage = &last
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

func (m *CommManager) ListChatMessagesFor(ctx context.Context, structureID, threadID, callerID, afterID, beforeID string, limit int) ([]models.ChatMessage, error) {
	if err := m.inThread(ctx, structureID, threadID, callerID); err != nil {
		return nil, err
	}
	if limit < 0 {
		return nil, fmt.Errorf("%w: limit must not be negative", models.ErrChatInvalid)
	}
	window := chatMessageWindow{Limit: limit}
	var err error
	if window.After, err = m.messageInThread(ctx, structureID, threadID, afterID); err != nil {
		return nil, err
	}
	if window.Before, err = m.messageInThread(ctx, structureID, threadID, beforeID); err != nil {
		return nil, err
	}
	return m.ChatStore.ListMessagesWindow(ctx, structureID, threadID, window)
}

func (m *CommManager) messageInThread(ctx context.Context, structureID, threadID, messageID string) (*models.ChatMessage, error) {
	if messageID == "" {
		return nil, nil
	}
	msg, err := m.ChatStore.GetMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg.StructureID != structureID || msg.ThreadID != threadID {
		return nil, models.ErrChatNotFound
	}
	return &msg, nil
}

func (m *CommManager) PostChatMessageFor(ctx context.Context, structureID, threadID, callerID, body, attachmentKind, attachmentID string) (models.ChatMessage, error) {
	if err := m.inThread(ctx, structureID, threadID, callerID); err != nil {
		return models.ChatMessage{}, err
	}
	if strings.TrimSpace(body) == "" {
		return models.ChatMessage{}, fmt.Errorf("%w: body is required", models.ErrChatInvalid)
	}
	if err := (models.ChatMessage{AttachmentKind: attachmentKind, AttachmentID: attachmentID}).AttachmentIsWhole(); err != nil {
		return models.ChatMessage{}, err
	}
	t, err := m.ChatStore.threadIn(ctx, structureID, threadID)
	if err != nil {
		return models.ChatMessage{}, err
	}
	if t.Declined() {
		return models.ChatMessage{}, models.ErrChatThreadDeclined
	}
	return m.ChatStore.Post(ctx, models.ChatMessage{
		StructureID: structureID, ThreadID: threadID, AuthorID: callerID, Body: body,
		AttachmentKind: attachmentKind, AttachmentID: attachmentID,
	})
}
