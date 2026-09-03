package mwanachamacomm

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// DMMemoryStore is the in-memory implementation of DMRepository.
type DMMemoryStore struct {
	mu           sync.RWMutex
	threads      map[string]DMThread
	participants []DMParticipant
	messages     map[string]DMMessage
	deviceKeys   map[string]DMDeviceKey // memberID+"\x00"+keyID
	// reactions is keyed messageID\x00memberID — one member, one reaction per
	// message, so reacting again replaces rather than accumulates (DEV-1530).
	reactions map[string]DMReaction
	ids       *IDGen
	clock     Clock
}

// NewDMMemoryStore constructs an empty DMMemoryStore.
func NewDMMemoryStore(ids *IDGen, clock Clock) *DMMemoryStore {
	return &DMMemoryStore{
		threads:      map[string]DMThread{},
		participants: []DMParticipant{},
		messages:     map[string]DMMessage{},
		deviceKeys:   map[string]DMDeviceKey{},
		reactions:    map[string]DMReaction{},
		ids:          ids,
		clock:        clock,
	}
}

// CreateThread mints a thread and seeds the initial roster (creator is active
// admin; other initial members are invited).
func (s *DMMemoryStore) CreateThread(_ context.Context, t DMThread, initial []string) (DMThread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = s.ids.New("dm")
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = s.clock()
	}
	s.threads[t.ID] = t
	if t.CreatedBy != "" {
		s.participants = append(s.participants, DMParticipant{
			ThreadID: t.ID, MemberID: t.CreatedBy,
			State: DMStateActive, IsAdmin: true, UpdatedAt: s.clock(),
		})
	}
	for _, m := range initial {
		if m == t.CreatedBy {
			continue
		}
		s.participants = append(s.participants, DMParticipant{
			ThreadID: t.ID, MemberID: m,
			State: DMStateInvited, UpdatedAt: s.clock(),
		})
	}
	return t, nil
}

// GetThread returns a thread by id.
func (s *DMMemoryStore) GetThread(_ context.Context, id string) (DMThread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.threads[id]
	if !ok {
		return DMThread{}, ErrDMNotFound
	}
	return t, nil
}

// ListThreadsFor returns every thread a member currently participates in.
func (s *DMMemoryStore) ListThreadsFor(_ context.Context, memberID string) ([]DMThread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := map[string]struct{}{}
	for _, p := range s.participants {
		if p.MemberID == memberID && (p.State == DMStateActive || p.State == DMStateInvited) {
			ids[p.ThreadID] = struct{}{}
		}
	}
	out := []DMThread{}
	for id := range ids {
		out = append(out, s.threads[id])
	}
	sort.Slice(out, func(i, j int) bool {
		return lessByTimeThenID(out[i].CreatedAt, out[j].CreatedAt, out[i].ID, out[j].ID)
	})
	return out, nil
}

func (s *DMMemoryStore) findParticipant(threadID, memberID string) int {
	for i := range s.participants {
		if s.participants[i].ThreadID == threadID && s.participants[i].MemberID == memberID {
			return i
		}
	}
	return -1
}

// Invite adds an invited member to a thread (admin-only).
//
// **An invite never writes over an acceptance** (ErrDMAlreadyActive):
// re-asking somebody who is already in used to demote them to `invited` and
// lock them out of a thread they were talking in, which is `Kick`'s job and
// not this one's. Re-asking somebody still pending is allowed and idempotent.
func (s *DMMemoryStore) Invite(_ context.Context, threadID, memberID, by string) (DMParticipant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAdmin(threadID, by) {
		return DMParticipant{}, errors.New("directmessage: not admin")
	}
	if i := s.findParticipant(threadID, memberID); i >= 0 {
		if s.participants[i].State == DMStateActive {
			return DMParticipant{}, ErrDMAlreadyActive
		}
		s.participants[i].State = DMStateInvited
		s.participants[i].UpdatedAt = s.clock()
		return s.participants[i], nil
	}
	p := DMParticipant{ThreadID: threadID, MemberID: memberID, State: DMStateInvited, UpdatedAt: s.clock()}
	s.participants = append(s.participants, p)
	return p, nil
}

