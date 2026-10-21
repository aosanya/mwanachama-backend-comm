package mwanachamacomm

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type Leads func(ctx context.Context, structureID string) ([]string, error)

func (m *CommManager) UseLeads(l Leads) { m.leadsOf = l }

func (m *CommManager) StartChatThreadFor(ctx context.Context, structureID, callerID, label string) (models.ChatThread, error) {
	if err := m.inStructure(ctx, structureID, callerID); err != nil {
		return models.ChatThread{}, err
	}
	callerLeads, err := m.leads(ctx, structureID, callerID)
	if err != nil {
		return models.ChatThread{}, err
	}
	if callerLeads {
		return m.ChatStore.StartThread(ctx, structureID, label)
	}
	return m.startRequested(ctx, structureID, callerID, label)
}

func (m *CommManager) startRequested(ctx context.Context, structureID, callerID, label string) (models.ChatThread, error) {
	toTell, err := m.leadsToTell(ctx, structureID, callerID)
	if err != nil {
		return models.ChatThread{}, err
	}
	readable := func(t models.ChatThread) bool { return t.ReadableBy(callerID, false) }
	var thread models.ChatThread
	raised, err := m.inChatTransaction(ctx, func(chat *ChatStore, notices *NotificationStore) ([]models.Notification, error) {
		t, minted, err := chat.RequestThread(ctx, structureID, label, callerID, readable)
		thread = t
		if err != nil || !minted {
			return nil, err
		}
		return raiseEach(ctx, notices, toTell, models.Notification{
			AuthorActorID: callerID,
			Category:      models.CategoryChat,
			Event:         models.EventThreadRequested,
			SubjectKind:   models.SubjectChatThread,
			SubjectID:     t.ID,
			StructureID:   structureID,
		})
	})
	if err != nil {
		return models.ChatThread{}, err
	}
	m.pushRaised(ctx, raised)
	return thread, nil
}

func (m *CommManager) DeclineChatThreadFor(ctx context.Context, structureID, threadID, callerID string) (models.ChatThread, error) {
	if err := m.asLead(ctx, structureID, callerID); err != nil {
		return models.ChatThread{}, err
	}
	var thread models.ChatThread
	raised, err := m.inChatTransaction(ctx, func(chat *ChatStore, notices *NotificationStore) ([]models.Notification, error) {
		t, declined, err := chat.DeclineThread(ctx, structureID, threadID, callerID)
		thread = t
		if err != nil || !declined || t.RequestedBy == callerID {
			return nil, err
		}
		return raiseEach(ctx, notices, []string{t.RequestedBy}, models.Notification{
			AuthorActorID: callerID,
			Category:      models.CategoryChat,
			Event:         models.EventThreadDeclined,
			SubjectKind:   models.SubjectChatThread,
			SubjectID:     t.ID,
			StructureID:   structureID,
		})
	})
	if err != nil {
		return models.ChatThread{}, err
	}
	m.pushRaised(ctx, raised)
	return thread, nil
}

func (m *CommManager) leadsToTell(ctx context.Context, structureID, requesterID string) ([]string, error) {
	if m.leadsOf == nil {
		return nil, nil
	}
	named, err := m.leadsOf(ctx, structureID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{requesterID: true, "": true}
	out := make([]string, 0, len(named))
	for _, a := range named {
		if seen[a] {
			continue
		}
		seen[a] = true
		err := m.inStructure(ctx, structureID, a)
		if errors.Is(err, models.ErrChatNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		lead, err := m.leads(ctx, structureID, a)
		if err != nil {
			return nil, err
		}
		if lead {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *CommManager) inChatTransaction(ctx context.Context, work func(chat *ChatStore, notices *NotificationStore) ([]models.Notification, error)) ([]models.Notification, error) {
	var raised []models.Notification
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		chat, err := NewChatStore(tx, m.spec, m.ChatStore.clock)
		if err != nil {
			return err
		}
		notices, err := NewNotificationStore(tx, m.spec, m.ChatStore.clock)
		if err != nil {
			return err
		}
		raised, err = work(chat, notices)
		return err
	})
	return raised, err
}

func (m *CommManager) pushRaised(ctx context.Context, raised []models.Notification) {
	if m.NotificationStore.onRaised == nil {
		return
	}
	for _, n := range raised {
		m.NotificationStore.onRaised(ctx, n)
	}
}

func raiseEach(ctx context.Context, notices *NotificationStore, recipients []string, template models.Notification) ([]models.Notification, error) {
	raised := make([]models.Notification, 0, len(recipients))
	for _, actorID := range recipients {
		n := template
		n.ActorID = actorID
		got, err := notices.Raise(ctx, n)
		switch {
		case errors.Is(err, models.ErrNotificationMuted):
		case err != nil:
			return nil, err
		default:
			raised = append(raised, got)
		}
	}
	return raised, nil
}
