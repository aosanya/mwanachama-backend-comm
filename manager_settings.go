package mwanachamacomm

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const notificationRulesSetting = "notification_rules"

type NotificationRules struct {
	Rules []models.NotificationRule `json:"rules"`
	Layer spec.Layer                `json:"layer"`
	By    string                    `json:"by,omitempty"`
	At    string                    `json:"at,omitempty"`
}

func (m *CommManager) NotificationRules(ctx context.Context) (NotificationRules, error) {
	effective, err := spec.ResolveStored(m.db.WithContext(ctx), m.spec, notificationRulesSetting)
	if err != nil {
		return NotificationRules{}, err
	}
	rules, err := models.ParseNotificationRules(effective.Value)
	if err != nil {
		return NotificationRules{}, err
	}
	return NotificationRules{Rules: rules, Layer: effective.Layer, By: effective.By, At: effective.At}, nil
}

func (m *CommManager) NotificationRule(ctx context.Context, event string) (models.NotificationRule, bool, error) {
	resolved, err := m.NotificationRules(ctx)
	if err != nil {
		return models.NotificationRule{}, false, err
	}
	for _, r := range resolved.Rules {
		if r.Event == event {
			return r, true, nil
		}
	}
	return models.NotificationRule{}, false, nil
}

const questionRoomsSetting = "question_rooms"

func (m *CommManager) QuestionRooms(ctx context.Context) ([]string, error) {
	effective, err := spec.ResolveStored(m.db.WithContext(ctx), m.spec, questionRoomsSetting)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(effective.Value)
	if err != nil {
		return nil, err
	}
	rooms := []string{}
	if string(raw) == "null" {
		return rooms, nil
	}
	if err := json.Unmarshal(raw, &rooms); err != nil {
		return nil, err
	}
	return rooms, nil
}

func (m *CommManager) SetQuestionRooms(ctx context.Context, structureIDs []string, by string) error {
	value := make([]any, 0, len(structureIDs))
	for _, id := range structureIDs {
		value = append(value, id)
	}
	_, err := spec.StoreOverride(m.db.WithContext(ctx), m.spec, questionRoomsSetting, value, by, m.clockNow().UTC().Format(time.RFC3339))
	return err
}

func (m *CommManager) isQuestionRoom(ctx context.Context, structureID string) (bool, error) {
	rooms, err := m.QuestionRooms(ctx)
	if err != nil {
		return false, err
	}
	return slices.Contains(rooms, structureID), nil
}

func (m *CommManager) SetChatQuestionRoomFor(ctx context.Context, structureID, callerID string, on bool) (models.ChatQuestionRoom, error) {
	if err := m.asLead(ctx, structureID, callerID); err != nil {
		return models.ChatQuestionRoom{}, err
	}
	rooms, err := m.QuestionRooms(ctx)
	if err != nil {
		return models.ChatQuestionRoom{}, err
	}
	rooms = slices.DeleteFunc(rooms, func(id string) bool { return id == structureID })
	if on {
		rooms = append(rooms, structureID)
	}
	if err := m.SetQuestionRooms(ctx, rooms, callerID); err != nil {
		return models.ChatQuestionRoom{}, err
	}
	return models.ChatQuestionRoom{StructureID: structureID, QuestionRoom: on}, nil
}
