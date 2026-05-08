package runtime

import (
	"encoding/json"
	"fmt"
	"time"
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

func newEvent(r *Runtime, eventType string, data map[string]any) CloudEvent {
	subject := ""
	if r != nil {
		subject = "runtime/" + r.ID
	}
	tenantID, workspaceID := "", ""
	if r != nil {
		tenantID = r.TenantID
		workspaceID = r.WorkspaceID
	}
	return CloudEvent{
		SpecVersion:      "1.0",
		ID:               newID(),
		Source:           "forge://service/runtime-registry",
		Type:             eventType,
		Subject:          subject,
		Time:             time.Now().UTC(),
		DataContentType:  "application/json",
		ForgeTenantID:    tenantID,
		ForgeWorkspaceID: workspaceID,
		ForgeActor:       "service:runtime-registry",
		Data:             data,
	}
}
