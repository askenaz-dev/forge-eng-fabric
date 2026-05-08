// Package engine implements the Phase 6 healing engine.
//
// The engine maps incidents to healing actions, decides the level (L1..L5)
// allowed by the matching envelope, and either executes (L4/L5), pauses for
// approval (L3), or just notifies/suggests (L1/L2). A global / per-Workspace
// kill switch can degrade everything to L1 instantly.
package engine

import (
	"errors"
	"sync"
	"time"
)

// Level represents the autonomy level for a healing action.
type Level string

const (
	LevelL1 Level = "L1"
	LevelL2 Level = "L2"
	LevelL3 Level = "L3"
	LevelL4 Level = "L4"
	LevelL5 Level = "L5"
)

// AllLevels lists the levels in ascending autonomy order.
var AllLevels = []Level{LevelL1, LevelL2, LevelL3, LevelL4, LevelL5}

// Outcome captures the result of an invocation.
type Outcome string

const (
	OutcomeNotified         Outcome = "notified"
	OutcomeSuggested        Outcome = "suggested"
	OutcomeWaitingApproval  Outcome = "waiting_approval"
	OutcomeExecuted         Outcome = "executed"
	OutcomeFailed           Outcome = "failed"
	OutcomeRolledBack       Outcome = "rolled_back"
	OutcomeEscalated        Outcome = "escalated"
	OutcomeSuppressed       Outcome = "suppressed"
	OutcomeDegradedByEnv    Outcome = "degraded_by_envelope"
	OutcomeBlockedByLimits  Outcome = "blocked_by_rate_limit"
)

// Envelope defines the autonomy boundary for a (capability, asset, env, criticality).
type Envelope struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	WorkspaceID       string    `json:"workspace_id,omitempty"`
	Capability        string    `json:"capability"`
	AssetPattern      string    `json:"asset_pattern"`
	Environment       string    `json:"environment"`
	Criticality       string    `json:"criticality"`
	DefaultLevel      Level     `json:"default_level"`
	AllowedLevels     []Level   `json:"allowed_levels"`
	TimeWindows       []string  `json:"time_windows,omitempty"`
	MaxActionsPerHour int       `json:"max_actions_per_hour"`
	KillSwitch        bool      `json:"kill_switch"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Action describes an entry in the healing action catalog.
type Action struct {
	ID                  string                 `json:"id"`
	Risk                string                 `json:"risk"`
	Reversible          bool                   `json:"reversible"`
	BlastRadius         string                 `json:"blast_radius"`
	AllowedLevelsByEnv  map[string][]Level     `json:"allowed_levels_by_env"`
	WorkflowRef         string                 `json:"workflow_ref"`
	RunbookURL          string                 `json:"runbook_url,omitempty"`
	EvalSuiteID         string                 `json:"eval_suite_id"`
	Parameters          map[string]any         `json:"parameters,omitempty"`
}

// PromotionRequest is the body of a request to promote an action's level.
type PromotionRequest struct {
	ActionID         string `json:"action_id"`
	Environment      string `json:"environment"`
	TargetLevel      Level  `json:"target_level"`
	RequestedBy      string `json:"requested_by"`
	PlatformAdminOK  bool   `json:"platform_admin_ok"`
	SecurityOK       bool   `json:"security_ok"`
}

// PromotionMetrics captures the prerequisites for promotion (D6.10).
type PromotionMetrics struct {
	EvalPassRateLast50         float64
	SuccessfulL3Runs           int
	DaysSinceLastPostmortem    int
}

// HealingDecision is what the engine produces for an incident.
type HealingDecision struct {
	ID            string  `json:"id"`
	IncidentID    string  `json:"incident_id"`
	ActionID      string  `json:"action_id"`
	Level         Level   `json:"level"`
	RequestedLevel Level  `json:"requested_level"`
	Outcome       Outcome `json:"outcome"`
	Reason        string  `json:"reason,omitempty"`
	WorkflowRunID string  `json:"workflow_run_id,omitempty"`
	ApprovalID    string  `json:"approval_id,omitempty"`
	Synthetic     bool    `json:"synthetic"`
	CreatedAt     time.Time `json:"created_at"`
}

// IncidentInput is the minimal incident shape fed into the engine.
type IncidentInput struct {
	IncidentID  string `json:"incident_id"`
	TenantID    string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id"`
	Service     string `json:"service"`
	Environment string `json:"environment"`
	Capability  string `json:"capability"`
	Criticality string `json:"criticality"`
	Synthetic   bool   `json:"synthetic"`
	SignatureHash string `json:"signature_hash"`
	SuggestedActions []string `json:"suggested_actions"`
}

