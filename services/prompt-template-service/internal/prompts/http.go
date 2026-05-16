package prompts

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Server exposes the HTTP API.
type Server struct {
	Renderer Renderer
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/render", s.handleRender)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RenderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad_json", http.StatusBadRequest)
		return
	}
	resp, err := s.Renderer.Render(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrBadRef):
			http.Error(w, "bad_template_ref", http.StatusBadRequest)
		case errors.Is(err, ErrUnknownTemplate):
			http.Error(w, "unknown_template", http.StatusNotFound)
		default:
			http.Error(w, "render_failed: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
