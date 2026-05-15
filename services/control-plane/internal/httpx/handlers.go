package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/forge-eng-fabric/services/control-plane/internal/auth"
	"github.com/forge-eng-fabric/services/control-plane/internal/events"
	"github.com/forge-eng-fabric/services/control-plane/internal/githubapp"
	"github.com/forge-eng-fabric/services/control-plane/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// API bundles dependencies needed by handlers.
type API struct {
	DB          *store.DB
	FGA         *auth.OpenFGAClient
	Pub         *events.KafkaPublisher
	GitHubRepos *githubapp.Service
}

func NewAPI(db *store.DB, fga *auth.OpenFGAClient, pub *events.KafkaPublisher, githubRepos *githubapp.Service) *API {
	return &API{DB: db, FGA: fga, Pub: pub, GitHubRepos: githubRepos}
}

// Routes mounts the v1 API on r.
func (a *API) Routes(r chi.Router) {
	r.Route("/v1", func(r chi.Router) {
		r.Get("/tenants", a.listTenants)
		r.Post("/tenants", a.createTenant)

		r.Get("/tenants/{tenantID}/business-units", a.listBUs)
		r.Post("/tenants/{tenantID}/business-units", a.createBU)
		r.Get("/tenants/{tenantID}/feature-flags", a.getTenantFeatureFlags)
		r.Patch("/tenants/{tenantID}/feature-flags", a.patchTenantFeatureFlags)

		r.Get("/business-units", a.listAllBUs)
		r.Get("/business-units/{buID}/workspaces", a.listWorkspaces)
		r.Post("/business-units/{buID}/workspaces", a.createWorkspace)
		r.Get("/business-units/{buID}/members", a.listBUMembers)

		r.Get("/platform/users", a.listPlatformUsers)

		r.Get("/workspaces", a.listAllWorkspaces)
		r.Get("/workspaces/{workspaceID}", a.getWorkspace)
		r.Patch("/workspaces/{workspaceID}", a.updateWorkspace)
		r.Delete("/workspaces/{workspaceID}", a.archiveWorkspace)
		r.Post("/workspaces/{workspaceID}/github/installations", a.createGitHubInstallation)
		r.Get("/workspaces/{workspaceID}/github/repositories", a.listGitHubRepositories)
	})
}

// --- helpers ------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	writeJSON(w, status, map[string]string{
		"code":           code,
		"message":        msg,
		"correlation_id": CorrelationFromContext(r.Context()),
	})
}

func parseJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func principal(r *http.Request) *auth.Principal {
	if p, ok := auth.FromContext(r.Context()); ok {
		return p
	}
	return &auth.Principal{Subject: "anonymous"}
}

func hasRole(p *auth.Principal, role string) bool {
	for _, x := range p.Roles {
		if x == role {
			return true
		}
	}
	return false
}

// --- tenants ------------------------------------------------------------

type tenantCreate struct {
	Name string `json:"name"`
}

