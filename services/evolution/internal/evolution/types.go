// Package evolution converts postmortem learnings into OpenSpec change
// proposals (Phase 6 evolution loop).
package evolution

import (
	"sync"
	"time"
)

// SkillVersion is the prompt skill version used to derive proposals.
const SkillVersion = "derive-openspec-evolution@1.0.0"

// AutonomousLoopMarker identifies proposals that came from this loop.
const AutonomousLoopMarker = "autonomous-loop"

// ProposalStatus enumerates lifecycle states.
type ProposalStatus string

const (
	StatusDraft     ProposalStatus = "draft"
	StatusInbox     ProposalStatus = "inbox"
	StatusAccepted  ProposalStatus = "accepted"
	StatusRejected  ProposalStatus = "rejected"
	StatusConverted ProposalStatus = "converted"
)

// ProposalKind enumerates the kind of suggestion.
type ProposalKind string

const (
	KindAcceptanceCriteria ProposalKind = "acceptance_criteria"
	KindRunbookUpdate      ProposalKind = "runbook_update"
	KindSLOAdjustment      ProposalKind = "slo_adjustment"
	KindNewGate            ProposalKind = "new_gate"
	KindNewHealingAction   ProposalKind = "new_healing_action"
)

// Suggestion is one item the proposal asks for.
type Suggestion struct {
	Kind        ProposalKind `json:"kind"`
	Title       string       `json:"title"`
	Detail      string       `json:"detail"`
	OpenSpecRef string       `json:"openspec_ref,omitempty"`
}

// PostmortemInput is the bundle the evolution loop consumes.
type PostmortemInput struct {
	IncidentID      string   `json:"incident_id"`
	TenantID        string   `json:"tenant_id"`
	WorkspaceID     string   `json:"workspace_id"`
	AssetID         string   `json:"asset_id"`
	Service         string   `json:"service"`
	Environment     string   `json:"environment"`
	Severity        string   `json:"severity"`
	Summary         string   `json:"summary"`
	RootCause       string   `json:"root_cause"`
	Lessons         []string `json:"lessons"`
	HealingActions  []string `json:"healing_actions"`
	PostmortemURL   string   `json:"postmortem_url"`
	Synthetic       bool     `json:"synthetic"`
}

// Proposal is the persistent record.
type Proposal struct {
	ID            string         `json:"id"`
	IncidentID    string         `json:"incident_id"`
	TenantID      string         `json:"tenant_id"`
	WorkspaceID   string         `json:"workspace_id"`
	AssetID       string         `json:"asset_id"`
	PostmortemURL string         `json:"postmortem_url"`
	Source        string         `json:"source"` // always "autonomous-loop"
	SkillVersion  string         `json:"skill_version"`
	Status        ProposalStatus `json:"status"`
	Title         string         `json:"title"`
	Why           string         `json:"why"`
	Suggestions   []Suggestion   `json:"suggestions"`
	OpenSpecChange string        `json:"openspec_change_id,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	Synthetic     bool           `json:"synthetic"`
}

// ReviewDecision is the outcome of a human review.
type ReviewDecision struct {
	Approved bool   `json:"approved"`
	Reviewer string `json:"reviewer"`
	Comment  string `json:"comment"`
}

// Store keeps proposals in memory.
type Store struct {
	mu        sync.RWMutex
	proposals map[string]*Proposal
}

// NewStore creates an empty store.
func NewStore() *Store {
	return &Store{proposals: map[string]*Proposal{}}
}

// Save inserts or updates.
func (s *Store) Save(p *Proposal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.UpdatedAt = time.Now().UTC()
	s.proposals[p.ID] = p
}

// Get returns a proposal by id.
func (s *Store) Get(id string) *Proposal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.proposals[id]
}

// List returns proposals optionally filtered by status / tenant.
func (s *Store) List(status ProposalStatus, tenantID string) []*Proposal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*Proposal{}
	for _, p := range s.proposals {
		if status != "" && p.Status != status {
			continue
		}
		if tenantID != "" && p.TenantID != tenantID {
			continue
		}
		out = append(out, p)
	}
	return out
}

// Stats summarises counts for the metrics endpoint (task 11.3).
func (s *Store) Stats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]int{"total": len(s.proposals)}
	for _, p := range s.proposals {
		out[string(p.Status)]++
	}
	return out
}
