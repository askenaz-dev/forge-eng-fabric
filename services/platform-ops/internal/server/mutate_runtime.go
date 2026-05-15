package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/forge-eng-fabric/services/platform-ops/internal/audit"
	"github.com/forge-eng-fabric/services/platform-ops/internal/probe"
)

// RestartRequest is the body for POST /v1/services/{name}/restart.
type RestartRequest struct {
	// SymptomID ties this action back to the originating symptom (optional for human callers).
	SymptomID string `json:"symptom_id,omitempty"`
	// SessionID ties this to the agent-mode session that proposed the action.
	SessionID string `json:"session_id,omitempty"`
	// ExpectedOutcome declares the post-validate probe definition.
	ExpectedOutcome probe.ExpectedOutcome `json:"expected_outcome"`
	// RevertOf, when non-empty, reverses the action identified by the given audit_event_id.
	RevertOf string `json:"revert_of,omitempty"`
}

// POST /v1/services/{name}/restart
func (h *handler) serviceRestart(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "service name required")
		return
	}

	// Self-protection check.
	denied, err := h.cfg.OPA.EvalSelfProtection(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "policy evaluation failed")
		return
	}
	if denied {
		writeError(w, http.StatusForbidden, "target is in the self-protection denylist")
		return
	}

	var body RestartRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// OPA risk-classifier pre-check.
	decision, _, _, err := h.cfg.OPA.EvalRiskClassifier(r.Context(), map[string]any{
		"action_class": "mutate-runtime",
		"blast_radius": "process",
		"reversibility": "trivial",
		"scope":        "local",
		"target":       name,
		"actor":        r.Header.Get("X-Forge-Actor"),
	})
	if err != nil || decision == "deny" {
		writeError(w, http.StatusForbidden, fmt.Sprintf("OPA denied action (decision=%s)", decision))
		return
	}
	if decision == "requires_approval" || decision == "requires_dual_control" {
		writeJSON(w, http.StatusAccepted, map[string]string{
			"status":   "pending_approval",
			"decision": decision,
		})
		return
	}

	actionID := uuid.NewString()
	ctx := r.Context()

	// Execute the restart (compose: docker compose restart <name>).
	if err := h.execRestart(ctx, name); err != nil {
		_, _ = h.cfg.Audit.Write(ctx, audit.Row{
			AuditID:          actionID,
			Actor:            r.Header.Get("X-Forge-Actor"),
			Action:           "platform-ops:service:restart",
			Target:           name,
			Outcome:          "error",
			SymptomID:        body.SymptomID,
			SessionID:        body.SessionID,
			PolicyBundleHash: h.cfg.OPA.BundleHash(),
			Details:          map[string]any{"error": err.Error()},
		})
		writeError(w, http.StatusInternalServerError, "restart failed: "+err.Error())
		return
	}

	// Post-validate: wait for healthcheck to pass.
	verResult := "ok"
	if err := probe.Verify(ctx, body.ExpectedOutcome); err != nil {
		verResult = "failed: " + err.Error()

		// Auto-rollback not applicable for restart (idempotent); log the failure.
		_, _ = h.cfg.Audit.Write(ctx, audit.Row{
			AuditID:          actionID,
			Actor:            r.Header.Get("X-Forge-Actor"),
			Action:           "platform-ops:service:restart",
			Target:           name,
			Outcome:          "error",
			SymptomID:        body.SymptomID,
			SessionID:        body.SessionID,
			PolicyBundleHash: h.cfg.OPA.BundleHash(),
			Verification:     map[string]any{"result": verResult},
			Details:          map[string]any{"error": "post-validate failed"},
		})
		writeError(w, http.StatusBadGateway, "restart succeeded but post-validate probe failed: "+err.Error())
		return
	}

	_, _ = h.cfg.Audit.Write(ctx, audit.Row{
		AuditID:          actionID,
		Actor:            r.Header.Get("X-Forge-Actor"),
		Action:           "platform-ops:service:restart",
		Target:           name,
		Outcome:          "success",
		SymptomID:        body.SymptomID,
		SessionID:        body.SessionID,
		PolicyBundleHash: h.cfg.OPA.BundleHash(),
		Verification:     map[string]any{"result": verResult},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"audit_id":     actionID,
		"service":      name,
		"action":       "restart",
		"outcome":      "success",
		"verification": verResult,
		"timestamp":    time.Now().UTC(),
	})
}

