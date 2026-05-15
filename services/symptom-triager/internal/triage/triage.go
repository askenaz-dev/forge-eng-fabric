// Package triage implements triage decision logic.
// In iter 1: structured-log-only (no session spawning).
// In iter 2+: spawns sessions via the spawner package.
package triage

import (
	"context"
	"log/slog"

	"github.com/forge-eng-fabric/services/symptom-triager/internal/spawner"
	"github.com/forge-eng-fabric/services/symptom-triager/internal/validator"
)

// Decision describes what the triager decided for a given symptom.
type Decision struct {
	Action     string // "log_only" | "spawn_session" | "queue_hitl" | "suppress"
	Reason     string
	PlaybookID string
	SessionID  string
}

// Engine applies triage rules to validated symptom events.
type Engine struct {
	spawner         *spawner.Spawner
	sessionSpawning bool
	alfredWorkspace string
}

// NewEngine creates a triage Engine.
func NewEngine(s *spawner.Spawner, sessionSpawning bool, alfredWorkspace string) *Engine {
	return &Engine{spawner: s, sessionSpawning: sessionSpawning, alfredWorkspace: alfredWorkspace}
}

// Decide applies triage rules and returns a Decision.
// Rule evaluation order matches the spec:
//  1. circuit breaker open → queue_hitl
//  2. noise rule matches → suppress
//  3. known playbook → spawn_session with playbook
//  4. symptom grade ≥ threshold → spawn diagnose-then-propose session
//  5. default → log_only
func (e *Engine) Decide(ctx context.Context, evt *validator.SymptomEvent) Decision {
	// Rule 5 (default) — iter 1 passive observation.
	if !e.sessionSpawning {
		d := Decision{Action: "log_only", Reason: "session_spawning_disabled"}
		logDecision(evt, d)
		return d
	}

	// Rule 4 — spawn a diagnose-then-propose session (iter 2 minimum).
	if evt.Severity == "high" || evt.Severity == "critical" {
		resp, err := e.spawner.Spawn(ctx, spawner.SessionRequest{
			WorkspaceID:     e.alfredWorkspace,
			CorrelationID:   evt.SymptomID,
			AutononymyPreset: "diagnose-then-propose",
			Actor:           "system:alfred",
			TriggerSource:   "symptom",
			SymptomID:       evt.SymptomID,
		})
		if err != nil {
			slog.Error("triage: spawn session failed", "err", err, "symptom_id", evt.SymptomID)
			d := Decision{Action: "log_only", Reason: "spawn_error: " + err.Error()}
			logDecision(evt, d)
			return d
		}
		d := Decision{
			Action:    "spawn_session",
			Reason:    "severity=" + evt.Severity,
			SessionID: resp.SessionID,
		}
		logDecision(evt, d)
		return d
	}

	d := Decision{Action: "log_only", Reason: "severity_below_threshold"}
	logDecision(evt, d)
	return d
}

func logDecision(evt *validator.SymptomEvent, d Decision) {
	slog.Info("triage decision",
		"symptom_id", evt.SymptomID,
		"fingerprint", evt.Fingerprint,
		"signal", evt.Signal,
		"service", evt.Service,
		"severity", evt.Severity,
		"action", d.Action,
		"reason", d.Reason,
		"session_id", d.SessionID,
	)
}
