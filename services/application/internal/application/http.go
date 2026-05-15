package application

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Handler is the HTTP adapter for Service. It mounts:
//
//	POST   /v1/workspaces/{ws}/apps
//	GET    /v1/workspaces/{ws}/apps
//	GET    /v1/apps/{id}
//	PATCH  /v1/apps/{id}
//	POST   /v1/apps/{id}:archive
//	POST   /v1/apps/{id}:restore
//	DELETE /v1/apps/{id}
//	POST   /v1/apps/{id}/design-system:swap        (when DesignSystem != nil)
//	PATCH  /v1/apps/{id}/design-system/overrides   (when DesignSystem != nil)
//	POST   /v1/webhooks/design-system              (when DesignSystem != nil)
type Handler struct {
	Service      *Service
	DesignSystem *DesignSystemService
}

func NewHandler(s *Service) *Handler { return &Handler{Service: s} }

func (h *Handler) WithDesignSystem(d *DesignSystemService) *Handler {
	h.DesignSystem = d
	return h
}

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1/workspaces/", h.handleWorkspaceRoute)
	mux.HandleFunc("/v1/apps/", h.handleAppRoute)
	if h.DesignSystem != nil {
		mux.HandleFunc("/v1/webhooks/design-system", h.handleDesignSystemWebhook)
	}
}

func (h *Handler) handleWorkspaceRoute(w http.ResponseWriter, r *http.Request) {
	// Expect /v1/workspaces/{ws}/apps
	rest := strings.TrimPrefix(r.URL.Path, "/v1/workspaces/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[1] != "apps" {
		http.NotFound(w, r)
		return
	}
	workspaceID := parts[0]
	caller := callerFromRequest(r)
	switch r.Method {
	case http.MethodGet:
		includeArchived := r.URL.Query().Get("include_archived") == "true"
		apps, err := h.Service.List(r.Context(), caller, workspaceID, includeArchived)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"apps": apps})
	case http.MethodPost:
		var input CreateInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, errors.New("bad_payload"))
			return
		}
		app, err := h.Service.Create(r.Context(), caller, workspaceID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, app)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleAppRoute(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/apps/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	caller := callerFromRequest(r)
	// design-system-catalog routes nest under /v1/apps/{id}/design-system... .
	// They are mounted on the same /v1/apps/ prefix so we dispatch here when
	// the wire-up has provided a DesignSystem service.
	if h.DesignSystem != nil {
		if strings.HasSuffix(id, "/design-system:swap") {
			h.handleDesignSystemSwap(w, r, caller, strings.TrimSuffix(id, "/design-system:swap"))
			return
		}
		if strings.HasSuffix(id, "/design-system/overrides") {
			h.handleDesignSystemOverrides(w, r, caller, strings.TrimSuffix(id, "/design-system/overrides"))
			return
		}
	}
	// Action endpoints use the `:action` suffix convention.
	if strings.HasSuffix(id, ":archive") {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		appID := strings.TrimSuffix(id, ":archive")
		app, err := h.Service.Archive(r.Context(), caller, appID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, app)
		return
	}
	if strings.HasSuffix(id, ":restore") {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		appID := strings.TrimSuffix(id, ":restore")
		app, err := h.Service.Restore(r.Context(), caller, appID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, app)
		return
	}
	switch r.Method {
	case http.MethodGet:
		app, err := h.Service.Get(r.Context(), caller, id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, app)
	case http.MethodPatch:
		var input PatchInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, errors.New("bad_payload"))
			return
		}
		app, err := h.Service.Patch(r.Context(), caller, id, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, app)
	case http.MethodDelete:
		result, err := h.Service.Delete(r.Context(), caller, id)
		if errors.Is(err, ErrAppHasLiveArtefacts) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":   "app_has_live_artefacts",
				"blocked": result.Blocked,
			})
			return
		}
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result.Deleted)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func callerFromRequest(r *http.Request) Caller {
	principal := r.Header.Get("X-Forge-Principal")
	if principal == "" {
		principal = "anonymous"
	}
	corr := r.Header.Get("X-Correlation-ID")
	if corr == "" {
		corr = newID()
	}
	return Caller{Principal: principal, CorrelationID: corr}
}

