// Package events emits the gateway's CloudEvents to Kafka so the
// asset-observability service can aggregate gateway traffic alongside
// internal traffic.
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	TypeInvocation = "com.forge.gateway.invocation.v1"
	TypeInstalled  = "com.forge.gateway.installed.v1"
)

// Producer wraps a Kafka client and the gateway's events topic.
type Producer struct {
	Client *kgo.Client
	Topic  string
}

// InvocationEvent is the body emitted on every MCP / A2A / package request.
type InvocationEvent struct {
	Route          string `json:"route"`
	AssetID        string `json:"asset_id,omitempty"`
	AssetVersion   string `json:"asset_version,omitempty"`
	TenantID       string `json:"tenant_id"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	DeveloperSub   string `json:"developer_sub"`
	Client         string `json:"client,omitempty"`
	Outcome        string `json:"outcome"`
	StatusCode     int    `json:"status_code"`
	LatencyMS      int64  `json:"latency_ms"`
	CostUSDCents   int64  `json:"cost_usd_cents,omitempty"`
	CorrelationID  string `json:"correlation_id"`
}

// InstalledEvent is emitted when the CLI downloads a package.
type InstalledEvent struct {
	AssetID       string `json:"asset_id"`
	AssetVersion  string `json:"asset_version"`
	TenantID      string `json:"tenant_id"`
	DeveloperSub  string `json:"developer_sub"`
	Client        string `json:"client"`
	PackageDigest string `json:"package_digest"`
	CorrelationID string `json:"correlation_id"`
}

// EmitInvocation produces the invocation CloudEvent.
func (p *Producer) EmitInvocation(ctx context.Context, ev InvocationEvent) {
	if p == nil || p.Client == nil {
		return
	}
	p.emit(ctx, TypeInvocation, ev.TenantID, "invocation/"+ev.Route, ev)
}

// EmitInstalled produces the installed CloudEvent.
func (p *Producer) EmitInstalled(ctx context.Context, ev InstalledEvent) {
	if p == nil || p.Client == nil {
		return
	}
	p.emit(ctx, TypeInstalled, ev.TenantID, "asset/"+ev.AssetID+"@"+ev.AssetVersion, ev)
}

func (p *Producer) emit(ctx context.Context, eventType, tenantID, subject string, data any) {
	envelope := map[string]any{
		"specversion":     "1.0",
		"id":              uuid.NewString(),
		"source":          "forge://service/skill-gateway",
		"type":            eventType,
		"subject":         subject,
		"time":            time.Now().UTC().Format(time.RFC3339Nano),
		"datacontenttype": "application/json",
		"forgetenantid":   tenantID,
		"data":            data,
	}
	body, _ := json.Marshal(envelope)
	_ = p.Client.ProduceSync(ctx, &kgo.Record{
		Topic:   p.Topic,
		Key:     []byte(tenantID),
		Value:   body,
		Headers: []kgo.RecordHeader{{Key: "ce_type", Value: []byte(eventType)}},
	}).FirstErr()
}