func (a *API) listTenants(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.ListTenants(r.Context())
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

func (a *API) createTenant(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if !hasRole(p, "platform-admin") {
		writeErr(w, r, 403, "forbidden", "platform-admin role required")
		return
	}
	var req tenantCreate
	if err := parseJSON(r, &req); err != nil || req.Name == "" {
		writeErr(w, r, 400, "bad_request", "name is required")
		return
	}
	t, err := a.DB.CreateTenant(r.Context(), req.Name, p.Subject)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	_ = a.FGA.Write(r.Context(), "user:"+p.Subject, "admin", "tenant:"+t.ID.String())
	writeJSON(w, 201, t)
}

// --- feature flags ------------------------------------------------------

// resolveTenantID tries to parse tenantID as a UUID first; if that fails it
// looks up the tenant by name so callers can use either the UUID or the slug.
func (a *API) resolveTenantID(ctx context.Context, tenantID string) (uuid.UUID, error) {
	if id, err := uuid.Parse(tenantID); err == nil {
		return id, nil
	}
	// Fall back: look up by name/slug.
	rows, err := a.DB.ListTenants(ctx)
	if err != nil {
		return uuid.UUID{}, err
	}
	for _, t := range rows {
		if t.Name == tenantID {
			return t.ID, nil
		}
	}
	return uuid.UUID{}, errors.New("tenant not found: " + tenantID)
}

func (a *API) getTenantFeatureFlags(w http.ResponseWriter, r *http.Request) {
	tid, err := a.resolveTenantID(r.Context(), chi.URLParam(r, "tenantID"))
	if err != nil {
		writeErr(w, r, 404, "not_found", err.Error())
		return
	}
	flags, err := a.DB.GetTenantFeatureFlags(r.Context(), tid)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	writeJSON(w, 200, flags)
}

func (a *API) patchTenantFeatureFlags(w http.ResponseWriter, r *http.Request) {
	tid, err := a.resolveTenantID(r.Context(), chi.URLParam(r, "tenantID"))
	if err != nil {
		writeErr(w, r, 404, "not_found", err.Error())
		return
	}
	var patch map[string]bool
	if err := parseJSON(r, &patch); err != nil {
		writeErr(w, r, 400, "bad_request", "body must be a JSON object of flag:bool pairs")
		return
	}
	updated, err := a.DB.PatchTenantFeatureFlags(r.Context(), tid, patch)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	writeJSON(w, 200, updated)
}

// --- business units -----------------------------------------------------

type buCreate struct {
	Name string `json:"name"`
}

func (a *API) listBUs(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeErr(w, r, 400, "bad_request", "invalid tenantID")
		return
	}
	rows, err := a.DB.ListBUs(r.Context(), tenantID)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

func (a *API) createBU(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeErr(w, r, 400, "bad_request", "invalid tenantID")
		return
	}
	ok, err := a.FGA.Check(r.Context(), "user:"+p.Subject, "admin", "tenant:"+tenantID.String())
	if err != nil {
		writeErr(w, r, 500, "fga_error", err.Error())
		return
	}
	if !ok {
		writeErr(w, r, 403, "forbidden", "tenant admin required")
		return
	}
	var req buCreate
	if err := parseJSON(r, &req); err != nil || req.Name == "" {
		writeErr(w, r, 400, "bad_request", "name is required")
		return
	}
	bu, err := a.DB.CreateBU(r.Context(), tenantID, req.Name, p.Subject)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	_ = a.FGA.Write(r.Context(), "tenant:"+tenantID.String(), "tenant", "business_unit:"+bu.ID.String())
	_ = a.FGA.Write(r.Context(), "user:"+p.Subject, "admin", "business_unit:"+bu.ID.String())
	writeJSON(w, 201, bu)
}

// --- workspaces ---------------------------------------------------------

type wsCreate struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Owners      []string `json:"owners"`
}

type wsUpdate struct {
	Name        *string   `json:"name,omitempty"`
	Description *string   `json:"description,omitempty"`
	Owners      *[]string `json:"owners,omitempty"`
}

type githubInstallationCreate struct {
	InstallationID string   `json:"installation_id"`
	GitHubAccount  string   `json:"github_account"`
	Scopes         []string `json:"scopes"`
}

type githubRepositoriesResponse struct {
	InstallationID string                 `json:"installation_id"`
	GitHubAccount  string                 `json:"github_account"`
	CacheHit       bool                   `json:"cache_hit"`
	Repositories   []githubapp.Repository `json:"repositories"`
}

func (a *API) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	buID, err := uuid.Parse(chi.URLParam(r, "buID"))
	if err != nil {
		writeErr(w, r, 400, "bad_request", "invalid buID")
		return
	}
	rows, err := a.DB.ListWorkspaces(r.Context(), &buID)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

type buMember struct {
	Subject    string   `json:"subject"`
	Workspaces []string `json:"workspaces"`
}

