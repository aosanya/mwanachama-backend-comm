package mwanachamacomm

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// maxReactionRunes bounds a reaction: an emoji, or a short cluster of them,
// never a sentence.
const maxReactionRunes = 8

// onRoster reports whether callerID holds any roster row on threadID, in
// any state. It is the fence GetDMThreadFor and ListDMParticipantsFor
// apply.
func (m *CommManager) onRoster(ctx context.Context, threadID, callerID string) (bool, error) {
	parts, err := m.DMStore.ListParticipants(ctx, threadID)
	if err != nil {
		return false, err
	}
	for _, p := range parts {
		if p.ActorID == callerID {
			return true, nil
		}
	}
	return false, nil
}

// activeOnRoster is the narrower fence: a caller who is merely invited, or
// who left or was kicked, is not reading messages.
func (m *CommManager) activeOnRoster(ctx context.Context, threadID, callerID string) (bool, error) {
	parts, err := m.DMStore.ListParticipants(ctx, threadID)
	if err != nil {
		return false, err
	}
	for _, p := range parts {
		if p.ActorID == callerID && p.State == models.DMStateActive {
			return true, nil
		}
	}
	return false, nil
}

// fence answers ErrDMNotFound — never a forbidden — when the caller is off
// the roster. A thread that exists but the caller is not on, and a thread
// that was never minted, must be indistinguishable, or the status code
// itself becomes an enumeration oracle.
func fence(allowed bool, err error) error {
	if err != nil {
		return err
	}
	if !allowed {
		return models.ErrDMNotFound
	}
	return nil
}

// forCaller resolves MyAddressIndex from whichever of the thread's two
// address pairs belongs to this reader. The owners and the raw indexes are
// json:"-", so this is the only thing standing between them and somebody
// who is not party to the pair.
func forCaller(t models.DMThread, callerID string) models.DMThread {
	switch callerID {
	case t.OpenedViaAddressOwner:
		t.MyAddressIndex = t.OpenedViaAddressIndex
	case t.SentFromAddressOwner:
		t.MyAddressIndex = t.SentFromAddressIndex
	}
	return t
}

func (m *CommManager) ListDMThreadsFor(ctx context.Context, callerID string) ([]models.DMThread, error) {
	out, err := m.DMStore.ListThreadsFor(ctx, callerID)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i] = forCaller(out[i], callerID)
	}
	return out, nil
}

func (m *CommManager) GetDMThreadFor(ctx context.Context, threadID, callerID string) (models.DMThread, error) {
	if err := fence(m.onRoster(ctx, threadID, callerID)); err != nil {
		return models.DMThread{}, err
	}
	t, err := m.DMStore.GetThread(ctx, threadID)
	if err != nil {
		return models.DMThread{}, err
	}
	return forCaller(t, callerID), nil
}

func (m *CommManager) ListDMParticipantsFor(ctx context.Context, threadID, callerID string) ([]models.DMParticipant, error) {
	if err := fence(m.onRoster(ctx, threadID, callerID)); err != nil {
		return nil, err
	}
	return m.DMStore.ListParticipants(ctx, threadID)
}

func (m *CommManager) ListDMMessagesFor(ctx context.Context, threadID, callerID string) ([]models.DMMessage, error) {
	if err := fence(m.activeOnRoster(ctx, threadID, callerID)); err != nil {
		return nil, err
	}
	return m.DMStore.ListMessages(ctx, threadID)
}

func (m *CommManager) ListDMReactionsFor(ctx context.Context, threadID, callerID string) ([]models.DMReaction, error) {
	if err := fence(m.activeOnRoster(ctx, threadID, callerID)); err != nil {
		return nil, err
	}
	return m.DMStore.ListReactions(ctx, threadID)
}

// activeOnMessageRoster resolves a message to its thread and fences on that.
// A message id nobody can read is ErrDMNotFound for the same reason a
// thread id is.
func (m *CommManager) activeOnMessageRoster(ctx context.Context, messageID, callerID string) error {
	msg, err := m.DMStore.GetMessage(ctx, messageID)
	if err != nil {
		return err
	}
	return fence(m.activeOnRoster(ctx, msg.ThreadID, callerID))
}

func (m *CommManager) SetDMReactionFor(ctx context.Context, messageID, callerID, emoji string) error {
	if err := m.activeOnMessageRoster(ctx, messageID, callerID); err != nil {
		return err
	}
	if emoji == "" {
		return fmt.Errorf("%w: emoji is required", models.ErrDMInvalidReaction)
	}
	if utf8.RuneCountInString(emoji) > maxReactionRunes {
		return fmt.Errorf("%w: emoji is too long", models.ErrDMInvalidReaction)
	}
	return m.DMStore.SetReaction(ctx, models.DMReaction{
		MessageID: messageID, ActorID: callerID, Emoji: emoji,
	})
}

func (m *CommManager) ClearDMReactionFor(ctx context.Context, messageID, callerID string) error {
	if err := m.activeOnMessageRoster(ctx, messageID, callerID); err != nil {
		return err
	}
	return m.DMStore.ClearReaction(ctx, messageID, callerID)
}

// ReEnableFor defaults the target to the caller, which is what makes a
// self re-enable a body-less POST. The hand-written handler did the same;
// a declared `required` argument would not, because required is schema
// documentation on the HTTP path rather than a gate (shared's S35).
func (m *CommManager) ReEnableFor(ctx context.Context, threadID, actorID, by string) (models.DMParticipant, error) {
	if actorID == "" {
		actorID = by
	}
	return m.DMStore.ReEnable(ctx, threadID, actorID, by)
}

// PublishDMDeviceKeyFor overwrites the two provenance fields from the
// session rather than refusing a body that claims them: they are session
// facts, and a client's opinion of them is silently replaced.
func (m *CommManager) PublishDMDeviceKeyFor(ctx context.Context, k models.DMDeviceKey, callerID, deviceID string) (models.DMDeviceKey, error) {
	k.ActorID = callerID
	k.PublishedBy = callerID
	k.DeviceID = deviceID
	return m.DMStore.PublishDeviceKey(ctx, k)
}
