package sdlc

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type Handler struct {
	Service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{Service: service} }

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /metrics", h.metrics)
	mux.HandleFunc("GET /v1/initiatives", h.list)
	mux.HandleFunc("POST /v1/initiatives", h.create)
	mux.HandleFunc("GET /v1/initiatives/", h.routeInitiative)
	mux.HandleFunc("POST /v1/initiatives/", h.routeInitiative)
	mux.HandleFunc("POST /v1/events", h.ingestEvent)
}

func (h *Handler) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(h.Service.Metrics()))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"initiatives": h.Service.ListInitiatives(r.URL.Query().Get("workspace_id"))})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req CreateInitiativeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	initiative, err := h.Service.CreateInitiative(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, initiative)
}

func (h *Handler) routeInitiative(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/initiatives/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		initiative, err := h.Service.GetInitiative(id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, initiative)
		return
	}
	if len(parts) == 4 && parts[1] == "phase" && parts[3] == "complete" {
		h.completePhase(w, r, id, Phase(parts[2]))
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) completePhase(w http.ResponseWriter, r *http.Request, id string, phase Phase) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req CompletePhaseRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	initiative, err := h.Service.CompletePhase(r.Context(), id, phase, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, initiative)
}

func (h *Handler) ingestEvent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var event BusEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	initiative, err := h.Service.HandleEvent(r.Context(), event)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, initiative)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInitiativeNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrPhaseMismatch):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, ErrInvalidOverride):
		http.Error(w, err.Error(), http.StatusForbidden)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
