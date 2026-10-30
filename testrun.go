package mwanachamacomm

import (
	"context"
	"database/sql"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type discardActs struct{}

func (discardActs) WriteAct(context.Context, *sql.Tx, models.ActEntry) error { return nil }

func (m *CommManager) ForRun(runID string) (*CommManager, error) {
	if runID == "" {
		return nil, specstore.ErrRunIDEmpty
	}
	chat, err := m.ChatStore.inRun(runID)
	if err != nil {
		return nil, err
	}
	dm, err := m.DMStore.inRun(runID)
	if err != nil {
		return nil, err
	}
	moderation, err := m.ModerationStore.inRun(runID)
	if err != nil {
		return nil, err
	}
	addresses, err := m.AddressStore.inRun(runID)
	if err != nil {
		return nil, err
	}
	notices, err := m.NotificationStore.inRun(runID)
	if err != nil {
		return nil, err
	}
	directory, err := m.AddressDirectoryStore.inRun(runID)
	if err != nil {
		return nil, err
	}
	push, err := m.PushStore.inRun(runID)
	if err != nil {
		return nil, err
	}
	subjects, err := m.subjects.inRun(runID)
	if err != nil {
		return nil, err
	}
	standings, err := m.standings.inRun(runID)
	if err != nil {
		return nil, err
	}
	run := *m
	run.ChatStore, run.DMStore, run.ModerationStore, run.AddressStore = chat, dm, moderation, addresses
	run.NotificationStore, run.AddressDirectoryStore, run.PushStore = notices, directory, push
	run.subjects, run.standings = subjects, standings
	run.runID = runID
	run.push = nil
	return &run, nil
}

func (s *ChatStore) inRun(runID string) (*ChatStore, error) {
	if runID == "" {
		return s, nil
	}
	st, err := s.store.ForRun(runID)
	if err != nil {
		return nil, err
	}
	run := *s
	run.store = st
	return &run, nil
}

func (s *DMStore) inRun(runID string) (*DMStore, error) {
	if runID == "" {
		return s, nil
	}
	st, err := s.store.ForRun(runID)
	if err != nil {
		return nil, err
	}
	run := *s
	run.store = st
	return &run, nil
}

func (s *ModerationStore) inRun(runID string) (*ModerationStore, error) {
	if runID == "" {
		return s, nil
	}
	st, err := s.store.ForRun(runID)
	if err != nil {
		return nil, err
	}
	run := *s
	run.store = st
	run.actWriter = discardActs{}
	return &run, nil
}

func (s *AddressStore) inRun(runID string) (*AddressStore, error) {
	if runID == "" {
		return s, nil
	}
	st, err := s.store.ForRun(runID)
	if err != nil {
		return nil, err
	}
	run := *s
	run.store = st
	return &run, nil
}

func (s *AddressDirectoryStore) inRun(runID string) (*AddressDirectoryStore, error) {
	if runID == "" {
		return s, nil
	}
	st, err := s.store.ForRun(runID)
	if err != nil {
		return nil, err
	}
	run := *s
	run.store = st
	return &run, nil
}

func (s *NotificationStore) inRun(runID string) (*NotificationStore, error) {
	if runID == "" {
		return s, nil
	}
	st, err := s.store.ForRun(runID)
	if err != nil {
		return nil, err
	}
	run := *s
	run.store = st
	run.onRaised = nil
	return &run, nil
}

func (s *PushStore) inRun(runID string) (*PushStore, error) {
	if runID == "" {
		return s, nil
	}
	st, err := s.store.ForRun(runID)
	if err != nil {
		return nil, err
	}
	run := *s
	run.store = st
	return &run, nil
}

func (s *SubjectStore) inRun(runID string) (*SubjectStore, error) {
	if runID == "" {
		return s, nil
	}
	st, err := s.store.ForRun(runID)
	if err != nil {
		return nil, err
	}
	run := *s
	run.store = st
	return &run, nil
}

func (s *StandingStore) inRun(runID string) (*StandingStore, error) {
	if runID == "" {
		return s, nil
	}
	st, err := s.store.ForRun(runID)
	if err != nil {
		return nil, err
	}
	chat, err := s.chat.inRun(runID)
	if err != nil {
		return nil, err
	}
	run := *s
	run.store = st
	run.chat = chat
	return &run, nil
}

func (m *CommManager) chatOn(tx *gorm.DB) (*ChatStore, error) {
	chat, err := NewChatStore(tx, m.spec, m.ChatStore.clock)
	if err != nil {
		return nil, err
	}
	return chat.inRun(m.runID)
}

func (m *CommManager) noticesOn(tx *gorm.DB) (*NotificationStore, error) {
	notices, err := NewNotificationStore(tx, m.spec, m.NotificationStore.clock)
	if err != nil {
		return nil, err
	}
	return notices.inRun(m.runID)
}

func (m *CommManager) subjectsOn(tx *gorm.DB) (*SubjectStore, error) {
	subjects, err := NewSubjectStore(tx, m.spec, m.subjects.clock)
	if err != nil {
		return nil, err
	}
	return subjects.inRun(m.runID)
}
