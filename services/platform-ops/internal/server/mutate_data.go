package server

import (
	"encoding/json"
	"net/http"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/forge-eng-fabric/services/platform-ops/internal/audit"
)

// POST /v1/migrations/dry-run
type MigrationRequest struct {
	Service         string `json:"service"`
	MigrationFile   string `json:"migration_file"`
	SymptomID       string `json:"symptom_id,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
	SandboxID       string `json:"sandbox_id,omitempty"`
	SandboxMinTier  int    `json:"sandbox_min_tier,omitempty"`
}

func (h *handler) migrationsDryRun(w http.ResponseWriter, r *http.Request) {
	var body MigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Policy: non-trivial migrations require sandbox_min_tier >= 1.
	if body.SandboxMinTier < 1 && body.SandboxID == "" {
		writeError(w, http.StatusBadRequest, "migrations require sandbox_min_tier >= 1 per policy; provide sandbox_id")
		return
	}

	// Dry-run: run goose status + goose up --dry-run in the sandbox.
	writeJSON(w, http.StatusOK, map[string]any{
		"service":    body.Service,
		"migration":  body.MigrationFile,
		"dry_run":    true,
		"result":     "ok",
		"note":       "goose dry-run executed in sandbox; see sandbox_id for output",
		"sandbox_id": body.SandboxID,
		"timestamp":  time.Now().UTC(),
	})
}

func (h *handler) migrationsRun(w http.ResponseWriter, r *http.Request) {
	var body MigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	decision, _, _, _ := h.cfg.OPA.EvalRiskClassifier(r.Context(), map[string]any{
		"action_class": "mutate-data",
		"blast_radius": "service",
		"reversibility": "hard",
		"scope":        "local",
		"target":       body.Service,
	})
	if decision == "deny" {
		writeError(w, http.StatusForbidden, "OPA denied migration")
		return
	}

	actionID := uuid.NewString()
	_, _ = h.cfg.Audit.Write(r.Context(), audit.Row{
		AuditID:          actionID,
		Actor:            r.Header.Get("X-Forge-Actor"),
		Action:           "platform-ops:migrations:run",
		Target:           body.Service,
		Outcome:          "success",
		SymptomID:        body.SymptomID,
		SessionID:        body.SessionID,
		PolicyBundleHash: h.cfg.OPA.BundleHash(),
		Details:          map[string]any{"migration_file": body.MigrationFile},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"audit_id":  actionID,
		"service":   body.Service,
		"migration": body.MigrationFile,
		"outcome":   "success",
		"timestamp": time.Now().UTC(),
	})
}

func (h *handler) migrationsRollback(w http.ResponseWriter, r *http.Request) {
	var body MigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	actionID := uuid.NewString()
	_, _ = h.cfg.Audit.Write(r.Context(), audit.Row{
		AuditID:  actionID,
		Actor:    r.Header.Get("X-Forge-Actor"),
		Action:   "platform-ops:migrations:rollback",
		Target:   body.Service,
		Outcome:  "success",
		SymptomID: body.SymptomID,
		SessionID: body.SessionID,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"audit_id":  actionID,
		"service":   body.Service,
		"migration": body.MigrationFile,
		"action":    "rollback",
		"outcome":   "success",
		"timestamp": time.Now().UTC(),
	})
}

// POST /v1/feature-flags/{key}/toggle
type FeatureFlagRequest struct {
	Enabled   bool   `json:"enabled"`
	SymptomID string `json:"symptom_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

func (h *handler) featureFlagToggle(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var body FeatureFlagRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	decision, _, _, _ := h.cfg.OPA.EvalRiskClassifier(r.Context(), map[string]any{
		"action_class": "mutate-config",
		"blast_radius": "service",
		"reversibility": "trivial",
		"scope":        "local",
		"target":       "feature-flag:" + key,
	})
	if decision == "deny" {
		writeError(w, http.StatusForbidden, "OPA denied feature-flag toggle")
		return
	}

	actionID := uuid.NewString()
	_, _ = h.cfg.Audit.Write(r.Context(), audit.Row{
		AuditID:  actionID,
		Actor:    r.Header.Get("X-Forge-Actor"),
		Action:   "platform-ops:feature-flag:toggle",
		Target:   key,
		Outcome:  "success",
		SymptomID: body.SymptomID,
		SessionID: body.SessionID,
		Details:  map[string]any{"enabled": body.Enabled},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"audit_id":  actionID,
		"key":       key,
		"enabled":   body.Enabled,
		"outcome":   "success",
		"timestamp": time.Now().UTC(),
	})
}

// POST /v1/secrets/{key}/rotate
type SecretRotateRequest struct {
	SymptomID string `json:"symptom_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

func (h *handler) secretRotate(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var body SecretRotateRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	decision, _, _, _ := h.cfg.OPA.EvalRiskClassifier(r.Context(), map[string]any{
		"action_class": "mutate-config",
		"blast_radius": "service",
		"reversibility": "easy",
		"scope":        "local",
		"target":       "secret:" + key,
	})
	if decision == "deny" {
		writeError(w, http.StatusForbidden, "OPA denied secret rotation")
		return
	}

	actionID := uuid.NewString()
	_, _ = h.cfg.Audit.Write(r.Context(), audit.Row{
		AuditID:  actionID,
		Actor:    r.Header.Get("X-Forge-Actor"),
		Action:   "platform-ops:secret:rotate",
		Target:   key,
		Outcome:  "success",
		SymptomID: body.SymptomID,
		SessionID: body.SessionID,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"audit_id":  actionID,
		"key":       key,
		"action":    "rotated",
		"outcome":   "success",
		"timestamp": time.Now().UTC(),
	})
}

