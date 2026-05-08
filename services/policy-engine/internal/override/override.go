// Package override implements policy override grants with TTL and an
// auto-revert reconciler, as required by `policies-and-approvals` spec
// (D2.7) and the override sections in `ci-pipeline-baseline`,
// `pr-openspec-linking`, and `github-app-provisioning`.
package override

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Template struct {
	ID              string   `yaml:"id"`
	Description     string   `yaml:"description"`
	Action          string   `yaml:"action"`
	Target          string   `yaml:"target"`
	RequiredRole    string   `yaml:"required_role"`
	MaxTTLSeconds   int      `yaml:"max_ttl_seconds"`
	RequiresReason  bool     `yaml:"requires_reason"`
	EventsOnGrant   []string `yaml:"events_on_grant"`
	EventsOnConsume []string `yaml:"events_on_consume"`
	EventsOnExpire  []string `yaml:"events_on_expire"`
}

type TemplateDoc struct {
	Templates []Template `yaml:"templates"`
}

func LoadTemplates(data []byte) (map[string]Template, error) {
	var doc TemplateDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	out := map[string]Template{}
	for _, t := range doc.Templates {
		out[t.ID] = t
	}
	return out, nil
}

type State string

const (
	StateActive   State = "active"
	StateConsumed State = "consumed"
	StateExpired  State = "expired"
	StateRevoked  State = "revoked"
)

