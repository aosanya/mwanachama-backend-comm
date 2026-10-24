package mwanachamacomm

import (
	"context"

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
