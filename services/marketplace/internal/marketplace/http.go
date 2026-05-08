package marketplace

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// Handler exposes marketplace operations via HTTP.
type Handler struct{ Service *Service }

func NewHandler(s *Service) *Handler { return &Handler{Service: s} }

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /v1/marketplace", h.search)
	mux.HandleFunc("POST /v1/marketplace", h.publish)
	mux.HandleFunc("GET /v1/marketplace/", h.routeListing)
	mux.HandleFunc("POST /v1/marketplace/", h.routeListing)
	mux.HandleFunc("POST /v1/marketplace/install", h.install)
	mux.HandleFunc("GET /v1/installs", h.listInstalls)
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tags := []string{}
	if t := q.Get("tags"); t != "" {
		tags = strings.Split(t, ",")
	}
	filters := SearchFilters{
		TenantID:       q.Get("tenant_id"),
		WorkspaceID:    q.Get("workspace_id"),
		Visibility:     ast.Visibility(q.Get("visibility")),
		Tags:           tags,
		Criticality:    ast.Criticality(q.Get("criticality")),
		Text:           q.Get("q"),
		IncludePending: q.Get("include_pending") == "1",
	}
	writeJSON(w, http.StatusOK, map[string]any{"listings": h.Service.Search(r.Context(), filters)})
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	listing, err := h.Service.Publish(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, listing)
}

func (h *Handler) routeListing(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/marketplace/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	if parts[0] == "install" {
		h.install(w, r)
		return
	}
	id := parts[0]
	tenant := r.URL.Query().Get("tenant_id")
	if len(parts) == 1 && r.Method == http.MethodGet {
		listing, err := h.Service.GetListing(tenant, id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, listing)
		return
	}
	if len(parts) == 2 && parts[1] == "approve" && r.Method == http.MethodPost {
		h.approve(w, r, id)
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request, id string) {
	defer r.Body.Close()
	var req ApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.ListingID = id
	listing, err := h.Service.Approve(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, listing)
}

func (h *Handler) install(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	install, err := h.Service.Install(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, install)
}

func (h *Handler) listInstalls(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant_id")
	workspace := r.URL.Query().Get("workspace_id")
	writeJSON(w, http.StatusOK, map[string]any{"installs": h.Service.ListInstalls(tenant, workspace)})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrListingNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrCertificationPrereq):
		http.Error(w, err.Error(), http.StatusPreconditionFailed)
	case errors.Is(err, ErrInstallNotPermitted), errors.Is(err, ErrCrossTenantInvisible):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, ErrApprovalNotPending):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
