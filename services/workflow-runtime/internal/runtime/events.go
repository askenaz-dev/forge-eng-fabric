package runtime

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CloudEvent type names emitted by the runtime.
const (
	EventExecutionStarted    = "workflow.execution.started.v1"
	EventStepStarted         = "workflow.step.started.v1"
	EventStepCompleted       = "workflow.step.completed.v1"
	EventStepFailed          = "workflow.step.failed.v1"
	EventRetried             = "workflow.retried.v1"
	EventCompensated         = "workflow.compensated.v1"
	EventStepWaitingHuman    = "workflow.step.waiting_human.v1"
	EventStepEscalated       = "workflow.step.escalated.v1"
	EventExecutionCompleted  = "workflow.execution.completed.v1"
	EventExecutionFailed     = "workflow.execution.failed.v1"
	EventGuardrailTrip       = "guardrail.trip.v1"
)

// CloudEvent matches the platform CloudEvents shape used by other services.
type CloudEvent struct {
	SpecVersion        string         `json:"specversion"`
	ID                 string         `json:"id"`
	Source             string         `json:"source"`
	Type               string         `json:"type"`
	Subject            string         `json:"subject,omitempty"`
	Time               time.Time      `json:"time"`
	DataContentType    string         `json:"datacontenttype"`
	ForgeTenantID      string         `json:"forgetenantid,omitempty"`
	ForgeWorkspaceID   string         `json:"forgeworkspaceid,omitempty"`
	ForgeActor         string         `json:"forgeactor,omitempty"`
	ForgeCorrelationID string         `json:"forgecorrelationid,omitempty"`
	Data               map[string]any `json:"data"`
}

// Sink is an event sink.
type Sink interface {
	Emit(event CloudEvent) error
}

// MemorySink keeps events in memory for tests.
type MemorySink struct {
	Events []CloudEvent
}

func (m *MemorySink) Emit(event CloudEvent) error {
	m.Events = append(m.Events, event)
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

// LogSink prints events to stdout.
type LogSink struct{}

func (LogSink) Emit(event CloudEvent) error {
	data, _ := json.Marshal(event)
	fmt.Println("event", string(data))
	return nil
}

func newEvent(exec *Execution, eventType string, data map[string]any) CloudEvent {
	return CloudEvent{
		SpecVersion:        "1.0",
		ID:                 uuid.NewString(),
		Source:             "forge://service/workflow-runtime",
		Type:               eventType,
		Subject:            "workflow-execution/" + exec.ID,
		Time:               time.Now().UTC(),
		DataContentType:    "application/json",
		ForgeTenantID:      exec.TenantID,
		ForgeWorkspaceID:   exec.WorkspaceID,
		ForgeCorrelationID: exec.CorrelationID,
		Data:               data,
	}
}
