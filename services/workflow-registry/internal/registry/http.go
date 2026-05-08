package registry

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Handler exposes the registry over HTTP.
type Handler struct {
	Service *Service
}

// NewHandler builds the handler.
func NewHandler(s *Service) *Handler { return &Handler{Service: s} }

// Mount installs routes.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /v1/workflows", h.list)
	mux.HandleFunc("POST /v1/workflows", h.create)
	mux.HandleFunc("GET /v1/workflows/", h.routeWorkflow)
	mux.HandleFunc("POST /v1/workflows/", h.routeWorkflow)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant_id")
	workspace := r.URL.Query().Get("workspace_id")
	writeJSON(w, http.StatusOK, map[string]any{"workflows": h.Service.ListWorkflows(tenant, workspace)})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	wf, err := h.Service.CreateWorkflow(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, wf)
}

func (h *Handler) routeWorkflow(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/workflows/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		wf, err := h.Service.GetWorkflow(id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, wf)
	case len(parts) == 2 && parts[1] == "versions" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"versions": h.Service.ListVersions(id)})
	case len(parts) == 2 && parts[1] == "versions" && r.Method == http.MethodPost:
		h.publish(w, r, id)
	case len(parts) == 3 && parts[1] == "versions" && r.Method == http.MethodGet:
		v, err := h.Service.GetVersion(id, parts[2])
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request, id string) {
	defer r.Body.Close()
	var req PublishVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.WorkflowID = id
	v, err := h.Service.PublishVersion(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrWorkflowNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrVersionAlreadyExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, ErrBreakingChange):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, ErrLintFailed), errors.Is(err, ErrSchemaValidationFailed), errors.Is(err, ErrInvalidVersion):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
