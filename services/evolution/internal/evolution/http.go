package evolution

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler binds the service to HTTP.
type Handler struct{ Service *Service }

// NewHandler wraps a service.
func NewHandler(s *Service) *Handler { return &Handler{Service: s} }

// Mount installs the routes.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /v1/evolution/from-postmortem", h.handleFromPostmortem)
	mux.HandleFunc("POST /v1/evolution/proposals/", h.handleProposalAction)
	mux.HandleFunc("GET /v1/evolution/proposals", h.handleList)
	mux.HandleFunc("GET /v1/evolution/proposals/", h.handleGet)
	mux.HandleFunc("GET /v1/evolution/stats", h.handleStats)
}

func (h *Handler) handleFromPostmortem(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var in PostmortemInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, err := h.Service.FromPostmortem(r.Context(), in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	status := ProposalStatus(r.URL.Query().Get("status"))
	tenant := r.URL.Query().Get("tenant_id")
	writeJSON(w, http.StatusOK, h.Service.Store.List(status, tenant))
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/evolution/proposals/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	p := h.Service.Store.Get(id)
	if p == nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) handleProposalAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/evolution/proposals/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "review" {
		http.NotFound(w, r)
		return
	}
	defer r.Body.Close()
	var dec ReviewDecision
	if err := json.NewDecoder(r.Body).Decode(&dec); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, err := h.Service.Review(r.Context(), parts[0], dec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) handleStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.Service.Store.Stats())
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
