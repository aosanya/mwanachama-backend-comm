package mwanachamacomm

import (
	"context"
	"fmt"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (m *CommManager) standingFence(ctx context.Context, structureID, threadID, callerID string) error {
	if threadID == "" {
		return m.inStructure(ctx, structureID, callerID)
	}
	return m.inThread(ctx, structureID, threadID, callerID)
}

func instantArg(name, raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s %q is not an RFC3339 instant", models.ErrStandingInvalid, name, raw)
	}
	return t, nil
}

func (m *CommManager) CreateStandingMessageFor(ctx context.Context, structureID, callerID, threadID, body, rule string, dates []string, zone, startsAt, endsAt string) (models.StandingMessage, error) {
	if err := m.asLead(ctx, structureID, callerID); err != nil {
		return models.StandingMessage{}, err
	}
	if err := m.standingFence(ctx, structureID, threadID, callerID); err != nil {
		return models.StandingMessage{}, err
	}
	start, err := instantArg("starts_at", startsAt)
	if err != nil {
		return models.StandingMessage{}, err
	}
	msg := models.StandingMessage{
		StructureID: structureID, ThreadID: threadID, AuthorID: callerID,
		Body: body, Rule: rule, Dates: dates, Zone: zone, StartsAt: start,
	}
	if endsAt != "" {
		end, err := instantArg("ends_at", endsAt)
		if err != nil {
			return models.StandingMessage{}, err
		}
		msg.EndsAt = &end
	}
	return m.standings.CreateStanding(ctx, msg)
}

func (m *CommManager) GetStandingMessageFor(ctx context.Context, structureID, id, callerID string) (models.StandingMessage, error) {
	if err := m.inStructure(ctx, structureID, callerID); err != nil {
		return models.StandingMessage{}, err
	}
	msg, err := m.standings.GetStanding(ctx, structureID, id)
	if err != nil {
		return models.StandingMessage{}, err
	}
	if err := m.standingFence(ctx, structureID, msg.ThreadID, callerID); err != nil {
		return models.StandingMessage{}, models.ErrStandingNotFound
	}
	return msg, nil
}

func (m *CommManager) ListStandingMessagesFor(ctx context.Context, structureID, callerID string) ([]models.StandingMessage, error) {
	if err := m.inStructure(ctx, structureID, callerID); err != nil {
		return nil, err
	}
	all, err := m.standings.ListStanding(ctx, structureID)
	if err != nil {
		return nil, err
	}
	out := make([]models.StandingMessage, 0, len(all))
	for _, msg := range all {
		if msg.ThreadID != "" && m.inThread(ctx, structureID, msg.ThreadID, callerID) != nil {
			continue
		}
		out = append(out, msg)
	}
	return out, nil
}

func (m *CommManager) StopStandingMessageFor(ctx context.Context, structureID, id, callerID string) (models.StandingMessage, error) {
	if err := m.asLead(ctx, structureID, callerID); err != nil {
		return models.StandingMessage{}, err
	}
	if _, err := m.GetStandingMessageFor(ctx, structureID, id, callerID); err != nil {
		return models.StandingMessage{}, err
	}
	return m.standings.StopStanding(ctx, structureID, id, callerID)
}

func (m *CommManager) SendDueStandingMessages(ctx context.Context, now time.Time) ([]models.ChatMessage, error) {
	return m.standings.SendDueStandingMessages(ctx, now)
}
