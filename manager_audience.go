package mwanachamacomm

import (
	"context"
	"errors"
	"fmt"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

var ErrNotificationNoAudience = errors.New("notification: no audience resolves for that record")

type Audience func(ctx context.Context, subjectKind models.NotificationSubjectKind, subjectID string) ([]string, error)

func (m *CommManager) UseAudience(a Audience) { m.audience = a }

type AudienceMiss struct {
	ActorID string
	Reason  error
}

type AudienceRaise struct {
	Resolved int
	Raised   []models.Notification
	Missed   []AudienceMiss
}

func (r AudienceRaise) Complete() bool { return len(r.Raised)+len(r.Missed) == r.Resolved }

func (m *CommManager) RaiseToAudience(ctx context.Context, template models.Notification) (AudienceRaise, error) {
	if template.ActorID != "" {
		return AudienceRaise{}, fmt.Errorf("%w: an audience raise names its recipients through the record, never one actor", models.ErrNotificationInvalid)
	}
	if template.SubjectKind == "" || template.SubjectID == "" {
		return AudienceRaise{}, fmt.Errorf("%w: an audience raise names the record whose set it reaches", models.ErrNotificationInvalid)
	}
	recipients, err := m.audienceOf(ctx, template.SubjectKind, template.SubjectID)
	if err != nil {
		return AudienceRaise{}, err
	}
	probe := template
	probe.ActorID = recipients[0]
	if err := Check(m.spec, roleNotification, probe); err != nil {
		return AudienceRaise{}, err
	}
	if err := probe.Validate(); err != nil {
		return AudienceRaise{}, err
	}
	out := AudienceRaise{Resolved: len(recipients)}
	for _, actor := range recipients {
		n := template
		n.ActorID = actor
		raised, err := m.NotificationStore.Raise(ctx, n)
		switch {
		case err == nil:
			out.Raised = append(out.Raised, raised)
		case errors.Is(err, models.ErrNotificationMuted), errors.Is(err, models.ErrNotificationCapSpent):
			out.Missed = append(out.Missed, AudienceMiss{ActorID: actor, Reason: err})
		default:
			return out, err
		}
	}
	return out, nil
}

func (m *CommManager) audienceOf(ctx context.Context, kind models.NotificationSubjectKind, id string) ([]string, error) {
	if m.audience == nil {
		return nil, ErrNotificationNoAudience
	}
	named, err := m.audience(ctx, kind, id)
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
	if len(out) == 0 {
		return nil, ErrNotificationNoAudience
	}
	return out, nil
}