func (a *API) listPlatformUsers(w http.ResponseWriter, r *http.Request) {
	prefix := strings.TrimSpace(r.URL.Query().Get("q"))
	users, err := a.DB.ListPlatformUsers(r.Context(), prefix, 200)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"users": users})
}

func (a *API) listBUMembers(w http.ResponseWriter, r *http.Request) {
	buID, err := uuid.Parse(chi.URLParam(r, "buID"))
	if err != nil {
		writeErr(w, r, 400, "bad_request", "invalid buID")
		return
	}
	rows, err := a.DB.ListWorkspaces(r.Context(), &buID)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	bySubject := map[string][]string{}
	order := []string{}
	for _, ws := range rows {
		for _, owner := range ws.Owners {
			owner = strings.TrimSpace(owner)
			if owner == "" {
				continue
			}
			if _, seen := bySubject[owner]; !seen {
				order = append(order, owner)
			}
			bySubject[owner] = append(bySubject[owner], ws.Name)
		}
	}
	members := make([]buMember, 0, len(order))
	for _, subject := range order {
		members = append(members, buMember{Subject: subject, Workspaces: bySubject[subject]})
	}
	writeJSON(w, 200, map[string]any{"members": members})
}

func (a *API) listAllBUs(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.ListAllBUs(r.Context())
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	p := principal(r)
	if hasRole(p, "platform-admin") {
		writeJSON(w, 200, rows)
		return
	}
	// FGA-scope: only BUs the principal can view. Mirrors listAllWorkspaces.
	out := []store.BusinessUnit{}
	for _, b := range rows {
		ok, err := a.FGA.Check(r.Context(), "user:"+p.Subject, "can_view", "business_unit:"+b.ID.String())
		if err != nil {
			writeErr(w, r, 500, "fga_error", err.Error())
			return
		}
		if ok {
			out = append(out, b)
		}
	}
	writeJSON(w, 200, out)
}

func (a *API) listAllWorkspaces(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.ListWorkspaces(r.Context(), nil)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	p := principal(r)
	if hasRole(p, "platform-admin") {
		writeJSON(w, 200, rows)
		return
	}

	// Best-effort OpenFGA scoping: list all workspaces and filter by can_view.
	out := []store.Workspace{}
	for _, ws := range rows {
		ok, err := a.FGA.Check(r.Context(), "user:"+p.Subject, "can_view", "workspace:"+ws.ID.String())
		if err != nil {
			// If FGA errors, fail closed for safety.
			writeErr(w, r, 500, "fga_error", err.Error())
			return
		}
		if ok {
			out = append(out, ws)
		}
	}
	writeJSON(w, 200, out)
}

func (a *API) createWorkspace(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	buID, err := uuid.Parse(chi.URLParam(r, "buID"))
	if err != nil {
		writeErr(w, r, 400, "bad_request", "invalid buID")
		return
	}
	ok, err := a.FGA.Check(r.Context(), "user:"+p.Subject, "admin", "business_unit:"+buID.String())
	if err != nil {
		writeErr(w, r, 500, "fga_error", err.Error())
		return
	}
	if !ok && !hasRole(p, "tenant-admin") && !hasRole(p, "platform-admin") {
		writeErr(w, r, 403, "forbidden", "BU admin required")
		return
	}
	var req wsCreate
	if err := parseJSON(r, &req); err != nil || req.Name == "" || len(req.Owners) == 0 {
		writeErr(w, r, 400, "bad_request", "name and owners are required")
		return
	}
	ws, err := a.DB.CreateWorkspace(r.Context(), buID, req.Name, req.Description, req.Owners, p.Subject)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	_ = a.FGA.Write(r.Context(), "business_unit:"+buID.String(), "business_unit", "workspace:"+ws.ID.String())
	for _, o := range req.Owners {
		_ = a.FGA.Write(r.Context(), "user:"+o, "owner", "workspace:"+ws.ID.String())
		if p.Username != "" && o == p.Username && p.Subject != p.Username {
			_ = a.FGA.Write(r.Context(), "user:"+p.Subject, "owner", "workspace:"+ws.ID.String())
		}
	}

	// Emit event (best-effort; failures don't roll back the API call).
	_ = a.Pub.Publish(r.Context(), events.CloudEvent{
		Type:          "com.forge.workspace.created.v1",
		Source:        "forge://service/control-plane",
		Subject:       "workspace/" + ws.ID.String(),
		TenantID:      ws.TenantID.String(),
		WorkspaceID:   ws.ID.String(),
		Actor:         "user:" + p.Subject,
		CorrelationID: CorrelationFromContext(r.Context()),
		Time:          time.Now().UTC(),
		Data: map[string]any{
			"workspace_id":     ws.ID,
			"tenant_id":        ws.TenantID,
			"business_unit_id": ws.BusinessUnitID,
			"name":             ws.Name,
			"description":      ws.Description,
			"owners":           ws.Owners,
			"created_at":       ws.CreatedAt,
			"created_by":       p.Subject,
		},
	})
	writeJSON(w, 201, ws)
}