// Accept turns an invite into active membership.
func (s *DMMemoryStore) Accept(_ context.Context, threadID, memberID string) (DMParticipant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.findParticipant(threadID, memberID)
	if i < 0 {
		return DMParticipant{}, ErrDMNotFound
	}
	s.participants[i].State = DMStateActive
	s.participants[i].UpdatedAt = s.clock()
	return s.participants[i], nil
}

// Leave marks the member as having left the thread.
func (s *DMMemoryStore) Leave(_ context.Context, threadID, memberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.findParticipant(threadID, memberID)
	if i < 0 {
		return ErrDMNotFound
	}
	s.participants[i].State = DMStateLeft
	s.participants[i].UpdatedAt = s.clock()
	return nil
}

// Kick marks the member as removed by an admin.
func (s *DMMemoryStore) Kick(_ context.Context, threadID, memberID, by string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAdmin(threadID, by) {
		return errors.New("directmessage: not admin")
	}
	i := s.findParticipant(threadID, memberID)
	if i < 0 {
		return ErrDMNotFound
	}
	s.participants[i].State = DMStateKicked
	s.participants[i].UpdatedAt = s.clock()
	return nil
}

// Promote makes the target an admin.
func (s *DMMemoryStore) Promote(_ context.Context, threadID, memberID, by string) (DMParticipant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAdmin(threadID, by) {
		return DMParticipant{}, errors.New("directmessage: not admin")
	}
	i := s.findParticipant(threadID, memberID)
	if i < 0 {
		return DMParticipant{}, ErrDMNotFound
	}
	s.participants[i].IsAdmin = true
	s.participants[i].UpdatedAt = s.clock()
	return s.participants[i], nil
}

// ReEnable is two acts wearing one route.
//
// **The member bringing back a thread they emptied** (memberID == by) is not
// an admin act: `Leave` has already set the caller's own row to `left`, so
// `isAdmin` — which requires `StateActive` — could never pass. What it takes
// instead is that nobody else is active on the thread and the caller is a
// member who *left* it (never one who was kicked — that removal stands until
// an admin undoes it). They come back **active, not invited**.
//
// **An admin restoring somebody else** who left or was kicked is the other
// act: the target comes back `invited`, and has to accept.
func (s *DMMemoryStore) ReEnable(_ context.Context, threadID, memberID, by string) (DMParticipant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.findParticipant(threadID, memberID)
	if i < 0 {
		return DMParticipant{}, ErrDMNotFound
	}
	if memberID == by {
		if s.participants[i].State != DMStateLeft || s.hasActive(threadID) {
			return DMParticipant{}, ErrDMNotLastToLeave
		}
		s.participants[i].State = DMStateActive
		s.participants[i].UpdatedAt = s.clock()
		return s.participants[i], nil
	}
	if !s.isAdmin(threadID, by) {
		return DMParticipant{}, errors.New("directmessage: not admin")
	}
	s.participants[i].State = DMStateInvited
	s.participants[i].UpdatedAt = s.clock()
	return s.participants[i], nil
}

// hasActive reports whether anybody is currently active on the thread.
func (s *DMMemoryStore) hasActive(threadID string) bool {
	for i := range s.participants {
		if s.participants[i].ThreadID == threadID && s.participants[i].State == DMStateActive {
			return true
		}
	}
	return false
}

// ListParticipants returns every participant row for a thread.
func (s *DMMemoryStore) ListParticipants(_ context.Context, threadID string) ([]DMParticipant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []DMParticipant{}
	for _, p := range s.participants {
		if p.ThreadID == threadID {
			out = append(out, p)
		}
	}
	// Matches `ORDER BY updated_at, member_id`.
	sort.Slice(out, func(i, j int) bool {
		return lessByTimeThenID(out[i].UpdatedAt, out[j].UpdatedAt, out[i].MemberID, out[j].MemberID)
	})
	return out, nil
}

// Post stores a ciphertext message.
func (s *DMMemoryStore) Post(_ context.Context, m DMMessage) (DMMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ID == "" {
		m.ID = s.ids.New("dmmsg")
	} else if _, exists := s.messages[m.ID]; exists {
		return DMMessage{}, Conflict("comm_dm_message_pkey")
	}
	m.CreatedAt = s.clock()
	s.messages[m.ID] = m
	return m, nil
}

