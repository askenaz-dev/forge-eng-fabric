package application

import (
	"context"
	"sync"
	"time"
)

// EventType is the CloudEvents `type` for App lifecycle events.
type EventType string

const (
	EventAppCreated      EventType = "app.created.v1"
	EventAppUpdated      EventType = "app.updated.v1"
	EventAppArchived     EventType = "app.archived.v1"
	EventAppRestored     EventType = "app.restored.v1"
	EventAppDeleted      EventType = "app.deleted.v1"
	EventSpecReparented  EventType = "spec.reparented.v1"
)

// Event is the payload shipped to the platform event bus. We carry both the
// `before` and `after` App bodies on update events so subscribers do not have
// to issue an extra lookup (per spec scenario "Update event carries diff").
type Event struct {
	Type          EventType `json:"type"`
	AppID         string    `json:"app_id"`
	WorkspaceID   string    `json:"workspace_id"`
	TenantID      string    `json:"tenant_id"`
	Actor         string    `json:"actor"`
	CorrelationID string    `json:"correlation_id"`
	Before        *App      `json:"before,omitempty"`
	After         *App      `json:"after,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// EventSink is the seam between this service and the platform event bus. The
// production wiring is an outbox-table publisher; tests use MemorySink.
type EventSink interface {
	Publish(ctx context.Context, event Event) error
}

// MemorySink records every published event and exposes them for assertions.
type MemorySink struct {
	mu     sync.Mutex
	events []Event
}

func NewMemorySink() *MemorySink { return &MemorySink{} }

func (m *MemorySink) Publish(_ context.Context, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	m.events = append(m.events, event)
	return nil
}

// Events returns a copy of all published events in publication order.
func (m *MemorySink) Events() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, len(m.events))
	copy(out, m.events)
	return out
}

// AuditRecord mirrors the row written to application_audit on every state
// transition. The service emits one record per mutating handler.
type AuditRecord struct {
	AppID         string         `json:"app_id"`
	WorkspaceID   string         `json:"workspace_id"`
	TenantID      string         `json:"tenant_id"`
	Action        string         `json:"action"`
	Actor         string         `json:"actor"`
	CorrelationID string         `json:"correlation_id"`
	Reason        string         `json:"reason,omitempty"`
	Before        *App           `json:"before,omitempty"`
	After         *App           `json:"after,omitempty"`
	Evidence      map[string]any `json:"evidence,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

// AuditSink is the seam to the application_audit partitioned table.
type AuditSink interface {
	Record(ctx context.Context, rec AuditRecord) error
}

type MemoryAuditSink struct {
	mu      sync.Mutex
	records []AuditRecord
}

func NewMemoryAuditSink() *MemoryAuditSink { return &MemoryAuditSink{} }

func (m *MemoryAuditSink) Record(_ context.Context, rec AuditRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	m.records = append(m.records, rec)
	return nil
}

func (m *MemoryAuditSink) Records() []AuditRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]AuditRecord, len(m.records))
	copy(out, m.records)
	return out
}
