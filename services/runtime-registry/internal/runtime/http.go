package runtime

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type Handler struct {
	Service *Service
}

func NewHandler(s *Service) *Handler { return &Handler{Service: s} }

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /v1/runtimes", h.list)
	mux.HandleFunc("POST /v1/runtimes", h.register)
	mux.HandleFunc("GET /v1/runtimes/", h.routeRuntimeID)
	mux.HandleFunc("POST /v1/runtimes/", h.routeRuntimeID)
	mux.HandleFunc("DELETE /v1/runtimes/", h.routeRuntimeID)
	mux.HandleFunc("POST /v1/runtimes/provision", h.provision)
}

func (h *Handler) routeRuntimeID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/runtimes/")
	if path == "provision" {
		h.provision(w, r)
		return
	}
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
		case http.MethodDelete:
			h.destroy(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	sub := parts[1]
	switch sub {
	case "preflight":
		h.preflight(w, r, id)
	case "revoke":
		h.revoke(w, r, id)
	case "verify":
		h.verify(w, r, id)
	case "verifications":
		h.verifications(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	writeJSON(w, http.StatusOK, map[string]any{"runtimes": h.Service.List(workspaceID)})
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rt, err := h.Service.Register(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, RegisterResponse{Runtime: rt})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, id string) {
	rt, err := h.Service.Get(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rt)
}

func (h *Handler) destroy(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.Service.Destroy(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) preflight(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var hints PreflightHints
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&hints); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	res, err := h.Service.RunPreflight(r.Context(), id, hints)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rt, err := h.Service.Revoke(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rt)
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var hints VerifyHints
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&hints); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	principal := r.Header.Get("x-principal")
	report, err := h.Service.RunVerify(r.Context(), id, principal, hints)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) verifications(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.Service.Get(id); err != nil {
		writeError(w, err)
		return
	}
	reports := h.Service.Store.Verifications(id)
	writeJSON(w, http.StatusOK, map[string]any{"verifications": reports})
}

func (h *Handler) provision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req ProvisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := h.Service.Provision(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrRuntimeNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrCrossWorkspace):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, ErrRuntimeRevoked):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, ErrStateBackendMissing):
		http.Error(w, err.Error(), http.StatusPreconditionFailed)
	case errors.Is(err, ErrDeploymentsPresent):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, ErrPlaintextForbidden):
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
