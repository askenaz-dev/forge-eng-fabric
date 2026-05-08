package onboarding

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	Service *Service
}

func NewHandler(s *Service) *Handler { return &Handler{Service: s} }

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/metrics", h.handleMetrics)
	mux.HandleFunc("/v1/templates", h.handleTemplates)
	mux.HandleFunc("/v1/pipeline-gates", h.handlePipelineGates)
	mux.HandleFunc("/v1/onboarding", h.handleCollection)
	mux.HandleFunc("/v1/onboarding/", h.handleItem)
}

func (h *Handler) handleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		filter := RequestFilter{
			WorkspaceID: r.URL.Query().Get("workspace_id"),
			Status:      Status(r.URL.Query().Get("status")),
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"requests": h.Service.Store.List(filter)})
		return
	case http.MethodPost:
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.RequestedBy == "" {
		req.RequestedBy = r.Header.Get("X-Forge-Principal")
	}
	if req.RequestedBy == "" {
		req.RequestedBy = "unknown"
	}
	out, err := h.Service.Submit(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(out)
}

func (h *Handler) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	templates, err := h.Service.ListTemplates(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"templates": templates})
}

func (h *Handler) handlePipelineGates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pr, _ := strconv.Atoi(firstNonEmpty(r.URL.Query().Get("pr_number"), r.URL.Query().Get("pr")))
	filter := GateResultFilter{
		WorkspaceID:  r.URL.Query().Get("workspace_id"),
		RepoFullName: r.URL.Query().Get("repo"),
		PRNumber:     pr,
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"results": h.Service.Store.GateResults(filter)})
}

func (h *Handler) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(h.Service.Store.PrometheusMetrics()))
}

func (h *Handler) handleItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/onboarding/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(id, "/events") {
		h.handleSSE(w, r, strings.TrimSuffix(id, "/events"))
		return
	}
	if strings.HasSuffix(id, "/timeline") {
		h.handleTimeline(w, r, strings.TrimSuffix(id, "/timeline"))
		return
	}
	req, ok := h.Service.Store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(req)
}

func (h *Handler) handleTimeline(w http.ResponseWriter, _ *http.Request, id string) {
	if _, ok := h.Service.Store.Get(id); !ok {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(h.Service.Store.Events(id))
}

func (h *Handler) handleSSE(w http.ResponseWriter, r *http.Request, id string) {
	req, ok := h.Service.Store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Subscribe BEFORE replaying so we don't miss new events.
	ch, cleanup := h.Service.Store.Subscribe(id)
	defer cleanup()

	for _, ev := range h.Service.Store.Events(id) {
		writeSSE(w, "stage."+string(ev.Outcome), ev)
	}
	flusher.Flush()

	// Re-read status after replay; it may have transitioned in between.
	if cur, ok := h.Service.Store.Get(id); ok && (cur.Status == StatusCompleted || cur.Status == StatusFailed) {
		writeSSE(w, "terminal", cur)
		flusher.Flush()
		return
	}
	_ = req

	for {
		select {
		case ev, open := <-ch:
			if !open {
				return
			}
			writeSSE(w, "stage."+string(ev.Outcome), ev)
			flusher.Flush()
			if ev.Stage == "asset.register" && ev.Outcome == OutcomeCompleted {
				if cur, ok := h.Service.Store.Get(id); ok {
					writeSSE(w, "terminal", cur)
				}
				flusher.Flush()
				return
			}
			if ev.Outcome == OutcomeFailed {
				if cur, ok := h.Service.Store.Get(id); ok {
					writeSSE(w, "terminal", cur)
				}
				flusher.Flush()
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", string(b))
}
