package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// GET /v1/diagnostics/probe
// Returns the current health status of all registered probes.
func (h *handler) diagnosticProbe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// In iter 2 we query the live probe state from symptom-emitter-probe via
	// a shared Postgres table or direct HTTP. For now, return a synthetic response.
	_ = ctx
	writeJSON(w, http.StatusOK, map[string]any{
		"probes":    []any{},
		"timestamp": time.Now().UTC(),
		"note":      "probe state aggregated from symptom-emitter-probe in iter2+",
	})
}

// GET /v1/diagnostics/inspect/{service}
// Returns service metadata: image, replica count, last deploy, env snapshot.
func (h *handler) diagnosticInspect(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")
	if service == "" {
		writeError(w, http.StatusBadRequest, "service name required")
		return
	}

	// Self-protection: Alfred must not inspect its own infrastructure control plane.
	denied, err := h.cfg.OPA.EvalSelfProtection(r.Context(), service)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "policy evaluation failed")
		return
	}
	if denied {
		writeError(w, http.StatusForbidden, "target is in the self-protection denylist")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"service":    service,
		"status":     "unknown",
		"replicas":   0,
		"image":      "",
		"last_deploy": nil,
		"timestamp":  time.Now().UTC(),
		"note":       "live data populated from runtime-registry in iter2+",
	})
}

// GET /v1/diagnostics/logs?fingerprint=...
// Returns the most recent sanitised log events matching a fingerprint.
func (h *handler) diagnosticLogs(w http.ResponseWriter, r *http.Request) {
	fingerprint := r.URL.Query().Get("fingerprint")
	if fingerprint == "" {
		writeError(w, http.StatusBadRequest, "fingerprint query param required")
		return
	}

	// Query symptom event rows from the symptom_session table / Loki.
	// In iter 2 this is a stub; real Loki query wired in iter 3.
	writeJSON(w, http.StatusOK, map[string]any{
		"fingerprint": fingerprint,
		"events":      []any{},
		"timestamp":   time.Now().UTC(),
		"note":        "events fetched from Loki via evidence_ref in iter2+",
	})
}

func (h *handler) notImplemented(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "endpoint not yet implemented in this iteration",
		"path":  r.URL.Path,
	})
}

// Ensure json is imported even if only used via writeJSON.
var _ = json.Marshal
