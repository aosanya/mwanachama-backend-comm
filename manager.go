package mwanachamacomm

import (
	"context"
	"fmt"
	"github.com/aosanya/mwanachama-backend-shared/consent"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type CommManager struct {
	*ChatStore
	*DMStore
	*ModerationStore
	*AddressStore
	*NotificationStore
	*AddressDirectoryStore
	*PushStore

	subjects  *SubjectStore
	standings *StandingStore

	db         *gorm.DB
	clock      Clock
	spec       *spec.Spec
	belonging  Belonging
	leading    Leading
	roomsOf    RoomsOf
	authorKind AuthorKindOf
	leadsOf    Leads
	standing   Standing
	answerers  Answerers
	recipients Recipients
	audience   Audience
	raisable   Raisable
	push       *pushDelivery
	runID      string
	consent    consent.Gate
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
	subjects, err := NewSubjectStore(db, s, clock)
	if err != nil {
		return nil, err
	}
	push, err := NewPushStore(db, s, clock)
	if err != nil {
		return nil, err
	}
	standings, err := NewStandingStore(db, s, clock)
	if err != nil {
		return nil, err
	}
	m := &CommManager{
		ChatStore: chat, DMStore: dm, ModerationStore: moderation,
		AddressStore: addr, NotificationStore: notif, AddressDirectoryStore: dir,
		subjects: subjects, standings: standings, db: db, clock: clock, spec: s,
		PushStore: push,
	}
	standings.kindOf = m.kindOfAuthor
	return m, nil
}

func (m *CommManager) Spec() *spec.Spec { return m.spec }

func (m *CommManager) UseConsentGate(g consent.Gate) { m.consent = g }

func (m *CommManager) PostChatMessage(ctx context.Context, msg models.ChatMessage) (models.ChatMessage, error) {
	if err := consent.Require(ctx, m.consent, msg.AuthorID); err != nil {
		return models.ChatMessage{}, err
	}
	return m.ChatStore.Post(ctx, msg)
}

func (m *CommManager) GetChatMessage(ctx context.Context, id string) (models.ChatMessage, error) {
	return m.ChatStore.GetMessage(ctx, id)
}

func (m *CommManager) ListChatMessages(ctx context.Context, structureID, threadID string) ([]models.ChatMessage, error) {
	return m.ChatStore.ListMessages(ctx, structureID, threadID)
}

func (m *CommManager) PostDMMessage(ctx context.Context, msg models.DMMessage) (models.DMMessage, error) {
	if err := consent.Require(ctx, m.consent, msg.SenderID); err != nil {
		return models.DMMessage{}, err
	}
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
