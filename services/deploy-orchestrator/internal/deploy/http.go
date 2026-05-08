package deploy

import (
	"encoding/json"
	"errors"
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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /v1/deployments", h.create)
	mux.HandleFunc("GET /v1/deployments", h.list)
	mux.HandleFunc("GET /v1/deployments/", h.routeDeploymentID)
	mux.HandleFunc("POST /v1/deployments/", h.routeDeploymentID)
	mux.HandleFunc("GET /v1/assets/", h.routeAssets)
}

func (h *Handler) routeDeploymentID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/deployments/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.get(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	sub := parts[1]
	switch sub {
	case "rollback":
		h.rollback(w, r, id)
	case "stream":
		h.stream(w, r, id)
	case "events":
		h.events(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) routeAssets(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/assets/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[1] != "deployments" {
		http.NotFound(w, r)
		return
	}
	assetID := parts[0]
	env := r.URL.Query().Get("env")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")
	deps, next := h.Service.Store.AssetDeployments(assetID, env, limit, cursor)
	writeJSON(w, http.StatusOK, map[string]any{"deployments": deps, "next_cursor": next})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := h.Service.Deploy(r.Context(), &req)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if !resp.Created {
		status = http.StatusOK
	}
	writeJSON(w, status, resp)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	env := r.URL.Query().Get("env")
	writeJSON(w, http.StatusOK, map[string]any{"deployments": h.Service.Store.List(workspaceID, env)})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, id string) {
	d, ok := h.Service.Store.Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deployment":           d,
		"events":               h.Service.Store.Events(id),
		"policy_evaluations":   h.Service.Store.PolicyEvals(id),
		"image_verifications":  h.Service.Store.ImageVerifications(id),
		"rollbacks":            h.Service.Store.Rollbacks(id),
	})
}

func (h *Handler) events(w http.ResponseWriter, _ *http.Request, id string) {
	if _, ok := h.Service.Store.Get(id); !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": h.Service.Store.Events(id)})
}

func (h *Handler) rollback(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req RollbackRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	req.Manual = true
	resp, err := h.Service.Rollback(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// stream implements `GET /v1/deployments/{id}/stream` SSE per the spec.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := h.Service.Store.Get(id); !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	// Replay history first so a late subscriber does not miss earlier
	// stages — supports the `Operator observes a live deployment`
	// scenario.
	for _, ev := range h.Service.Store.Events(id) {
		writeSSE(w, ev)
		flusher.Flush()
	}
	ch, cleanup := h.Service.Store.Subscribe(id)
	defer cleanup()
	notify := r.Context().Done()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, ev)
			flusher.Flush()
		case <-notify:
			return
		}
	}
}

func writeSSE(w http.ResponseWriter, ev DeploymentEvent) {
	b, _ := json.Marshal(ev)
	fmt.Fprintf(w, "event: stage.%s\ndata: %s\n\n", ev.Outcome, string(b))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrDeploymentNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrCrossWorkspace):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, ErrRuntimeRevoked):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, ErrPreviousRevision):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, ErrNonReversible):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
