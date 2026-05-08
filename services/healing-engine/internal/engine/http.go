package engine

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Handler exposes the engine over HTTP.
type Handler struct{ Service *Service }

// NewHandler wraps a service.
func NewHandler(s *Service) *Handler { return &Handler{Service: s} }

// Mount installs routes.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /v1/healing/trigger", h.handleTrigger)
	mux.HandleFunc("POST /v1/envelopes", h.handleSetEnvelope)
	mux.HandleFunc("GET /v1/envelopes", h.handleListEnvelopes)
	mux.HandleFunc("POST /v1/actions", h.handleSetAction)
	mux.HandleFunc("POST /v1/kill-switch", h.handleKillSwitch)
	mux.HandleFunc("GET /v1/kill-switch", h.handleGetKillSwitch)
	mux.HandleFunc("POST /v1/actions/promote", h.handlePromote)
	mux.HandleFunc("GET /v1/decisions/", h.handleDecisions)
}

func (h *Handler) handleTrigger(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var in IncidentInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d, err := h.Service.Trigger(r.Context(), in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) handleSetEnvelope(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var e Envelope
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.Service.Store.SetEnvelope(&e)
	writeJSON(w, http.StatusCreated, e)
}

func (h *Handler) handleListEnvelopes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.Service.Store.ListEnvelopes())
}

func (h *Handler) handleSetAction(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var a Action
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.Service.Store.SetAction(&a)
	writeJSON(w, http.StatusCreated, a)
}

func (h *Handler) handleKillSwitch(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req struct {
		WorkspaceID string `json:"workspace_id"`
		Active      bool   `json:"active"`
		Actor       string `json:"actor"`
		Reason      string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.Service.Store.SetKillSwitch(req.WorkspaceID, req.Active)
	_ = h.Service.Sink.Emit(newEvent("", req.WorkspaceID, "healing.kill_switch.toggled.v1",
		"workspace/"+req.WorkspaceID, map[string]any{
			"active": req.Active,
			"actor":  req.Actor,
			"reason": req.Reason,
		}))
	writeJSON(w, http.StatusOK, map[string]bool{"active": req.Active})
}

func (h *Handler) handleGetKillSwitch(w http.ResponseWriter, r *http.Request) {
	ws := r.URL.Query().Get("workspace_id")
	writeJSON(w, http.StatusOK, map[string]bool{"active": h.Service.Store.KillSwitch(ws)})
}

func (h *Handler) handlePromote(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req PromotionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.Service.PromoteAction(req); err != nil {
		switch {
		case errors.Is(err, ErrPromotionPrerequisites):
			http.Error(w, err.Error(), http.StatusPreconditionFailed)
		case errors.Is(err, ErrPromotionApproval):
			writeJSON(w, http.StatusUnprocessableEntity,
				map[string]string{"code": "approval_missing", "message": err.Error()})
		case errors.Is(err, ErrActionNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "promoted"})
}

func (h *Handler) handleDecisions(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/decisions/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, h.Service.Store.ListDecisions(id))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
