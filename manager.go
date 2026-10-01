package mwanachamacomm

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// CommManager is the one value the declared route table dispatches against.
// It aggregates the five stores rather than replacing them, and adds the
// two things a declaration cannot state: the roster fences that decide
// whether a caller may read a thread at all, and the per-caller projection
// of a thread's own address index.
type CommManager struct {
	*ChatStore
	*DMStore
	*ModerationStore
	*AddressStore
	*NotificationStore
	*AddressDirectoryStore

	spec *spec.Spec
}

func NewCommManager(db *gorm.DB, s *spec.Spec, clock Clock, actWriter models.ActWriter, actorsTable string) (*CommManager, error) {
	chat, err := NewChatStore(db, s, clock)
	if err != nil {
		return nil, err
	}
	dm, err := NewDMStore(db, s, clock)
	if err != nil {
		return nil, err
	}
	moderation, err := NewModerationStore(db, s, clock, actWriter)
	if err != nil {
		return nil, err
	}
	addr, err := NewAddressStore(db, s, clock)
	if err != nil {
		return nil, err
	}
	notif, err := NewNotificationStore(db, s, clock)
	if err != nil {
		return nil, err
	}
	dir, err := NewAddressDirectoryStore(db, s, actorsTable, clock)
	if err != nil {
		return nil, err
	}
	return &CommManager{
		ChatStore: chat, DMStore: dm, ModerationStore: moderation,
		AddressStore: addr, NotificationStore: notif, AddressDirectoryStore: dir,
		spec: s,
	}, nil
}

func (m *CommManager) Spec() *spec.Spec { return m.spec }

// Three method names are on both ChatStore and DMStore, so a promoted
// selector would be ambiguous. Each domain's is named for its own object.

func (m *CommManager) PostChatMessage(ctx context.Context, msg models.ChatMessage) (models.ChatMessage, error) {
	return m.ChatStore.Post(ctx, msg)
}

func (m *CommManager) GetChatMessage(ctx context.Context, id string) (models.ChatMessage, error) {
	return m.ChatStore.GetMessage(ctx, id)
}

func (m *CommManager) ListChatMessages(ctx context.Context, structureID, threadID string) ([]models.ChatMessage, error) {
	return m.ChatStore.ListMessages(ctx, structureID, threadID)
}

func (m *CommManager) PostDMMessage(ctx context.Context, msg models.DMMessage) (models.DMMessage, error) {
	return m.DMStore.Post(ctx, msg)
}

func (m *CommManager) GetDMMessage(ctx context.Context, id string) (models.DMMessage, error) {
	return m.DMStore.GetMessage(ctx, id)
}

func (m *CommManager) ListMyAddresses(ctx context.Context, callerID string) ([]models.AddressMine, error) {
	out, err := m.AddressStore.ListFor(ctx, callerID)
	if err != nil {
		return nil, err
	}
	mine := make([]models.AddressMine, len(out))
	for i, a := range out {
		mine[i] = a.Mine()
	}
	return mine, nil
}

func (m *CommManager) RetireMyAddress(ctx context.Context, callerID string, index int) error {
	if index < 0 {
		return fmt.Errorf("%w: that is not an address index", models.ErrAddressBadSettings)
	}
	return m.AddressStore.Retire(ctx, callerID, index, m.clockNow())
}

func (m *CommManager) clockNow() time.Time { return m.AddressStore.clock() }

// MarkMyNotificationsRead takes the instant from the body when the caller
// supplies one and from the server's clock otherwise, which is what the
// hand-written route did. The read mark is the recipient's own assertion,
// so a caller may date it, but an absent value is never the zero instant.
func (m *CommManager) MarkMyNotificationsRead(ctx context.Context, callerID string, ids []string, readAt string) (int, error) {
	at := m.clockNow()
	if readAt != "" {
		parsed, err := time.Parse(time.RFC3339, readAt)
		if err != nil {
			return 0, fmt.Errorf("%w: read_at must be an RFC3339 timestamp", models.ErrNotificationInvalid)
		}
		at = parsed.UTC()
	}
	return m.NotificationStore.MarkRead(ctx, callerID, ids, at)
}

// SetNotificationPreferenceFor answers an unknown category with
// ErrNotificationNoSuchCategory, which the declared table maps to 404 —
// the status the hand-written route gave it. The vocabulary it checks
// against is the mounted domain's declared values, not a Go slice, which is
// what CM30 was filed against.
func (m *CommManager) SetNotificationPreferenceFor(ctx context.Context, callerID string, category models.NotificationCategory, muted bool) (models.NotificationPreference, error) {
	o, ok := m.spec.ByRole(roleNotificationPreference)
	if !ok {
		return models.NotificationPreference{}, fmt.Errorf("comm: no object fills %q", roleNotificationPreference)
	}
	declared := false
	for _, f := range o.Fields {
		if f.Name != "category" {
			continue
		}
		declared = holds(f.Values, string(category))
	}
	if !declared {
		return models.NotificationPreference{}, models.ErrNotificationNoSuchCategory
	}
	return m.NotificationStore.SetPreference(ctx, callerID, category, muted)
}
