package mwanachamacomm

import (
	"context"
	"errors"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (m *CommManager) ChatReachOf(ctx context.Context, structureID, personID string) ([]models.ChatReach, error) {
	belongs, err := m.belongsTo(ctx, structureID, personID)
	if err != nil {
		return nil, err
	}
	if !belongs {
		return []models.ChatReach{}, nil
	}
	return m.reachIn(ctx, structureID, personID)
}

func (m *CommManager) ChatReachOfPerson(ctx context.Context, personID string) ([]models.ChatReach, error) {
	if m.roomsOf == nil || personID == "" {
		return nil, models.ErrChatNotFound
	}
	rooms, err := m.roomsOf(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := []models.ChatReach{}
	seen := map[string]bool{}
	inAnyRoom := false
	for _, structureID := range rooms {
		if seen[structureID] {
			continue
		}
		seen[structureID] = true
		belongs, err := m.belongsTo(ctx, structureID, personID)
		if err != nil {
			return nil, err
		}
		if !belongs {
			continue
		}
		inAnyRoom = true
		reach, err := m.reachIn(ctx, structureID, personID)
		if err != nil {
			return nil, err
		}
		out = append(out, reach...)
	}
	if !inAnyRoom {
		return nil, models.ErrChatNotFound
	}
	return out, nil
}

func (m *CommManager) belongsTo(ctx context.Context, structureID, personID string) (bool, error) {
	err := m.inStructure(ctx, structureID, personID)
	if errors.Is(err, models.ErrChatNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (m *CommManager) reachIn(ctx context.Context, structureID, personID string) ([]models.ChatReach, error) {
	threads, err := m.ChatStore.ListThreads(ctx, structureID)
	if err != nil {
		return nil, err
	}
	leads, err := m.leads(ctx, structureID, personID)
	if err != nil {
		return nil, err
	}
	out := make([]models.ChatReach, 0, len(threads))
	for _, t := range threads {
		reason, ok := t.ReadableReason(personID, leads)
		if !ok {
			continue
		}
		out = append(out, models.ChatReach{
			StructureID: structureID, ThreadID: t.ID, Label: t.Label,
			Audience: t.Audience, RequestedBy: t.RequestedBy, Reason: reason,
		})
	}
	return out, nil
}