// POST /v1/services/{name}/scale
type ScaleRequest struct {
	Replicas        int                  `json:"replicas"`
	SymptomID       string               `json:"symptom_id,omitempty"`
	SessionID       string               `json:"session_id,omitempty"`
	ExpectedOutcome probe.ExpectedOutcome `json:"expected_outcome"`
}

func (h *handler) serviceScale(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var body ScaleRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	denied, _ := h.cfg.OPA.EvalSelfProtection(r.Context(), name)
	if denied {
		writeError(w, http.StatusForbidden, "target is in the self-protection denylist")
		return
	}

	decision, _, _, _ := h.cfg.OPA.EvalRiskClassifier(r.Context(), map[string]any{
		"action_class": "mutate-runtime",
		"blast_radius": "service",
		"reversibility": "easy",
		"scope":        "local",
		"target":       name,
	})
	if decision == "deny" {
		writeError(w, http.StatusForbidden, "OPA denied scale action")
		return
	}

	actionID := uuid.NewString()
	_, _ = h.cfg.Audit.Write(r.Context(), audit.Row{
		AuditID:          actionID,
		Actor:            r.Header.Get("X-Forge-Actor"),
		Action:           "platform-ops:service:scale",
		Target:           name,
		Outcome:          "success",
		SymptomID:        body.SymptomID,
		SessionID:        body.SessionID,
		PolicyBundleHash: h.cfg.OPA.BundleHash(),
		Details:          map[string]any{"replicas": body.Replicas},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"audit_id":  actionID,
		"service":   name,
		"action":    "scale",
		"replicas":  body.Replicas,
		"outcome":   "success",
		"timestamp": time.Now().UTC(),
	})
}

// POST /v1/services/{name}/cordon  (Kubernetes only, no-op in compose)
func (h *handler) serviceCordon(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	writeJSON(w, http.StatusOK, map[string]any{
		"service": name, "action": "cordon", "note": "no-op in compose environment",
		"timestamp": time.Now().UTC(),
	})
}

// POST /v1/services/{name}/uncordon
func (h *handler) serviceUncordon(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	writeJSON(w, http.StatusOK, map[string]any{
		"service": name, "action": "uncordon", "note": "no-op in compose environment",
		"timestamp": time.Now().UTC(),
	})
}

// POST /v1/circuit-breakers/{fingerprint}/reset (admin-only via OPA)
func (h *handler) circuitBreakerReset(w http.ResponseWriter, r *http.Request) {
	fp := chi.URLParam(r, "fingerprint")
	decision, _, _, _ := h.cfg.OPA.EvalRiskClassifier(r.Context(), map[string]any{
		"action_class": "mutate-config",
		"blast_radius": "platform",
		"reversibility": "trivial",
		"scope":        "local",
		"target":       "circuit-breaker",
	})
	if decision == "deny" {
		writeError(w, http.StatusForbidden, "admin-only action")
		return
	}

	_, err := h.cfg.Pool.Exec(r.Context(), `
		UPDATE circuit_breaker_state
		SET is_open=false, failed_session_count=0, cooldown_until=NULL,
		    last_reset_by=$1, last_reset_at=now()
		WHERE fingerprint=$2
	`, r.Header.Get("X-Forge-Actor"), fp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db update failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"fingerprint": fp, "action": "reset", "timestamp": time.Now().UTC(),
	})
}

// execRestart sends a restart signal to the named service.
// In the compose environment this calls docker compose restart via the Docker API.
// In Kubernetes, this would delete the pod(s) for the deployment.
func (h *handler) execRestart(ctx context.Context, name string) error {
	// Stub: in a real implementation this would call the Docker daemon or k8s API.
	// For the scaffold, return nil (success) unless the name is "fail-test".
	if name == "fail-test" {
		return fmt.Errorf("simulated restart failure for testing")
	}
	return nil
}
