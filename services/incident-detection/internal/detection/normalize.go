package detection

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PrometheusAlert is the subset of Alertmanager webhook fields we consume.
type PrometheusAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
}

// PrometheusWebhook is the standard Alertmanager v4 payload.
type PrometheusWebhook struct {
	Version  string            `json:"version"`
	Receiver string            `json:"receiver"`
	Status   string            `json:"status"`
	Alerts   []PrometheusAlert `json:"alerts"`
	Common   map[string]string `json:"commonLabels"`
}

// CloudMonitoringIncident is the GCP webhook subset.
type CloudMonitoringIncident struct {
	IncidentID  string            `json:"incident_id"`
	ResourceID  string            `json:"resource_id"`
	State       string            `json:"state"` // open / closed
	StartedAt   int64             `json:"started_at"` // unix seconds
	Summary     string            `json:"summary"`
	PolicyName  string            `json:"policy_name"`
	Labels      map[string]string `json:"resource_labels"`
}

// CloudMonitoringWebhook wraps the incident payload.
type CloudMonitoringWebhook struct {
	Version  string                  `json:"version"`
	Incident CloudMonitoringIncident `json:"incident"`
}

// LokiAlert mirrors Loki ruler webhook payloads (compatible with Alertmanager).
type LokiAlert = PrometheusAlert
type LokiWebhook = PrometheusWebhook

// InternalEvent is the body for ingest of platform CloudEvents.
type InternalEvent struct {
	Type        string            `json:"type"`
	Subject     string            `json:"subject"`
	TenantID    string            `json:"forgetenantid"`
	WorkspaceID string            `json:"forgeworkspaceid"`
	Data        map[string]any    `json:"data"`
	Time        time.Time         `json:"time"`
}

// signatureHash computes a stable hash for dedup based on the alert kind +
// service + environment.
func signatureHash(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, strings.ToLower(p))
		}
	}
	sort.Strings(cleaned)
	h := sha256.Sum256([]byte(strings.Join(cleaned, "|")))
	return hex.EncodeToString(h[:])[:16]
}

// fromPrometheus builds an Incident + Event from a Prometheus alert.
func fromPrometheus(a PrometheusAlert) (*Incident, Event, error) {
	if a.Status == "resolved" {
		return nil, Event{}, ErrInvalidPayload
	}
	service := a.Labels["service"]
	if service == "" {
		service = a.Labels["job"]
	}
	env := a.Labels["env"]
	if env == "" {
		env = a.Labels["environment"]
	}
	if service == "" || env == "" {
		return nil, Event{}, fmt.Errorf("%w: missing service/env labels", ErrInvalidPayload)
	}
	alertname := a.Labels["alertname"]
	if alertname == "" {
		alertname = a.Annotations["summary"]
	}
	sig := signatureHash(service, env, alertname)
	severity := Severity(a.Labels["severity"])
	if severity == "" {
		severity = SeverityWarning
	}
	now := time.Now().UTC()
	if !a.StartsAt.IsZero() {
		now = a.StartsAt.UTC()
	}
	inc := &Incident{
		ID:            "inc-" + uuid.NewString(),
		TenantID:      a.Labels["tenant"],
		WorkspaceID:   a.Labels["workspace"],
		Service:       service,
		Environment:   env,
		SignatureHash: sig,
		Source:        SourcePrometheus,
		Severity:      severity,
		Title:         alertname,
		Description:   a.Annotations["description"],
		Labels:        a.Labels,
		Synthetic:     a.Labels["synthetic"] == "true",
	}
	ev := Event{
		ID:         "ie-" + uuid.NewString(),
		Source:     SourcePrometheus,
		Severity:   severity,
		OccurredAt: now,
		Payload: map[string]any{
			"labels":      a.Labels,
			"annotations": a.Annotations,
		},
		Labels: a.Labels,
	}
	return inc, ev, nil
}

// fromCloudMonitoring builds an Incident + Event from a Cloud Monitoring payload.
func fromCloudMonitoring(p CloudMonitoringIncident) (*Incident, Event, error) {
	if p.State == "closed" {
		return nil, Event{}, ErrInvalidPayload
	}
	service := p.Labels["service"]
	env := p.Labels["env"]
	if env == "" {
		env = p.Labels["environment"]
	}
	if service == "" || env == "" {
		return nil, Event{}, fmt.Errorf("%w: missing service/env labels", ErrInvalidPayload)
	}
	policy := p.PolicyName
	if policy == "" {
		policy = "cloud-monitoring-incident"
	}
	sig := signatureHash(service, env, policy)
	now := time.Now().UTC()
	if p.StartedAt > 0 {
		now = time.Unix(p.StartedAt, 0).UTC()
	}
	inc := &Incident{
		ID:            "inc-" + uuid.NewString(),
		TenantID:      p.Labels["tenant"],
		WorkspaceID:   p.Labels["workspace"],
		Service:       service,
		Environment:   env,
		SignatureHash: sig,
		Source:        SourceCloudMonitoring,
		Severity:      SeverityWarning,
		Title:         policy,
		Description:   p.Summary,
		Labels:        p.Labels,
		Synthetic:     p.Labels["synthetic"] == "true",
	}
	ev := Event{
		ID:         "ie-" + uuid.NewString(),
		Source:     SourceCloudMonitoring,
		Severity:   SeverityWarning,
		OccurredAt: now,
		Payload: map[string]any{
			"incident_id": p.IncidentID,
			"resource_id": p.ResourceID,
			"summary":     p.Summary,
		},
		Labels: p.Labels,
	}
	return inc, ev, nil
}

// fromInternal builds an Incident + Event from an internal CloudEvent.
// Supported event types: slo.burn-rate.fast.v1, cost.spike.v1,
// eval.regression.detected.v1, iac.drift.detected.v1, deployment.failed.v1.
func fromInternal(e InternalEvent) (*Incident, Event, error) {
	supported := map[string]Severity{
		"slo.burn-rate.fast.v1":       SeverityCritical,
		"cost.spike.v1":               SeverityWarning,
		"eval.regression.detected.v1": SeverityWarning,
		"iac.drift.detected.v1":       SeverityWarning,
		"deployment.failed.v1":        SeverityCritical,
	}
	severity, ok := supported[e.Type]
	if !ok {
		return nil, Event{}, ErrUnknownSource
	}
	service, _ := e.Data["service"].(string)
	env, _ := e.Data["env"].(string)
	if env == "" {
		env, _ = e.Data["environment"].(string)
	}
	if service == "" {
		service = e.Subject
	}
	if env == "" {
		env = "unknown"
	}
	sig := signatureHash(service, env, e.Type)
	now := e.Time
	if now.IsZero() {
		now = time.Now().UTC()
	}
	labels := map[string]string{"event_type": e.Type, "service": service, "env": env}
	synthetic := false
	if v, ok := e.Data["synthetic"].(bool); ok && v {
		synthetic = true
	}
	inc := &Incident{
		ID:            "inc-" + uuid.NewString(),
		TenantID:      e.TenantID,
		WorkspaceID:   e.WorkspaceID,
		Service:       service,
		Environment:   env,
		SignatureHash: sig,
		Source:        SourceInternal,
		Severity:      severity,
		Title:         e.Type,
		Description:   stringFromAny(e.Data["description"]),
		Labels:        labels,
		Synthetic:     synthetic,
	}
	ev := Event{
		ID:         "ie-" + uuid.NewString(),
		Source:     SourceInternal,
		Severity:   severity,
		OccurredAt: now,
		Payload:    e.Data,
		Labels:     labels,
	}
	return inc, ev, nil
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
