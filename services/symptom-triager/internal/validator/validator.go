// Package validator validates incoming symptom events against the v1 schema.
package validator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

var knownDimensions = map[string]bool{
	"service":     true,
	"signal":      true,
	"tenant":      true,
	"workspace":   true,
	"error_class": true,
	"port":        true,
	"route":       true,
}

var knownSignals = map[string]bool{
	"probe-failed": true, "probe-recovered": true,
	"log-pattern": true, "metric-threshold": true,
	"ci-failure": true, "webhook": true,
}

var knownSeverities = map[string]bool{
	"low": true, "medium": true, "high": true, "critical": true,
}

// SymptomEvent is the wire format we validate.
type SymptomEvent struct {
	SymptomID       string    `json:"symptom_id"`
	Fingerprint     string    `json:"fingerprint"`
	Signal          string    `json:"signal"`
	Service         string    `json:"service"`
	Severity        string    `json:"severity"`
	Emitter         string    `json:"emitter"`
	TenantID        string    `json:"tenant_id,omitempty"`
	WorkspaceID     string    `json:"workspace_id,omitempty"`
	ErrorClass      string    `json:"error_class,omitempty"`
	Port            int       `json:"port,omitempty"`
	Route           string    `json:"route,omitempty"`
	EvidenceExcerpt string    `json:"evidence_excerpt"`
	EvidenceRef     string    `json:"evidence_ref,omitempty"`
	ObservedAt      time.Time `json:"observed_at"`
	SchemaVersion   string    `json:"schema_version"`
}

// Validate parses and validates the raw JSON bytes of a symptom event.
// Returns the parsed event and a nil error on success; a descriptive error on failure.
func Validate(raw []byte) (*SymptomEvent, error) {
	var evt SymptomEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}
	if evt.SymptomID == "" {
		return nil, fmt.Errorf("missing symptom_id")
	}
	if evt.Service == "" {
		return nil, fmt.Errorf("missing service")
	}
	if !knownSignals[evt.Signal] {
		return nil, fmt.Errorf("unknown signal %q", evt.Signal)
	}
	if !knownSeverities[evt.Severity] {
		return nil, fmt.Errorf("unknown severity %q", evt.Severity)
	}
	if evt.EvidenceExcerpt == "" {
		return nil, fmt.Errorf("missing evidence_excerpt")
	}
	if evt.SchemaVersion != "v1" {
		return nil, fmt.Errorf("unsupported schema_version %q", evt.SchemaVersion)
	}
	if err := validateFingerprint(evt.Fingerprint); err != nil {
		return nil, fmt.Errorf("fingerprint: %w", err)
	}
	return &evt, nil
}

func validateFingerprint(fp string) error {
	if fp == "" {
		return fmt.Errorf("empty")
	}
	parts := strings.Split(fp, "|")
	seen := map[string]bool{}
	dims := make([]string, 0, len(parts))
	hasService, hasSignal := false, false
	for _, p := range parts {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return fmt.Errorf("malformed pair %q", p)
		}
		k := kv[0]
		if !knownDimensions[k] {
			return fmt.Errorf("unknown dimension %q", k)
		}
		if seen[k] {
			return fmt.Errorf("duplicate dimension %q", k)
		}
		seen[k] = true
		dims = append(dims, k)
		if k == "service" {
			hasService = true
		}
		if k == "signal" {
			hasSignal = true
		}
	}
	if !hasService {
		return fmt.Errorf("missing required dimension 'service'")
	}
	if !hasSignal {
		return fmt.Errorf("missing required dimension 'signal'")
	}
	sorted := make([]string, len(dims))
	copy(sorted, dims)
	sort.Strings(sorted)
	for i, d := range dims {
		if d != sorted[i] {
			return fmt.Errorf("dimensions not sorted")
		}
	}
	return nil
}
