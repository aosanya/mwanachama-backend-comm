package mwanachamacomm

import (
	"context"
	"sort"
	"sync"
	"time"
)

// ModerationMemoryStore is the in-memory implementation of
// ModerationRepository.
type ModerationMemoryStore struct {
	mu         sync.RWMutex
	reports    map[string]Report
	removals   map[string]Removal
	dismissals map[string]Dismissal
	disputes   map[string]Dispute
	ids        *IDGen
	// actWriter is required, not an optional setter: a store built without
	// one withholds posts silently. See moderation_act.go.
	actWriter MemoryActWriter
	clock     Clock
}

// NewModerationMemoryStore constructs an empty store.
func NewModerationMemoryStore(ids *IDGen, clock Clock, actWriter MemoryActWriter) *ModerationMemoryStore {
	return &ModerationMemoryStore{
		reports:    map[string]Report{},
		removals:   map[string]Removal{},
		dismissals: map[string]Dismissal{},
		disputes:   map[string]Dispute{},
		ids:        ids,
		actWriter:  actWriter,
		clock:      clock,
	}
}

// FileReport inserts a report. One per (MessageID, ReportedBy), matching
// message-report.md's unique(message_id, reported_by).
func (s *ModerationMemoryStore) FileReport(_ context.Context, r Report) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.reports {
		if existing.MessageID == r.MessageID && existing.ReportedBy == r.ReportedBy {
			return Report{}, Conflict("comm_message_report_message_id_reported_by_key")
		}
	}
	if r.ID == "" {
		r.ID = s.ids.New("modreport")
	}
	if r.ReportedAt.IsZero() {
		r.ReportedAt = s.clock()
	}
	s.reports[r.ID] = r
	return r, nil
}

// ListReportsForMessage returns every report against one message, newest
// first.
func (s *ModerationMemoryStore) ListReportsForMessage(_ context.Context, messageID string) ([]Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Report{}
	for _, r := range s.reports {
		if r.MessageID == messageID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return newestFirst(out[i].ReportedAt, out[j].ReportedAt, out[i].ID, out[j].ID) })
	return out, nil
}

// ListReportQueue returns every report filed against a message in
// chapterID, newest first.
func (s *ModerationMemoryStore) ListReportQueue(_ context.Context, chapterID string) ([]Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Report{}
	for _, r := range s.reports {
		if r.ChapterID == chapterID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return newestFirst(out[i].ReportedAt, out[j].ReportedAt, out[i].ID, out[j].ID) })
	return out, nil
}

// CreateRemoval inserts a removal and writes the chapter's act-log row with
// it — DEV-1341.
//
// The conflict returns before either write, so a second removal of an
// already-withheld post writes no second log row. If the append fails the
// removal is deleted again under the same lock — this backend's stand-in for
// a ROLLBACK.
func (s *ModerationMemoryStore) CreateRemoval(ctx context.Context, rem Removal, wall string, actor Actor) (Removal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.removals {
		if existing.MessageID == rem.MessageID {
			return Removal{}, Conflict("comm_message_removal_message_id_key")
		}
	}
	if rem.ID == "" {
		rem.ID = s.ids.New("modremoval")
	}
	if rem.RemovedAt.IsZero() {
		rem.RemovedAt = s.clock()
	}
	s.removals[rem.ID] = rem
	if err := s.actWriter.WriteAct(ctx, WithheldAct(rem, wall, actor)); err != nil {
		delete(s.removals, rem.ID)
		return Removal{}, err
	}
	return rem, nil
}

// DismissReports records G79's third outcome and appends the chapter's
// `report_left_standing` act row — DEV-1351.
//
// The two refusals are the Postgres store's, in the same order and under one
// lock: already-withheld first (`Removed` and `Left standing` are two states
// of one post), then one-dismissal-per-message. Both return before the log
// is appended, so the log holds exactly the dismissals that happened.
func (s *ModerationMemoryStore) DismissReports(ctx context.Context, d Dismissal, wall string, actor Actor) (Dismissal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.removals {
		if existing.MessageID == d.MessageID {
			return Dismissal{}, ErrAlreadyRemoved
		}
	}
	for _, existing := range s.dismissals {
		if existing.MessageID == d.MessageID {
			return Dismissal{}, Conflict("comm_message_report_dismissal_message_id_key")
		}
	}
	if d.ID == "" {
		d.ID = s.ids.New("moddismissal")
	}
	if d.DismissedAt.IsZero() {
		d.DismissedAt = s.clock()
	}
	s.dismissals[d.ID] = d
	if err := s.actWriter.WriteAct(ctx, ReportLeftStandingAct(d, wall, actor)); err != nil {
		delete(s.dismissals, d.ID)
		return Dismissal{}, err
	}
	return d, nil
}

