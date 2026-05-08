// Package onboarding implements the orchestration of app onboarding requests
// per the `app-onboarding-service` spec delta.
package onboarding

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending         Status = "pending"
	StatusPendingApproval Status = "pending_approval"
	StatusRunning         Status = "running"
	StatusCompleted       Status = "completed"
	StatusFailed          Status = "failed"
)

type Request struct {
	ID                 string         `json:"id"`
	WorkspaceID        string         `json:"workspace_id"`
	TenantID           string         `json:"tenant_id"`
	RepoOrg            string         `json:"repo_org"`
	RepoName           string         `json:"repo_name"`
	TemplateID         string         `json:"template_id"`
	TemplateVersion    string         `json:"template_version"`
	Parameters         map[string]any `json:"parameters"`
	Criticality        string         `json:"criticality"`
	DataClassification string         `json:"data_classification"`
	Owners             []string       `json:"owners"`
	Status             Status         `json:"status"`
	StatusReason       string         `json:"status_reason,omitempty"`
	AssetID            string         `json:"asset_id,omitempty"`
	CorrelationID      string         `json:"correlation_id"`
	RequestedBy        string         `json:"requested_by"`
	CreatedAt          time.Time      `json:"created_at"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
}

type RequestFilter struct {
	WorkspaceID string
	Status      Status
}

type TemplateParameter struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type TemplateSummary struct {
	ID                   string                       `json:"id"`
	Version              string                       `json:"version"`
	Description          string                       `json:"description,omitempty"`
	Category             string                       `json:"category,omitempty"`
	LifecycleState       string                       `json:"lifecycle_state"`
	TrustLevel           string                       `json:"trust_level"`
	Parameters           map[string]TemplateParameter `json:"parameters,omitempty"`
	RequiredCapabilities []string                     `json:"required_capabilities,omitempty"`
}

type PipelineGateResult struct {
	WorkspaceID    string         `json:"workspace_id"`
	RepoFullName   string         `json:"repo_full_name"`
	PRNumber       int            `json:"pr_number,omitempty"`
	CommitSHA      string         `json:"commit_sha"`
	Stage          string         `json:"stage"`
	Tool           string         `json:"tool"`
	Outcome        string         `json:"outcome"`
	SeverityCounts map[string]any `json:"severity_counts,omitempty"`
	ReportURL      string         `json:"report_url,omitempty"`
	PolicyVersion  string         `json:"policy_version,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type GateResultFilter struct {
	WorkspaceID  string
	RepoFullName string
	PRNumber     int
}

type PROpenSpecLink struct {
	WorkspaceID  string
	RepoFullName string
	PRNumber     int
	Status       string
}

type ImageSignature struct {
	WorkspaceID         string
	AssetID             string
	SignatureVerified   bool
	AttestationVerified bool
}

type Outcome string

const (
	OutcomeStarted   Outcome = "started"
	OutcomeCompleted Outcome = "completed"
	OutcomeFailed    Outcome = "failed"
	OutcomeWarn      Outcome = "warn"
)

type Event struct {
	ID         string         `json:"id"`
	RequestID  string         `json:"request_id"`
	Stage      string         `json:"stage"`
	Outcome    Outcome        `json:"outcome"`
	Message    string         `json:"message,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

func newID() string { return uuid.NewString() }

// Store is an in-memory persistence backend that mirrors the schema in
// `db/migrations/app-onboarding/0001_init.sql`. It exists so tests can run
// without Postgres; production wires the same interface to pgx.
type Store struct {
	mu              sync.RWMutex
	requests        map[string]*Request
	byKey           map[string]string // (workspace_id, repo_name) -> request_id
	events          map[string][]Event
	listeners       map[string][]chan Event
	gateResults     []PipelineGateResult
	prLinks         []PROpenSpecLink
	imageSignatures []ImageSignature
	overrideCount   int
}

func NewStore() *Store {
	return &Store{
		requests:  map[string]*Request{},
		byKey:     map[string]string{},
		events:    map[string][]Event{},
		listeners: map[string][]chan Event{},
	}
}

func (s *Store) List(filter RequestFilter) []*Request {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Request, 0, len(s.requests))
	for _, r := range s.requests {
		if filter.WorkspaceID != "" && r.WorkspaceID != filter.WorkspaceID {
			continue
		}
		if filter.Status != "" && r.Status != filter.Status {
			continue
		}
		copy := *r
		out = append(out, &copy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func key(workspaceID, repoName string) string { return workspaceID + "/" + repoName }

func (s *Store) Insert(r *Request) (*Request, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.byKey[key(r.WorkspaceID, r.RepoName)]; ok {
		// Idempotent: return existing instead of creating a duplicate.
		return s.requests[existing], false, nil
	}
	if r.ID == "" {
		r.ID = newID()
	}
	if r.CorrelationID == "" {
		r.CorrelationID = newID()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	if r.Status == "" {
		r.Status = StatusPending
	}
	s.requests[r.ID] = r
	s.byKey[key(r.WorkspaceID, r.RepoName)] = r.ID
	return r, true, nil
}

func (s *Store) Get(id string) (*Request, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.requests[id]
	return r, ok
}

func (s *Store) UpdateStatus(id string, status Status, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return
	}
	r.Status = status
	r.StatusReason = reason
	if status == StatusCompleted || status == StatusFailed {
		now := time.Now().UTC()
		r.CompletedAt = &now
	}
}

func (s *Store) SetAsset(id, assetID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.requests[id]; ok {
		r.AssetID = assetID
	}
}

func (s *Store) AppendEvent(requestID, stage string, outcome Outcome, message string, payload map[string]any, duration time.Duration) Event {
	ev := Event{
		ID:         newID(),
		RequestID:  requestID,
		Stage:      stage,
		Outcome:    outcome,
		Message:    message,
		Payload:    payload,
		DurationMS: duration.Milliseconds(),
		CreatedAt:  time.Now().UTC(),
	}
	s.mu.Lock()
	s.events[requestID] = append(s.events[requestID], ev)
	listeners := append([]chan Event(nil), s.listeners[requestID]...)
	s.mu.Unlock()
	for _, ch := range listeners {
		select {
		case ch <- ev:
		default:
		}
	}
	return ev
}

func (s *Store) Events(requestID string) []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]Event(nil), s.events[requestID]...)
	return out
}

func (s *Store) RecordGateResult(result PipelineGateResult) {
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gateResults = append(s.gateResults, result)
}

func (s *Store) GateResults(filter GateResultFilter) []PipelineGateResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PipelineGateResult, 0, len(s.gateResults))
	for _, result := range s.gateResults {
		if filter.WorkspaceID != "" && result.WorkspaceID != filter.WorkspaceID {
			continue
		}
		if filter.RepoFullName != "" && result.RepoFullName != filter.RepoFullName {
			continue
		}
		if filter.PRNumber != 0 && result.PRNumber != filter.PRNumber {
			continue
		}
		out = append(out, result)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// Subscribe returns a buffered channel that receives every subsequent event
// for the given request, plus a cleanup function to call when done.
func (s *Store) Subscribe(requestID string) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	s.mu.Lock()
	s.listeners[requestID] = append(s.listeners[requestID], ch)
	s.mu.Unlock()
	cleanup := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		ls := s.listeners[requestID]
		for i, c := range ls {
			if c == ch {
				s.listeners[requestID] = append(ls[:i], ls[i+1:]...)
				break
			}
		}
		close(ch)
	}
	return ch, cleanup
}
