package traceability

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	Service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{Service: service} }

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /metrics", h.metrics)
	mux.HandleFunc("GET /v1/traceability/", h.getTraceability)
	mux.HandleFunc("POST /v1/events", h.ingestEvent)
	mux.HandleFunc("POST /v1/backfill/audit-log", h.backfillAuditLog)
}

func (h *Handler) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(h.Service.Metrics()))
}

func (h *Handler) getTraceability(w http.ResponseWriter, r *http.Request) {
	openSpecID := strings.TrimPrefix(r.URL.Path, "/v1/traceability/")
	if openSpecID == "" {
		http.NotFound(w, r)
		return
	}
	depth := 4
	if raw := r.URL.Query().Get("depth"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			depth = parsed
		}
	}
	writeJSON(w, http.StatusOK, h.Service.TraceabilityForOpenSpec(openSpecID, depth))
}

func (h *Handler) ingestEvent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var event BusEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.Service.HandleEvent(r.Context(), event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (h *Handler) backfillAuditLog(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var request BackfillRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.Service.BackfillAuditLog(r.Context(), request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
