package observability

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CloudEvent matches platform conventions.
type CloudEvent struct {
	SpecVersion      string         `json:"specversion"`
	ID               string         `json:"id"`
	Source           string         `json:"source"`
	Type             string         `json:"type"`
	Subject          string         `json:"subject,omitempty"`
	Time             time.Time      `json:"time"`
	DataContentType  string         `json:"datacontenttype"`
	ForgeTenantID    string         `json:"forgetenantid,omitempty"`
	ForgeWorkspaceID string         `json:"forgeworkspaceid,omitempty"`
	Data             map[string]any `json:"data"`
}

// Sink emits events.
type Sink interface{ Emit(e CloudEvent) error }

type MemorySink struct{ Events []CloudEvent }

func (m *MemorySink) Emit(e CloudEvent) error { m.Events = append(m.Events, e); return nil }

type LogSink struct{}

func (LogSink) Emit(e CloudEvent) error {
	data, _ := json.Marshal(e)
	fmt.Println("event", string(data))
	return nil
}

func newEvent(tenantID, workspaceID, eventType, subject string, data map[string]any) CloudEvent {
	return CloudEvent{
		SpecVersion:      "1.0",
		ID:               uuid.NewString(),
		Source:           "forge://service/asset-observability",
		Type:             eventType,
		Subject:          subject,
		Time:             time.Now().UTC(),
		DataContentType: "application/json",
		ForgeTenantID:    tenantID,
		ForgeWorkspaceID: workspaceID,
		Data:             data,
	}
}