// handleDesignSystemSwap routes POST /v1/apps/{id}/design-system:swap.
func (h *Handler) handleDesignSystemSwap(w http.ResponseWriter, r *http.Request, caller Caller, appID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in SwapInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, errors.New("bad_payload"))
		return
	}
	app, pr, err := h.DesignSystem.Swap(r.Context(), caller, appID, in)
	if err != nil {
		writeDesignSystemError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"app": app, "swap_pr": pr})
}

// handleDesignSystemOverrides routes PATCH /v1/apps/{id}/design-system/overrides.
func (h *Handler) handleDesignSystemOverrides(w http.ResponseWriter, r *http.Request, caller Caller, appID string) {
	if r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in OverridesInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, errors.New("bad_payload"))
		return
	}
	app, err := h.DesignSystem.PatchOverrides(r.Context(), caller, appID, in)
	if err != nil {
		writeDesignSystemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

// handleDesignSystemWebhook receives the portal-bundle PR merge notification.
func (h *Handler) handleDesignSystemWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Action      string `json:"action"`
		PullRequest struct {
			URL    string `json:"html_url"`
			Merged bool   `json:"merged"`
		} `json:"pull_request"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, errors.New("bad_payload"))
		return
	}
	if body.Action != "closed" || !body.PullRequest.Merged || body.PullRequest.URL == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	caller := callerFromRequest(r)
	if caller.Principal == "anonymous" {
		caller.Principal = "system:portal-bundle-webhook"
	}
	app, err := h.DesignSystem.HandleSwapPRMerged(r.Context(), caller, body.PullRequest.URL)
	if err != nil {
		writeDesignSystemError(w, err)
		return
	}
	if app == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeDesignSystemError maps the design-system errors to canonical HTTP
// codes. Defined alongside writeError so the design-system endpoints can use
// the same writer style.
func writeDesignSystemError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrDesignSystemNotApproved):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "design_system_not_approved", "message": err.Error()})
	case errors.Is(err, ErrDesignSystemNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "design_system_not_found"})
	case errors.Is(err, ErrUnknownComponent):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "unknown_component", "message": err.Error()})
	case errors.Is(err, ErrLayoutTokenOverride):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "layout_token_override_forbidden"})
	case errors.Is(err, ErrAppRepoMissing):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "app_repo_missing", "message": "App has no portal-bundle repo in repo_links"})
	case errors.Is(err, ErrSwapPRNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "swap_pr_not_found"})
	case errors.Is(err, ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "missing_app_owner"})
	default:
		writeError(w, err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var targetErr *TargetValidationError
	switch {
	case errors.As(err, &targetErr):
		body := map[string]any{
			"error":   targetErr.Code.Error(),
			"phase":   targetErr.Phase,
			"value":   targetErr.Value,
			"allowed": []string{"required", "optional", "opt-in", "skipped"},
		}
		writeJSON(w, http.StatusUnprocessableEntity, body)
	case errors.Is(err, ErrAppNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "app_not_found"})
	case errors.Is(err, ErrSlugConflict):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "app_slug_conflict"})
	case errors.Is(err, ErrSlugInvalid):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "app_slug_invalid"})
	case errors.Is(err, ErrSlugReserved):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "app_slug_reserved"})
	case errors.Is(err, ErrSystemManaged):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "system_managed_app"})
	case errors.Is(err, ErrAppArchived):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "app_archived"})
	case errors.Is(err, ErrAppWorkspaceMismatch):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "app_workspace_mismatch"})
	case errors.Is(err, ErrMissingOwners):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "missing_owners"})
	case errors.Is(err, ErrMissingName):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "missing_name"})
	case errors.Is(err, ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
}
