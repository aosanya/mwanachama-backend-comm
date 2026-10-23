package mwanachamacomm

import (
	"context"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type RaiseRoom struct {
	StructureID string
	ThreadID    string
	SubjectKind string
	SubjectID   string
}

type Recipients func(ctx context.Context, event models.NotificationEvent, room RaiseRoom, defaults []string) ([]string, error)

func (m *CommManager) UseRecipients(r Recipients) { m.recipients = r }

func (m *CommManager) recipientsFor(ctx context.Context, event models.NotificationEvent, room RaiseRoom, defaults []string) ([]string, error) {
	if m.recipients == nil {
		return defaults, nil
	}
	named, err := m.recipients(ctx, event, room, append([]string(nil), defaults...))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{"": true}
	out := make([]string, 0, len(named))
	for _, a := range named {
		if seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out, nil
}
