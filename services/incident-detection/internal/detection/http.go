package detection

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler wires HTTP endpoints to the service.
type Handler struct {
	Service *Service
}

// NewHandler builds an HTTP handler.
func NewHandler(s *Service) *Handler { return &Handler{Service: s} }

// Mount installs routes on the given mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /v1/detect/prometheus", h.handlePrometheus)
	mux.HandleFunc("POST /v1/detect/cloud-monitoring", h.handleCloudMonitoring)
	mux.HandleFunc("POST /v1/detect/loki", h.handleLoki)
	mux.HandleFunc("POST /v1/detect/internal", h.handleInternal)
	mux.HandleFunc("POST /v1/incidents/declare", h.handleDeclare)
	mux.HandleFunc("POST /v1/incidents/", h.handleIncidentAction)
	mux.HandleFunc("GET /v1/incidents", h.handleList)
	mux.HandleFunc("GET /v1/incidents/", h.handleGet)
}

func (h *Handler) handlePrometheus(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var payload PrometheusWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	newCount, dedup, _ := h.Service.IngestPrometheus(payload)
	writeJSON(w, http.StatusAccepted, map[string]int{"created": newCount, "deduplicated": dedup})
}

func (h *Handler) handleCloudMonitoring(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var payload CloudMonitoringWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	created, err := h.Service.IngestCloudMonitoring(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"created": created})
}

func (h *Handler) handleLoki(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var payload LokiWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	newCount, dedup, _ := h.Service.IngestLoki(payload)
	writeJSON(w, http.StatusAccepted, map[string]int{"created": newCount, "deduplicated": dedup})
}

func (h *Handler) handleInternal(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var payload InternalEvent
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	created, err := h.Service.IngestInternal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"created": created})
}

func (h *Handler) handleDeclare(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req DeclareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	inc, err := h.Service.Declare(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, inc)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	status := Status(r.URL.Query().Get("status"))
	filter := func(i *Incident) bool {
		if status != "" && i.Status != status {
			return false
		}
		return true
	}
	writeJSON(w, http.StatusOK, h.Service.Store.List(filter))
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/incidents/")
	id = strings.TrimSuffix(id, "/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	inc := h.Service.Store.Get(id)
	if inc == nil {
		http.Error(w, "not_found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, inc)
}

func (h *Handler) handleIncidentAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/incidents/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	id, action := parts[0], parts[1]
	switch action {
	case "resolve":
		inc, err := h.Service.Resolve(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, inc)
	default:
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
