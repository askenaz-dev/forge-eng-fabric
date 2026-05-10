package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/forge-eng-fabric/pkg/workflow/dsl"
)

// Service binds the engine and provides high-level operations exposed via
// HTTP. It centralises namespace provisioning (one per Tenant) and metrics.
type Service struct {
	Engine  TemporalEngine
	Metrics *Metrics
}

// NewService creates a Service.
func NewService(engine TemporalEngine, metrics *Metrics) *Service {
	if metrics == nil {
		metrics = NewMetrics()
	}
	return &Service{Engine: engine, Metrics: metrics}
}

// Handler exposes the runtime over HTTP.
type Handler struct {
	Service *Service
}

// NewHandler builds a Handler.
func NewHandler(s *Service) *Handler { return &Handler{Service: s} }

// Mount installs the routes onto a mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /metrics", h.metrics)
	mux.HandleFunc("POST /v1/executions", h.start)
	mux.HandleFunc("GET /v1/executions", h.list)
	mux.HandleFunc("GET /v1/executions/", h.routeExecution)
	mux.HandleFunc("POST /v1/executions/", h.routeExecution)
}

func (h *Handler) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(h.Service.Metrics.Render()))
}

type startRequest struct {
	TenantID      string          `json:"tenant_id"`
	WorkspaceID   string          `json:"workspace_id"`
	WorkflowYAML  string          `json:"workflow_yaml,omitempty"`
	Workflow      json.RawMessage `json:"workflow,omitempty"`
	Inputs        map[string]any  `json:"inputs,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	DryRun        bool            `json:"dry_run,omitempty"`
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.WorkflowYAML == "" && len(req.Workflow) == 0 {
		http.Error(w, "workflow_required", http.StatusBadRequest)
		return
	}
	wf, err := dsl.Parse([]byte(req.WorkflowYAML))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	exec, err := h.Service.Engine.StartWorkflow(context.WithoutCancel(r.Context()), StartRequest{
		TenantID:      req.TenantID,
		WorkspaceID:   req.WorkspaceID,
		Workflow:      wf,
		Inputs:        req.Inputs,
		CorrelationID: req.CorrelationID,
		DryRun:        req.DryRun,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	h.Service.Metrics.IncStarted(string(exec.Status))
	writeJSON(w, http.StatusAccepted, exec)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant_id")
	workspace := r.URL.Query().Get("workspace_id")
	out := h.Service.Engine.ListExecutions(r.Context(), tenant, workspace)
	writeJSON(w, http.StatusOK, map[string]any{"executions": out})
}

func (h *Handler) routeExecution(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/executions/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	tenant := r.URL.Query().Get("tenant_id")
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		exec, err := h.Service.Engine.GetExecution(r.Context(), tenant, id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, exec)
		return
	}
	switch parts[1] {
	case "signal":
		h.signal(w, r, tenant, id)
	case "cancel":
		h.cancel(w, r, tenant, id)
	case "query":
		h.query(w, r, tenant, id, r.URL.Query().Get("step_id"))
	default:
		http.NotFound(w, r)
	}
}

type signalBody struct {
	Signal  string         `json:"signal"`
	Payload map[string]any `json:"payload,omitempty"`
}

func (h *Handler) signal(w http.ResponseWriter, r *http.Request, tenant, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var body signalBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	exec, err := h.Service.Engine.SignalWorkflow(r.Context(), SignalRequest{
		TenantID:    tenant,
		ExecutionID: id,
		Signal:      body.Signal,
		Payload:     body.Payload,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, exec)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request, tenant, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	exec, err := h.Service.Engine.CancelWorkflow(r.Context(), tenant, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, exec)
}

func (h *Handler) query(w http.ResponseWriter, r *http.Request, tenant, id, stepID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	out, err := h.Service.Engine.QueryWorkflow(r.Context(), QueryRequest{
		TenantID:    tenant,
		ExecutionID: id,
		StepID:      stepID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrCrossTenantAccess):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, ErrExecutionNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
