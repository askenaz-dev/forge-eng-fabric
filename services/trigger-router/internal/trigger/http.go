package trigger

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Server wires every HTTP handler trigger-router exposes:
//   - /v1/hooks/in/{workflow_id}/{trigger_id}  -> WebhookHandler
//   - /v1/triggers/{workflow_id}/{trigger_id}/fire  -> manual fire (admin / Portal test)
//   - /v1/triggers/_status  -> list current subscriptions (admin)
//   - /healthz  -> liveness
type Server struct {
	Registry   *Registry
	Dispatcher *Dispatcher
	Webhook    *WebhookHandler
}

// Handler returns an http.Handler with all routes mounted.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/hooks/in/", s.Webhook)
	mux.HandleFunc("/v1/triggers/", s.handleTriggerAdmin)
	mux.HandleFunc("/v1/triggers/_status", s.handleStatus)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	subs := s.Registry.All()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count":         len(subs),
		"subscriptions": subs,
	})
}

// handleTriggerAdmin services /v1/triggers/{workflow_id}/{trigger_id}/fire
// for manual fires originated from the Portal's "Run now" button or the
// admin CLI.
func (s *Server) handleTriggerAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	const prefix = "/v1/triggers/"
	path := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[2] != "fire" {
		http.Error(w, "bad_path: expected /v1/triggers/{workflow_id}/{trigger_id}/fire", http.StatusBadRequest)
		return
	}
	workflowID, triggerID := parts[0], parts[1]
	sub, ok := s.Registry.Lookup(workflowID, triggerID, "")
	if !ok {
		http.Error(w, "trigger_not_registered", http.StatusNotFound)
		return
	}
	payload := map[string]any{}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad_json", http.StatusBadRequest)
			return
		}
	}
	payload["_manual_fired_at"] = time.Now().UTC()
	execID, err := s.Dispatcher.Fire(r.Context(), sub, payload)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, ErrDropConcurrency) {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"execution_id": execID})
}