func (a *API) getWorkspace(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeErr(w, r, 400, "bad_request", "invalid workspaceID")
		return
	}
	ws, err := a.DB.GetWorkspace(r.Context(), id)
	if err != nil {
		writeErr(w, r, 404, "not_found", err.Error())
		return
	}
	p := principal(r)
	ok, _ := a.FGA.Check(r.Context(), "user:"+p.Subject, "can_view", "workspace:"+id.String())
	if !ok && !hasRole(p, "platform-admin") {
		writeErr(w, r, 403, "forbidden", "no view access")
		return
	}
	writeJSON(w, 200, ws)
}

func (a *API) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeErr(w, r, 400, "bad_request", "invalid workspaceID")
		return
	}
	p := principal(r)
	ok, _ := a.FGA.Check(r.Context(), "user:"+p.Subject, "can_edit", "workspace:"+id.String())
	if !ok && !hasRole(p, "platform-admin") {
		writeErr(w, r, 403, "forbidden", "no edit access")
		return
	}
	var req wsUpdate
	if err := parseJSON(r, &req); err != nil {
		writeErr(w, r, 400, "bad_request", "invalid body")
		return
	}
	// If owners is provided, ensure it won't leave the workspace without owners.
	if req.Owners != nil {
		if len(*req.Owners) == 0 {
			writeErr(w, r, 409, "conflict", "operation would remove the last owner; transfer ownership before removing")
			return
		}
	}

	ws, err := a.DB.UpdateWorkspace(r.Context(), id, req.Name, req.Description, req.Owners)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	_ = a.Pub.Publish(r.Context(), events.CloudEvent{
		Type:          "com.forge.workspace.updated.v1",
		Source:        "forge://service/control-plane",
		Subject:       "workspace/" + ws.ID.String(),
		TenantID:      ws.TenantID.String(),
		WorkspaceID:   ws.ID.String(),
		Actor:         "user:" + p.Subject,
		CorrelationID: CorrelationFromContext(r.Context()),
		Time:          time.Now().UTC(),
		Data: map[string]any{
			"workspace_id": ws.ID,
			"tenant_id":    ws.TenantID,
			"changes":      req,
			"updated_at":   time.Now().UTC(),
			"updated_by":   p.Subject,
		},
	})
	writeJSON(w, 200, ws)
}