type Override struct {
	ID            string         `json:"id"`
	TemplateID    string         `json:"template_id"`
	WorkspaceID   string         `json:"workspace_id"`
	Subject       string         `json:"subject"`
	RequestedBy   string         `json:"requested_by"`
	ApprovedBy    string         `json:"approved_by"`
	Reason        string         `json:"reason"`
	GrantedAt     time.Time      `json:"granted_at"`
	ExpiresAt     time.Time      `json:"expires_at"`
	State         State          `json:"state"`
	ConsumedAt    *time.Time     `json:"consumed_at,omitempty"`
	ConsumedBy    string         `json:"consumed_by,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type Event struct {
	Type      string         `json:"type"`
	Override  *Override      `json:"override"`
	Time      time.Time      `json:"time"`
	Extra     map[string]any `json:"extra,omitempty"`
}

type Sink interface {
	Emit(Event)
}

type MemorySink struct {
	mu     sync.Mutex
	Events []Event
}

func (m *MemorySink) Emit(e Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Events = append(m.Events, e)
}

type Manager struct {
	mu        sync.RWMutex
	templates map[string]Template
	overrides map[string]*Override
	sink      Sink
	now       func() time.Time
}

func NewManager(templates map[string]Template, sink Sink) *Manager {
	if sink == nil {
		sink = &MemorySink{}
	}
	return &Manager{templates: templates, overrides: map[string]*Override{}, sink: sink, now: time.Now}
}

func (m *Manager) Templates() map[string]Template {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := map[string]Template{}
	for k, v := range m.templates {
		out[k] = v
	}
	return out
}

type GrantInput struct {
	TemplateID  string
	WorkspaceID string
	Subject     string
	RequestedBy string
	ApprovedBy  string
	ApproverRole string
	Reason      string
	TTLSeconds  int
	Metadata    map[string]any
}

var (
	ErrUnknownTemplate    = errors.New("unknown template")
	ErrInsufficientRole   = errors.New("insufficient_role")
	ErrTTLExceedsMax      = errors.New("ttl_exceeds_max")
	ErrReasonRequired     = errors.New("reason_required")
	ErrSubjectRequired    = errors.New("subject_required")
	ErrAlreadyTerminated  = errors.New("override_already_terminated")
)

func (m *Manager) Grant(in GrantInput) (*Override, error) {
	m.mu.Lock()
	tpl, ok := m.templates[in.TemplateID]
	m.mu.Unlock()
	if !ok {
		return nil, ErrUnknownTemplate
	}
	if in.ApproverRole != tpl.RequiredRole {
		return nil, ErrInsufficientRole
	}
	if in.TTLSeconds <= 0 || in.TTLSeconds > tpl.MaxTTLSeconds {
		return nil, ErrTTLExceedsMax
	}
	if tpl.RequiresReason && in.Reason == "" {
		return nil, ErrReasonRequired
	}
	if in.Subject == "" {
		return nil, ErrSubjectRequired
	}
	now := m.now().UTC()
	ov := &Override{
		ID:          uuid.NewString(),
		TemplateID:  in.TemplateID,
		WorkspaceID: in.WorkspaceID,
		Subject:     in.Subject,
		RequestedBy: in.RequestedBy,
		ApprovedBy:  in.ApprovedBy,
		Reason:      in.Reason,
		GrantedAt:   now,
		ExpiresAt:   now.Add(time.Duration(in.TTLSeconds) * time.Second),
		State:       StateActive,
		Metadata:    in.Metadata,
	}
	m.mu.Lock()
	m.overrides[ov.ID] = ov
	m.mu.Unlock()
	for _, t := range tpl.EventsOnGrant {
		m.sink.Emit(Event{Type: t, Override: ov, Time: now})
	}
	return ov, nil
}

// Consume marks an override as used (one-shot for templates that support it).
func (m *Manager) Consume(id, by string) error {
	m.mu.Lock()
	ov, ok := m.overrides[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("override not found")
	}
	if ov.State != StateActive {
		m.mu.Unlock()
		return ErrAlreadyTerminated
	}
	tpl, _ := m.templates[ov.TemplateID]
	now := m.now().UTC()
	ov.State = StateConsumed
	ov.ConsumedAt = &now
	ov.ConsumedBy = by
	m.mu.Unlock()
	for _, t := range tpl.EventsOnConsume {
		m.sink.Emit(Event{Type: t, Override: ov, Time: now, Extra: map[string]any{"consumed_by": by}})
	}
	return nil
}

// Revoke retires an override before TTL.
func (m *Manager) Revoke(id, by, reason string) error {
	m.mu.Lock()
	ov, ok := m.overrides[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("override not found")
	}
	if ov.State != StateActive {
		m.mu.Unlock()
		return ErrAlreadyTerminated
	}
	ov.State = StateRevoked
	tpl := m.templates[ov.TemplateID]
	m.mu.Unlock()
	now := m.now().UTC()
	m.sink.Emit(Event{Type: "policy.override.revoked.v1", Override: ov, Time: now, Extra: map[string]any{"by": by, "reason": reason}})
	_ = tpl
	return nil
}

// IsActive returns true iff an override matching subject and template exists,
// is active, and has not expired. Used at evaluation time by callers like
// the GitHub MCP guardrails or the openspec-link check.
func (m *Manager) IsActive(templateID, subject string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := m.now().UTC()
	for _, ov := range m.overrides {
		if ov.TemplateID == templateID && ov.Subject == subject && ov.State == StateActive && ov.ExpiresAt.After(now) {
			return true
		}
	}
	return false
}

// ReconcileExpired marks active overrides whose TTL has elapsed as expired
// and emits the expiration events. Returns the IDs that were transitioned.
func (m *Manager) ReconcileExpired() []string {
	m.mu.Lock()
	expired := []*Override{}
	now := m.now().UTC()
	for _, ov := range m.overrides {
		if ov.State == StateActive && !ov.ExpiresAt.After(now) {
			ov.State = StateExpired
			expired = append(expired, ov)
		}
	}
	tpls := m.templates
	m.mu.Unlock()
	ids := make([]string, 0, len(expired))
	for _, ov := range expired {
		tpl := tpls[ov.TemplateID]
		for _, t := range tpl.EventsOnExpire {
			m.sink.Emit(Event{Type: t, Override: ov, Time: now})
		}
		ids = append(ids, ov.ID)
	}
	return ids
}

// List returns active overrides sorted by ExpiresAt ascending.
func (m *Manager) List() []*Override {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Override, 0, len(m.overrides))
	for _, ov := range m.overrides {
		out = append(out, ov)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExpiresAt.Before(out[j].ExpiresAt) })
	return out
}

// SetClock overrides time.Now (test-only).
func (m *Manager) SetClock(now func() time.Time) { m.now = now }

// RunReconciler kicks off a goroutine that calls ReconcileExpired every
// `interval`. Returns a stop function. Production wires this from main.
func (m *Manager) RunReconciler(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.ReconcileExpired()
		case <-stop:
			return
		}
	}
}
