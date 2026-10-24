package models

import (
	"encoding/json"
	"errors"
)

var ErrNotificationRuleInvalid = errors.New("notification rule is invalid")

type NotificationRule struct {
	Event    string   `json:"event"`
	Told     []string `json:"told"`
	Fallback []string `json:"fallback"`
	Mutable  bool     `json:"mutable"`
}

func ParseNotificationRules(value any) ([]NotificationRule, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, errors.Join(ErrNotificationRuleInvalid, err)
	}
	rules := []NotificationRule{}
	if string(raw) == "null" {
		return rules, nil
	}
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, errors.Join(ErrNotificationRuleInvalid, err)
	}
	seen := map[string]bool{}
	for _, r := range rules {
		if r.Event == "" || seen[r.Event] {
			return nil, ErrNotificationRuleInvalid
		}
		seen[r.Event] = true
		for _, list := range [][]string{r.Told, r.Fallback} {
			for _, a := range list {
				if a == "" {
					return nil, ErrNotificationRuleInvalid
				}
			}
		}
	}
	return rules, nil
}