// Errors.
var (
	ErrEnvelopeNotFound        = errors.New("envelope_not_found")
	ErrActionNotFound          = errors.New("action_not_found")
	ErrPromotionPrerequisites  = errors.New("promotion_prerequisites_unmet")
	ErrPromotionApproval       = errors.New("promotion_approval_missing")
	ErrInvalidLevel            = errors.New("invalid_level")
)

// Store keeps envelopes / actions / decisions / kill-switch state in memory.
type Store struct {
	mu              sync.RWMutex
	envelopes       map[string]*Envelope
	actions         map[string]*Action
	decisions       map[string]*HealingDecision
	killGlobal      bool
	killWorkspace   map[string]bool
	rateBuckets     map[string][]time.Time
	promotionStats  map[string]PromotionMetrics
}

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		envelopes:      map[string]*Envelope{},
		actions:        map[string]*Action{},
		decisions:      map[string]*HealingDecision{},
		killWorkspace:  map[string]bool{},
		rateBuckets:    map[string][]time.Time{},
		promotionStats: map[string]PromotionMetrics{},
	}
}

// SetEnvelope upserts an envelope.
func (s *Store) SetEnvelope(e *Envelope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = e.Capability + "/" + e.Environment + "/" + e.Criticality
	}
	e.UpdatedAt = time.Now().UTC()
	s.envelopes[e.ID] = e
}

// GetEnvelope returns the matching envelope or nil.
func (s *Store) GetEnvelope(capability, env, criticality string) *Envelope {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id := capability + "/" + env + "/" + criticality
	if e, ok := s.envelopes[id]; ok {
		return e
	}
	// Fallback: capability+env, no criticality match.
	for _, e := range s.envelopes {
		if e.Capability == capability && e.Environment == env {
			return e
		}
	}
	return nil
}

// ListEnvelopes returns a snapshot.
func (s *Store) ListEnvelopes() []*Envelope {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Envelope, 0, len(s.envelopes))
	for _, e := range s.envelopes {
		out = append(out, e)
	}
	return out
}

// SetAction registers a healing action.
func (s *Store) SetAction(a *Action) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions[a.ID] = a
}

// GetAction returns an action by id.
func (s *Store) GetAction(id string) *Action {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.actions[id]
}

// SaveDecision persists a decision.
func (s *Store) SaveDecision(d *HealingDecision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions[d.ID] = d
}

// ListDecisions returns all decisions for an incident.
func (s *Store) ListDecisions(incidentID string) []*HealingDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*HealingDecision{}
	for _, d := range s.decisions {
		if d.IncidentID == incidentID {
			out = append(out, d)
		}
	}
	return out
}

// SetKillSwitch toggles the kill switch (workspaceID="" means global).
func (s *Store) SetKillSwitch(workspaceID string, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workspaceID == "" {
		s.killGlobal = active
		return
	}
	s.killWorkspace[workspaceID] = active
}

// KillSwitch returns the active state for workspaceID (global wins).
func (s *Store) KillSwitch(workspaceID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.killGlobal {
		return true
	}
	return s.killWorkspace[workspaceID]
}

// trackInvocation appends a timestamp and prunes entries older than 1h.
// Returns the new count within the trailing hour.
func (s *Store) trackInvocation(envelopeID string, now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.rateBuckets[envelopeID]
	cutoff := now.Add(-time.Hour)
	out := bucket[:0]
	for _, t := range bucket {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	out = append(out, now)
	s.rateBuckets[envelopeID] = out
	return len(out)
}

// SetPromotionStats updates promotion-eligibility metrics for an action+env.
func (s *Store) SetPromotionStats(actionID, env string, m PromotionMetrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.promotionStats[actionID+"/"+env] = m
}

// GetPromotionStats returns the cached metrics for an action+env.
func (s *Store) GetPromotionStats(actionID, env string) PromotionMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.promotionStats[actionID+"/"+env]
}