// GetDismissalForMessage returns the dismissal naming messageID, if any.
func (s *ModerationMemoryStore) GetDismissalForMessage(_ context.Context, messageID string) (Dismissal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.dismissals {
		if d.MessageID == messageID {
			return d, nil
		}
	}
	return Dismissal{}, ErrModerationNotFound
}

// GetRemoval returns one removal by id.
func (s *ModerationMemoryStore) GetRemoval(_ context.Context, id string) (Removal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rem, ok := s.removals[id]
	if !ok {
		return Removal{}, ErrModerationNotFound
	}
	return rem, nil
}

// GetRemovalForMessage returns the removal naming messageID, if any.
func (s *ModerationMemoryStore) GetRemovalForMessage(_ context.Context, messageID string) (Removal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rem := range s.removals {
		if rem.MessageID == messageID {
			return rem, nil
		}
	}
	return Removal{}, ErrModerationNotFound
}

// ListRemovalsForChapter returns every removal at chapterID, newest first.
func (s *ModerationMemoryStore) ListRemovalsForChapter(_ context.Context, chapterID string) ([]Removal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Removal{}
	for _, rem := range s.removals {
		if rem.ChapterID == chapterID {
			out = append(out, rem)
		}
	}
	sort.Slice(out, func(i, j int) bool { return newestFirst(out[i].RemovedAt, out[j].RemovedAt, out[i].ID, out[j].ID) })
	return out, nil
}

// CreateDispute inserts a dispute in the Open state. One per RemovalID.
func (s *ModerationMemoryStore) CreateDispute(_ context.Context, d Dispute) (Dispute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.disputes {
		if existing.RemovalID == d.RemovalID {
			return Dispute{}, Conflict("comm_removal_dispute_removal_id_key")
		}
	}
	if _, ok := s.removals[d.RemovalID]; !ok {
		return Dispute{}, Reference("removal_id")
	}
	if d.ID == "" {
		d.ID = s.ids.New("moddispute")
	}
	now := s.clock()
	if d.RaisedAt.IsZero() {
		d.RaisedAt = now
	}
	if d.HeldSince.IsZero() {
		d.HeldSince = now
	}
	d.State = DisputeOpen
	d.DecidedBy = ""
	d.DecidedAt = nil
	s.disputes[d.ID] = d
	return d, nil
}

// GetDispute returns one dispute by id.
func (s *ModerationMemoryStore) GetDispute(_ context.Context, id string) (Dispute, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.disputes[id]
	if !ok {
		return Dispute{}, ErrModerationNotFound
	}
	return d, nil
}

// GetDisputeForRemoval returns the dispute naming removalID, if any.
func (s *ModerationMemoryStore) GetDisputeForRemoval(_ context.Context, removalID string) (Dispute, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.disputes {
		if d.RemovalID == removalID {
			return d, nil
		}
	}
	return Dispute{}, ErrModerationNotFound
}

// DecideDispute moves a dispute from Open to outcome, enforcing G62
// (reviewer != remover) and single-shot decision.
func (s *ModerationMemoryStore) DecideDispute(ctx context.Context, id string, outcome DisputeState, decidedBy string, now time.Time, wall string, actor Actor) (Dispute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.disputes[id]
	if !ok {
		return Dispute{}, ErrModerationNotFound
	}
	if d.State != DisputeOpen {
		return Dispute{}, ErrAlreadyDecided
	}
	rem, ok := s.removals[d.RemovalID]
	if !ok {
		return Dispute{}, Reference("removal_id")
	}
	if rem.RemovedBy == decidedBy {
		return Dispute{}, ErrReviewerIsRemover
	}
	d.State = outcome
	d.DecidedBy = decidedBy
	decidedAt := now
	d.DecidedAt = &decidedAt
	s.disputes[id] = d
	// DEV-1341 · the outcome's row. All three refusals above return first, so
	// only a decision that happened is logged, and re-deciding a decided
	// dispute is refused before it can write a second outcome.
	if err := s.actWriter.WriteAct(ctx, DisputeOutcomeAct(rem, d, wall, actor)); err != nil {
		d.State = DisputeOpen
		d.DecidedBy, d.DecidedAt = "", nil
		s.disputes[id] = d
		return Dispute{}, err
	}
	return d, nil
}

var _ ModerationRepository = (*ModerationMemoryStore)(nil)
