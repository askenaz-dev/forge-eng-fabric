package trigger

import (
	"time"

	"github.com/google/uuid"
)

// Event names emitted to the platform bus by trigger-router.
const (
	EventTriggerFired   = "workflow.trigger.fired.v1"
	EventTriggerDropped = "workflow.trigger.dropped.v1"
	EventTriggerFailed  = "workflow.trigger.failed.v1"
)

// CloudEvent is the minimal CloudEvents-shaped envelope trigger-router
// emits. It mirrors the shape used by workflow-runtime.events.go so
// downstream consumers (traceability-graph) need no special handling.
type CloudEvent struct {
	SpecVersion     string         `json:"specversion"`
	ID              string         `json:"id"`
	Type            string         `json:"type"`
	Source          string         `json:"source"`
	TenantID        string         `json:"tenant_id"`
	WorkspaceID     string         `json:"workspace_id"`
	WorkflowID      string         `json:"workflow_id"`
	Version         string         `json:"version"`
	TriggerID       string         `json:"trigger_id"`
	CorrelationID   string         `json:"correlation_id,omitempty"`
	Time            time.Time      `json:"time"`
	Data            map[string]any `json:"data,omitempty"`
}

// EventSink ships emitted events to the platform bus. In-process tests
// use MemorySink; production wires a Pulsar / NATS / Kafka producer.
type EventSink interface {
	Emit(ev CloudEvent) error
}

// NoopSink discards events; used when the bus is not wired (dev mode).
type NoopSink struct{}

func (NoopSink) Emit(CloudEvent) error { return nil }

// MemorySink retains every emitted event for assertion in tests.
type MemorySink struct {
	Events []CloudEvent
}

func (m *MemorySink) Emit(ev CloudEvent) error {
	m.Events = append(m.Events, ev)
	return nil
}

func (m *MemorySink) ByType(t string) []CloudEvent {
	out := []CloudEvent{}
	for _, e := range m.Events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func newEvent(sub Subscription, eventType, correlationID string, data map[string]any) CloudEvent {
	return CloudEvent{
		SpecVersion:   "1.0",
		ID:            uuid.NewString(),
		Type:          eventType,
		Source:        "forge.trigger-router",
		TenantID:      sub.TenantID,
		WorkspaceID:   sub.WorkspaceID,
		WorkflowID:    sub.WorkflowID,
		Version:       sub.Version,
		TriggerID:     sub.TriggerID,
		CorrelationID: correlationID,
		Time:          time.Now(),
		Data:          data,
	}
}
