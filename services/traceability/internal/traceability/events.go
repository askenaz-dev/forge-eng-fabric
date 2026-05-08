package traceability

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	EventLinkCreated       = "traceability.link.created.v1"
	EventBackfillCompleted = "traceability.backfill.completed.v1"
)

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

type Sink interface {
	Emit(event CloudEvent) error
}

type MemorySink struct {
	Events []CloudEvent
}

func (m *MemorySink) Emit(event CloudEvent) error {
	m.Events = append(m.Events, event)
	return nil
}

func (m *MemorySink) ByType(eventType string) []CloudEvent {
	out := []CloudEvent{}
	for _, event := range m.Events {
		if event.Type == eventType {
			out = append(out, event)
		}
	}
	return out
}

type LogSink struct{}

func (LogSink) Emit(event CloudEvent) error {
	data, _ := json.Marshal(event)
	fmt.Println("event", string(data))
	return nil
}

func newEvent(event BusEvent, eventType string, subject string, data map[string]any) CloudEvent {
	return CloudEvent{
		SpecVersion:        "1.0",
		ID:                 newID(),
		Source:             "forge://service/traceability",
		Type:               eventType,
		Subject:            subject,
		Time:               time.Now().UTC(),
		DataContentType:    "application/json",
		ForgeTenantID:      event.TenantID,
		ForgeWorkspaceID:   event.WorkspaceID,
		ForgeActor:         event.Actor,
		ForgeCorrelationID: event.CorrelationID,
		Data:               data,
	}
}
