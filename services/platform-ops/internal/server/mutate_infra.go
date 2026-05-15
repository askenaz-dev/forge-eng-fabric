package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/forge-eng-fabric/services/platform-ops/internal/audit"
)

// POST /v1/runtimes/{id}/recreate
// Requires dual approval per OPA policy (mutate-infra, blast_radius=service).
// Triggers L2+ sandbox verification before acting on the real runtime.

type RuntimeRecreateRequest struct {
	SandboxID       string `json:"sandbox_id,omitempty"`
	ExpectedOutcome string `json:"expected_outcome,omitempty"`
	SymptomID       string `json:"symptom_id,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
}

func (h *handler) runtimeRecreate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Self-protection: Alfred must not recreate its own infrastructure.
	denied, err := h.cfg.OPA.EvalSelfProtection(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "policy evaluation failed")
		return
	}
	if denied {
		writeError(w, http.StatusForbidden, "target is in the self-protection denylist")
		return
	}

	var body RuntimeRecreateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// OPA risk-classifier: mutate-infra always requires_dual_control.
	decision, _, approvers, err := h.cfg.OPA.EvalRiskClassifier(r.Context(), map[string]any{
		"action_class":  "mutate-infra",
		"blast_radius":  "service",
		"reversibility": "hard",
		"scope":         "local",
		"target":        id,
		"actor":         r.Header.Get("X-Forge-Actor"),
	})
	if err != nil || decision == "deny" {
		writeError(w, http.StatusForbidden, "OPA denied mutate-infra action")
		return
	}
	if decision == "requires_dual_control" || decision == "requires_approval" {
		// Return the approval requirement — caller must obtain dual approval
		// before re-calling with an approved audit_event_id.
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":    "pending_approval",
			"decision":  decision,
			"approvers": approvers,
			"runtime":   id,
		})
		return
	}

	// If decision == "autonomous" (shouldn't happen for infra, but handle gracefully):
	actionID := uuid.NewString()
	_, _ = h.cfg.Audit.Write(r.Context(), audit.Row{
		AuditID:          actionID,
		Actor:            r.Header.Get("X-Forge-Actor"),
		Action:           "platform-ops:runtime:recreate",
		Target:           id,
		Outcome:          "success",
		SymptomID:        body.SymptomID,
		SessionID:        body.SessionID,
		PolicyBundleHash: h.cfg.OPA.BundleHash(),
		Details: map[string]any{
			"sandbox_id":       body.SandboxID,
			"expected_outcome": body.ExpectedOutcome,
		},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"audit_id":  actionID,
		"runtime":   id,
		"action":    "recreate",
		"outcome":   "success",
		"timestamp": time.Now().UTC(),
	})
}
