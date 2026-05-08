package sdlc

import (
	"sort"
	"sync"
	"time"
)

type Store struct {
	mu          sync.RWMutex
	initiatives map[string]*Initiative
}

func NewStore() *Store {
	return &Store{initiatives: map[string]*Initiative{}}
}

func (s *Store) Insert(initiative *Initiative) *Initiative {
	s.mu.Lock()
	defer s.mu.Unlock()
	if initiative.ID == "" {
		initiative.ID = newID()
	}
	now := time.Now().UTC()
	if initiative.CreatedAt.IsZero() {
		initiative.CreatedAt = now
	}
	initiative.UpdatedAt = now
	s.initiatives[initiative.ID] = cloneInitiative(initiative)
	return cloneInitiative(initiative)
}

func (s *Store) Get(id string) (*Initiative, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	initiative, ok := s.initiatives[id]
	if !ok {
		return nil, false
	}
	return cloneInitiative(initiative), true
}

func (s *Store) Update(initiative *Initiative) *Initiative {
	s.mu.Lock()
	defer s.mu.Unlock()
	initiative.UpdatedAt = time.Now().UTC()
	s.initiatives[initiative.ID] = cloneInitiative(initiative)
	return cloneInitiative(initiative)
}

func (s *Store) List(workspaceID string) []*Initiative {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Initiative, 0, len(s.initiatives))
	for _, initiative := range s.initiatives {
		if workspaceID != "" && initiative.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, cloneInitiative(initiative))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func cloneInitiative(in *Initiative) *Initiative {
	if in == nil {
		return nil
	}
	out := *in
	out.PhaseStates = make([]PhaseState, len(in.PhaseStates))
	for i, state := range in.PhaseStates {
		out.PhaseStates[i] = clonePhaseState(state)
	}
	return &out
}

func clonePhaseState(in PhaseState) PhaseState {
	out := in
	out.Gates = append([]GateResult(nil), in.Gates...)
	out.Blockers = append([]Blocker(nil), in.Blockers...)
	return out
}

func phaseState(initiative *Initiative, phase Phase) *PhaseState {
	for i := range initiative.PhaseStates {
		if initiative.PhaseStates[i].Phase == phase {
			return &initiative.PhaseStates[i]
		}
	}
	return nil
}