// ListMessages returns every message in a thread that is still within its
// deadline.
//
// The same filter Postgres applies in SQL (DEV-1541), against this store's
// own clock: refusing the row from the first request after the deadline is
// what makes a disappearing message exact.
func (s *DMMemoryStore) ListMessages(_ context.Context, threadID string) ([]DMMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Off the thread, where the timer was frozen at open — never off the
	// address, whose owner may have changed it since.
	var ttl *int
	if t, ok := s.threads[threadID]; ok {
		ttl = t.MessageTTLSeconds
	}
	now := s.clock()
	out := []DMMessage{}
	for _, m := range s.messages {
		if m.ThreadID != threadID {
			continue
		}
		if ttl != nil &&
			!m.CreatedAt.Add(time.Duration(*ttl)*time.Second).After(now) {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return lessByTimeThenID(out[i].CreatedAt, out[j].CreatedAt, out[i].ID, out[j].ID)
	})
	return out, nil
}

// PublishDeviceKey stores a device public key for a member.
func (s *DMMemoryStore) PublishDeviceKey(_ context.Context, k DMDeviceKey) (DMDeviceKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k.KeyID == "" {
		k.KeyID = s.ids.New("dkey")
	}
	if k.CreatedAt.IsZero() {
		k.CreatedAt = s.clock()
	}
	// Re-publishing the same key id un-retires it, matching the Postgres
	// upsert's `retired_at = NULL`.
	k.RetiredAt = nil
	s.deviceKeys[k.MemberID+"\x00"+k.KeyID] = k

	// DEV-1265 · a handset publishing a new key retires that handset's
	// previous one. Scoped to DeviceID and not to the member.
	if k.DeviceID != "" {
		now := s.clock()
		for mapKey, other := range s.deviceKeys {
			if other.MemberID == k.MemberID && other.DeviceID == k.DeviceID &&
				other.KeyID != k.KeyID && other.RetiredAt == nil {
				retired := now
				other.RetiredAt = &retired
				s.deviceKeys[mapKey] = other
			}
		}
	}
	return k, nil
}

// LookupDeviceKeys returns every device key held for the requested members.
func (s *DMMemoryStore) LookupDeviceKeys(_ context.Context, memberIDs []string) ([]DMDeviceKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	want := map[string]struct{}{}
	for _, m := range memberIDs {
		want[m] = struct{}{}
	}
	out := []DMDeviceKey{}
	for _, k := range s.deviceKeys {
		// DEV-1265 · a retired key is never returned.
		if k.RetiredAt != nil {
			continue
		}
		if _, ok := want[k.MemberID]; ok {
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MemberID == out[j].MemberID {
			return out[i].KeyID < out[j].KeyID
		}
		return out[i].MemberID < out[j].MemberID
	})
	return out, nil
}

func (s *DMMemoryStore) isAdmin(threadID, memberID string) bool {
	i := s.findParticipant(threadID, memberID)
	if i < 0 {
		return false
	}
	return s.participants[i].IsAdmin && s.participants[i].State == DMStateActive
}

// SetReaction records one member's emoji on one message, replacing any
// earlier one from the same member.
func (s *DMMemoryStore) SetReaction(_ context.Context, r DMReaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.messages[r.MessageID]; !ok {
		return ErrDMNotFound
	}
	if s.reactions == nil {
		s.reactions = map[string]DMReaction{}
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = s.clock()
	}
	s.reactions[r.MessageID+"\x00"+r.MemberID] = r
	return nil
}

// ClearReaction removes a member's reaction. Removing one that is not there
// is not an error — the caller asked for a state and it already holds.
func (s *DMMemoryStore) ClearReaction(_ context.Context, messageID, memberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.reactions, messageID+"\x00"+memberID)
	return nil
}

// ListReactions returns every reaction on every message of a thread.
func (s *DMMemoryStore) ListReactions(_ context.Context, threadID string) ([]DMReaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []DMReaction{}
	for _, r := range s.reactions {
		if m, ok := s.messages[r.MessageID]; ok && m.ThreadID == threadID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MessageID != out[j].MessageID {
			return out[i].MessageID < out[j].MessageID
		}
		return out[i].MemberID < out[j].MemberID
	})
	return out, nil
}

// GetMessage returns one message by id.
func (s *DMMemoryStore) GetMessage(_ context.Context, id string) (DMMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.messages[id]
	if !ok {
		return DMMessage{}, ErrDMNotFound
	}
	return m, nil
}
