package mwanachamacomm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-shared/identity"
)

const identityModule = "comm"

var ErrNoStableIdentity = errors.New("comm: this object has no stable identity")

var identifiedRoles = map[string]bool{
	roleChatThread:      true,
	roleChatMessage:     true,
	roleSubjectThread:   true,
	roleSubjectMessage:  true,
	roleReport:          true,
	roleRemoval:         true,
	roleDismissal:       true,
	roleDispute:         true,
	roleNotification:    true,
	roleNotificationAck: true,
	roleStandingMessage: true,
}

type commIdentity struct{ m *CommManager }

func (m *CommManager) Identity() identity.Owner { return commIdentity{m} }

func (o commIdentity) StableIdentity(ctx context.Context, object, id string) (string, error) {
	if !identifiedRoles[object] {
		return "", fmt.Errorf("%s: %w", object, ErrNoStableIdentity)
	}
	exists, err := o.rowExists(ctx, object, id)
	if err != nil {
		return "", fmt.Errorf("StableIdentity: %w", err)
	}
	if !exists {
		return "", fmt.Errorf("%s %s: %w", object, id, identity.ErrNotFound)
	}
	raw, err := identity.New(identityModule, o.m.spec.Instance, o.m.spec.MountName()+"/"+object+"/"+id)
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", object, id, err)
	}
	return raw.String(), nil
}

func (o commIdentity) Resolve(ctx context.Context, raw string) (string, string, error) {
	r, err := identity.Parse(raw)
	if err != nil {
		return "", "", err
	}
	if r.Module != identityModule || r.Instance != o.m.spec.Instance {
		return "", "", fmt.Errorf("%s: %w", raw, identity.ErrNotOwned)
	}
	mount, rest, _ := strings.Cut(r.Identity, "/")
	if mount != o.m.spec.MountName() {
		return "", "", fmt.Errorf("%s: %w", raw, identity.ErrNotOwned)
	}
	object, id, _ := strings.Cut(rest, "/")
	if !identifiedRoles[object] || id == "" {
		return "", "", fmt.Errorf("%s: %w", raw, identity.ErrNotFound)
	}
	exists, err := o.rowExists(ctx, object, id)
	if err != nil {
		return "", "", fmt.Errorf("Resolve: %w", err)
	}
	if !exists {
		return "", "", fmt.Errorf("%s: %w", raw, identity.ErrNotFound)
	}
	return object, id, nil
}

func (o commIdentity) rowExists(ctx context.Context, role, id string) (bool, error) {
	object, ok := o.m.spec.ByRole(role)
	if !ok {
		return false, fmt.Errorf("comm: no object fills %q", role)
	}
	var rows []map[string]any
	err := o.m.db.WithContext(ctx).Table(o.m.spec.TableFor(object)).Select("id").Where("id = ?", id).Limit(1).Find(&rows).Error
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}
