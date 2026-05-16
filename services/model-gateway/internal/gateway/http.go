package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Server exposes the HTTP API.
type Server struct {
	Resolver Resolver
}

// Handler returns the mux with routes mounted.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/resolve", s.handleResolve)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad_json", http.StatusBadRequest)
		return
	}
	resp, err := s.Resolver.Resolve(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrModelNotWhitelisted):
			http.Error(w, "model_not_whitelisted", http.StatusForbidden)
		case errors.Is(err, ErrBadRef):
			http.Error(w, "bad_model_ref", http.StatusBadRequest)
		case errors.Is(err, ErrUnknownModel):
			http.Error(w, "unknown_model", http.StatusNotFound)
		default:
			http.Error(w, "resolve_failed: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
