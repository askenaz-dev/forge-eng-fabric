package sdlc

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	EventPhaseEntered     = "sdlc.phase.entered.v1"
	EventGateEvaluated    = "sdlc.phase.gate_evaluated.v1"
	EventPhaseProgressed  = "sdlc.phase.progressed.v1"
	EventPhaseBlocked     = "sdlc.phase.blocked.v1"
	EventPhaseRegressed   = "sdlc.phase.regressed.v1"
	EventOverrideConsumed = "policy.override.consumed.v1"
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

func newEvent(initiative *Initiative, req eventContext, eventType string, data map[string]any) CloudEvent {
	return CloudEvent{
		SpecVersion:        "1.0",
		ID:                 newID(),
		Source:             "forge://service/sdlc-orchestrator",
		Type:               eventType,
		Subject:            "sdlc-initiative/" + initiative.ID,
		Time:               time.Now().UTC(),
		DataContentType:    "application/json",
		ForgeTenantID:      req.tenantID,
		ForgeWorkspaceID:   initiative.WorkspaceID,
		ForgeActor:         req.actor,
		ForgeCorrelationID: req.correlationID,
		Data:               data,
	}
}

type eventContext struct {
	tenantID      string
	actor         string
	correlationID string
}
