// Package detection normalizes alerts from external monitoring systems and
// internal CloudEvents into the canonical incident.detected.v1 stream.
package detection

import (
	"errors"
	"sync"
	"time"
)

// Source is where an alert came from.
type Source string

const (
	SourcePrometheus      Source = "prometheus"
	SourceCloudMonitoring Source = "cloud-monitoring"
	SourceLoki            Source = "loki"
	SourceManual          Source = "manual"
	SourceInternal        Source = "internal-event"
)

// Severity matches Prometheus + standard convention.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Status of the open incident.
type Status string

const (
	StatusOpen     Status = "open"
	StatusResolved Status = "resolved"
)

// Incident is the normalized record persisted by the detection service.
type Incident struct {
	ID            string            `json:"id"`
	TenantID      string            `json:"tenant_id"`
	WorkspaceID   string            `json:"workspace_id"`
	Service       string            `json:"service"`
	Environment   string            `json:"environment"`
	SignatureHash string            `json:"signature_hash"`
	Source        Source            `json:"source"`
	Severity      Severity          `json:"severity"`
	Title         string            `json:"title"`
	Description   string            `json:"description,omitempty"`
	Status        Status            `json:"status"`
	OpenedAt      time.Time         `json:"opened_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	ResolvedAt    *time.Time        `json:"resolved_at,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Synthetic     bool              `json:"synthetic"`
	Events        []Event           `json:"events,omitempty"`
}

// Event is a single occurrence appended to an open incident.
type Event struct {
	ID        string            `json:"id"`
	IncidentID string           `json:"incident_id"`
	Source    Source            `json:"source"`
	Severity  Severity          `json:"severity"`
	OccurredAt time.Time        `json:"occurred_at"`
	Payload   map[string]any    `json:"payload,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// DeclareRequest is the manual-declare payload.
type DeclareRequest struct {
	TenantID    string            `json:"tenant_id"`
	WorkspaceID string            `json:"workspace_id"`
	Service     string            `json:"service"`
	Environment string            `json:"environment"`
	Severity    Severity          `json:"severity"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Actor       string            `json:"actor"`
	Labels      map[string]string `json:"labels,omitempty"`
	Synthetic   bool              `json:"synthetic,omitempty"`
}

// DedupWindow is the deduplication window for matched incidents.
const DedupWindow = 5 * time.Minute

// Errors.
var (
	ErrInvalidPayload = errors.New("invalid_payload")
	ErrUnknownSource  = errors.New("unknown_source")
)

// Store keeps incidents in memory keyed by (service, env, signature_hash).
type Store struct {
	mu        sync.RWMutex
	incidents map[string]*Incident // key = composite key
	byID      map[string]*Incident
}

// NewStore creates an empty store.
func NewStore() *Store {
	return &Store{
		incidents: map[string]*Incident{},
		byID:      map[string]*Incident{},
	}
}

// Get returns an incident by id or nil.
func (s *Store) Get(id string) *Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byID[id]
}

// List returns a snapshot of incidents (newest first).
func (s *Store) List(filter func(*Incident) bool) []*Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Incident, 0, len(s.byID))
	for _, inc := range s.byID {
		if filter == nil || filter(inc) {
			out = append(out, inc)
		}
	}
	// Newest open first.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].OpenedAt.After(out[i].OpenedAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func compositeKey(service, env, sig string) string {
	return service + "|" + env + "|" + sig
}

// upsert dedups by (service, env, signature_hash) within DedupWindow.
// Returns the incident, plus a bool that is true if a new incident was created.
func (s *Store) upsert(inc *Incident, ev Event, now time.Time) (*Incident, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := compositeKey(inc.Service, inc.Environment, inc.SignatureHash)
	if existing, ok := s.incidents[key]; ok && existing.Status == StatusOpen {
		if now.Sub(existing.UpdatedAt) <= DedupWindow {
			ev.IncidentID = existing.ID
			existing.Events = append(existing.Events, ev)
			existing.UpdatedAt = now
			if inc.Severity == SeverityCritical {
				existing.Severity = SeverityCritical
			}
			return existing, false
		}
	}
	inc.Status = StatusOpen
	inc.OpenedAt = now
	inc.UpdatedAt = now
	ev.IncidentID = inc.ID
	inc.Events = []Event{ev}
	s.incidents[key] = inc
	s.byID[inc.ID] = inc
	return inc, true
}

// Resolve marks an incident as resolved.
func (s *Store) Resolve(id string, when time.Time) (*Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inc, ok := s.byID[id]
	if !ok {
		return nil, errors.New("incident_not_found")
	}
	inc.Status = StatusResolved
	inc.ResolvedAt = &when
	inc.UpdatedAt = when
	return inc, nil
}
