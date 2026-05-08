package onboarding

import (
	"encoding/json"
	"fmt"
	"time"
)

// CloudEvent is a minimal CloudEvents 1.0 envelope sufficient for the
// `app.onboarding.*`, `repo.created.v1`, `branch_protection.applied.v1`
// types referenced in the spec.
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

// Sink consumes emitted CloudEvents. Production wires this to Kafka; tests
// use the in-memory implementation.
type Sink interface {
	Emit(ev CloudEvent) error
}

type MemorySink struct {
	Events []CloudEvent
}

func (m *MemorySink) Emit(ev CloudEvent) error {
	m.Events = append(m.Events, ev)
	return nil
}

func (m *MemorySink) ByType(t string) []CloudEvent {
	var out []CloudEvent
	for _, e := range m.Events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

type LogSink struct{}

func (LogSink) Emit(ev CloudEvent) error {
	b, _ := json.Marshal(ev)
	fmt.Println("event", string(b))
	return nil
}

func newCloudEvent(req *Request, eventType, subject string, data map[string]any) CloudEvent {
	return CloudEvent{
		SpecVersion:        "1.0",
		ID:                 newID(),
		Source:             "forge://service/app-onboarding",
		Type:               eventType,
		Subject:            subject,
		Time:               time.Now().UTC(),
		DataContentType:    "application/json",
		ForgeTenantID:      req.TenantID,
		ForgeWorkspaceID:   req.WorkspaceID,
		ForgeActor:         "service:app-onboarding",
		ForgeCorrelationID: req.CorrelationID,
		Data:               data,
	}
}