func (a *API) archiveWorkspace(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeErr(w, r, 400, "bad_request", "invalid workspaceID")
		return
	}
	p := principal(r)
	ok, _ := a.FGA.Check(r.Context(), "user:"+p.Subject, "can_admin", "workspace:"+id.String())
	if !ok && !hasRole(p, "platform-admin") {
		writeErr(w, r, 403, "forbidden", "owner required")
		return
	}
	ws, err := a.DB.ArchiveWorkspace(r.Context(), id)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	_ = a.Pub.Publish(r.Context(), events.CloudEvent{
		Type:          "com.forge.workspace.archived.v1",
		Source:        "forge://service/control-plane",
		Subject:       "workspace/" + ws.ID.String(),
		TenantID:      ws.TenantID.String(),
		WorkspaceID:   ws.ID.String(),
		Actor:         "user:" + p.Subject,
		CorrelationID: CorrelationFromContext(r.Context()),
		Time:          time.Now().UTC(),
		Data: map[string]any{
			"workspace_id": ws.ID,
			"tenant_id":    ws.TenantID,
			"archived_at":  ws.ArchivedAt,
			"archived_by":  p.Subject,
		},
	})
	w.WriteHeader(204)
}

func (a *API) createGitHubInstallation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeErr(w, r, 400, "bad_request", "invalid workspaceID")
		return
	}
	p := principal(r)
	ok, _ := a.FGA.Check(r.Context(), "user:"+p.Subject, "can_admin", "workspace:"+id.String())
	if !ok && !hasRole(p, "platform-admin") {
		writeErr(w, r, 403, "forbidden", "workspace owner required")
		return
	}
	var req githubInstallationCreate
	if err := parseJSON(r, &req); err != nil || req.InstallationID == "" || req.GitHubAccount == "" {
		writeErr(w, r, 400, "bad_request", "installation_id and github_account are required")
		return
	}
	installation, err := a.DB.CreateGitHubInstallation(r.Context(), id, req.InstallationID, req.GitHubAccount, req.Scopes, p.Subject)
	if err != nil {
		writeErr(w, r, 500, "db_error", err.Error())
		return
	}
	_ = a.Pub.Publish(r.Context(), events.CloudEvent{
		Type:          "com.forge.github.connected.v1",
		Source:        "forge://service/control-plane",
		Subject:       "github-installation/" + installation.InstallationID,
		TenantID:      installation.TenantID.String(),
		WorkspaceID:   installation.WorkspaceID.String(),
		Actor:         "user:" + p.Subject,
		CorrelationID: CorrelationFromContext(r.Context()),
		Time:          time.Now().UTC(),
		Data: map[string]any{
			"installation_id": installation.InstallationID,
			"github_account":  installation.GitHubAccount,
			"scopes":          installation.Scopes,
			"connected_at":    installation.ConnectedAt,
			"connected_by":    installation.ConnectedBy,
		},
	})
	writeJSON(w, 201, installation)
}

func (a *API) listGitHubRepositories(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeErr(w, r, 400, "bad_request", "invalid workspaceID")
		return
	}
	p := principal(r)
	ok, _ := a.FGA.Check(r.Context(), "user:"+p.Subject, "can_view", "workspace:"+id.String())
	if !ok && !hasRole(p, "platform-admin") {
		writeErr(w, r, 403, "forbidden", "no view access")
		return
	}
	if a.GitHubRepos == nil {
		writeErr(w, r, 503, "github_unavailable", "github repository service is not configured")
		return
	}
	installation, err := a.DB.LatestGitHubInstallation(r.Context(), id)
	if err != nil {
		writeErr(w, r, 404, "not_found", err.Error())
		return
	}
	repos, cacheHit, err := a.GitHubRepos.ListRepositories(r.Context(), githubapp.Installation{
		InstallationID: installation.InstallationID,
		GitHubAccount:  installation.GitHubAccount,
	}, refreshRequested(r))
	if err != nil {
		writeErr(w, r, 502, "github_error", err.Error())
		return
	}
	writeJSON(w, 200, githubRepositoriesResponse{
		InstallationID: installation.InstallationID,
		GitHubAccount:  installation.GitHubAccount,
		CacheHit:       cacheHit,
		Repositories:   repos,
	})
}

func refreshRequested(r *http.Request) bool {
	refresh := r.URL.Query().Get("refresh")
	return refresh == "1" || refresh == "true"
}

var _ = context.Background
