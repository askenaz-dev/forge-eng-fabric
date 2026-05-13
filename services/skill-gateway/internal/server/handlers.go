package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/forge-eng-fabric/services/skill-gateway/internal/auth"
	"github.com/forge-eng-fabric/services/skill-gateway/internal/events"
	"github.com/forge-eng-fabric/services/skill-gateway/internal/packagestore"
	"github.com/forge-eng-fabric/services/skill-gateway/internal/registry"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// --- /v1/gateway/assets -----------------------------------------------

type listedAsset struct {
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	Type          string            `json:"type"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	TrustLevel    string            `json:"trust_level"`
	EvalScores    map[string]any    `json:"eval_scores,omitempty"`
	PackageDigest *string           `json:"package_digest,omitempty"`
	HomepageURL   string            `json:"homepage_url,omitempty"`
	InstallHint   map[string]string `json:"install_hint,omitempty"`
}

func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	pat := patFromContext(r)
	if !pat.HasScope(auth.ScopeRead) && !pat.HasScope(auth.ScopeInstall) && !pat.HasScope(auth.ScopeInvoke) {
		http.Error(w, "missing_scope: gateway.read", 403)
		return
	}
	assetType := r.URL.Query().Get("type")
	assets, err := s.cfg.Registry.ListWorkspaceAssets(r.Context(), pat.AssumeWorkspaceID, assetType)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	out := make([]listedAsset, 0, len(assets))
	for _, a := range assets {
		if !a.Distribution.GatewayPublished {
			continue
		}
		if a.LifecycleState != "approved" || a.TrustLevel == "T0" {
			continue
		}
		if a.TenantID != pat.TenantID {
			continue
		}
		if !pat.AllowsAsset(a.ID) {
			continue
		}
		out = append(out, listedAsset{
			ID:            a.ID,
			Version:       a.Version,
			Type:          a.Type,
			Name:          a.Name,
			Description:   a.Description,
			TrustLevel:    a.TrustLevel,
			EvalScores:    a.EvalScores,
			PackageDigest: a.Distribution.PackageDigest,
			InstallHint: map[string]string{
				"cli": fmt.Sprintf("forge skills install %s@%s", a.Name, a.Version),
			},
		})
	}
	writeJSON(w, 200, out)
}

// --- /v1/gateway/assets/{id}/versions/{v}/package ---------------------

func (s *Server) handleDownloadPackage(w http.ResponseWriter, r *http.Request) {
	pat := patFromContext(r)
	if !pat.HasScope(auth.ScopeRead) && !pat.HasScope(auth.ScopeInstall) {
		http.Error(w, "missing_scope: gateway.install", 403)
		return
	}
	assetID := chi.URLParam(r, "assetID")
	version := chi.URLParam(r, "version")
	if !pat.AllowsAsset(assetID) {
		http.Error(w, "asset_not_in_allowlist", 403)
		return
	}
	asset, err := s.cfg.Registry.GetAssetVersion(r.Context(), assetID, version)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			http.Error(w, "package_not_found", 404)
			return
		}
		http.Error(w, err.Error(), 502)
		return
	}
	if !asset.Distribution.GatewayPublished || asset.Distribution.PackageDigest == nil {
		http.Error(w, "package_not_found", 404)
		return
	}
	bytesURI, ok := asset.Metadata["bytes_uri"].(string)
	if !ok || bytesURI == "" {
		// Fall back to building a deterministic key when metadata is missing.
		bytesURI = "s3://forge-packages/" + assetID + "/" + version + ".tar.zst"
	}

	// Large bundles: hand back a presigned URL when the store supports it.
	if presigned, _ := s.cfg.PackageStore.PresignedURL(r.Context(), bytesURI, 600); presigned != "" {
		w.Header().Set("X-Forge-Package-Digest", *asset.Distribution.PackageDigest)
		w.Header().Set("X-Forge-Asset-Version", version)
		http.Redirect(w, r, presigned, http.StatusFound)
		s.emitInstall(r, pat, asset)
		return
	}
	body, size, err := s.cfg.PackageStore.Get(r.Context(), bytesURI)
	if err != nil {
		if errors.Is(err, packagestore.ErrNotFound) {
			http.Error(w, "package_not_found", 404)
			return
		}
		http.Error(w, err.Error(), 502)
		return
	}
	defer body.Close()
	w.Header().Set("content-type", "application/zstd")
	w.Header().Set("X-Forge-Package-Digest", *asset.Distribution.PackageDigest)
	w.Header().Set("X-Forge-Asset-Version", version)
	w.Header().Set("Cache-Control", "max-age=300")
	if size > 0 {
		w.Header().Set("Content-Length", itoa(int(size)))
	}
	_, _ = io.Copy(w, body)
	s.emitInstall(r, pat, asset)
}

func (s *Server) emitInstall(r *http.Request, pat *auth.PAT, asset *registry.Asset) {
	digest := ""
	if asset.Distribution.PackageDigest != nil {
		digest = *asset.Distribution.PackageDigest
	}
	s.events.EmitInstalled(r.Context(), events.InstalledEvent{
		AssetID:       asset.ID,
		AssetVersion:  asset.Version,
		TenantID:      asset.TenantID.String(),
		DeveloperSub:  pat.DeveloperSub,
		Client:        r.Header.Get("X-Forge-Client"),
		PackageDigest: digest,
		CorrelationID: correlationFromContext(r),
	})
}

// --- /v1/gateway/mcp/{id} (HTTP + SSE reverse proxy) ------------------

func (s *Server) handleMCPProxy(w http.ResponseWriter, r *http.Request) {
	pat := patFromContext(r)
	if !pat.HasScope(auth.ScopeInvoke) {
		http.Error(w, "missing_scope: gateway.invoke", 403)
		return
	}
	assetID := chi.URLParam(r, "assetID")
	if !pat.AllowsAsset(assetID) {
		http.Error(w, "asset_not_in_allowlist", 403)
		return
	}
	asset, err := s.cfg.Registry.GetAssetVersion(r.Context(), assetID, latestVersion(r))
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if asset.Type != "mcp" || !asset.Distribution.GatewayPublished {
		http.Error(w, "remote_transport_unavailable", 409)
		return
	}
	transport, ok := asset.Metadata["remote_transport"].(map[string]any)
	if !ok || len(transport) == 0 {
		http.Error(w, "remote_transport_unavailable", 409)
		return
	}
	httpCfg, _ := transport["http"].(map[string]any)
	target, _ := httpCfg["upstream_url"].(string)
	if target == "" {
		http.Error(w, "remote_transport_unavailable", 409)
		return
	}
	upstream, err := url.Parse(target)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	started := time.Now()
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = upstream.Host
		req.Header.Set("X-Forge-Principal", pat.DeveloperSub)
		req.Header.Set("X-Forge-Tenant", pat.TenantID.String())
		req.Header.Set("X-Forge-Workspace", pat.AssumeWorkspaceID.String())
		req.Header.Set("X-Forge-Correlation-Id", correlationFromContext(r))
		// Strip our gateway auth before forwarding.
		req.Header.Del("Authorization")
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		s.events.EmitInvocation(r.Context(), events.InvocationEvent{
			Route:         "mcp",
			AssetID:       asset.ID,
			AssetVersion:  asset.Version,
			TenantID:      asset.TenantID.String(),
			WorkspaceID:   pat.AssumeWorkspaceID.String(),
			DeveloperSub:  pat.DeveloperSub,
			Outcome:       outcomeForStatus(resp.StatusCode),
			StatusCode:    resp.StatusCode,
			LatencyMS:     time.Since(started).Milliseconds(),
			CorrelationID: correlationFromContext(r),
		})
		return nil
	}
	proxy.ServeHTTP(w, r)
}

func latestVersion(r *http.Request) string {
	v := r.URL.Query().Get("version")
	if v == "" {
		return "latest"
	}
	return v
}

func outcomeForStatus(code int) string {
	if code/100 == 2 {
		return "success"
	}
	if code == http.StatusForbidden {
		return "forbidden"
	}
	return "error"
}

// --- /v1/gateway/a2a/{id} ---------------------------------------------

func (s *Server) handleA2A(w http.ResponseWriter, r *http.Request) {
	pat := patFromContext(r)
	if !pat.HasScope(auth.ScopeInvoke) {
		http.Error(w, "missing_scope: gateway.invoke", 403)
		return
	}
	assetID := chi.URLParam(r, "assetID")
	if !pat.AllowsAsset(assetID) {
		http.Error(w, "asset_not_in_allowlist", 403)
		return
	}
	asset, err := s.cfg.Registry.GetAssetVersion(r.Context(), assetID, latestVersion(r))
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if asset.Type != "agent" {
		http.Error(w, "not_an_agent_asset", 409)
		return
	}
	if asset.LifecycleState != "approved" {
		http.Error(w, "not_approved", 403)
		return
	}
	upstream, _ := asset.Metadata["a2a_upstream_url"].(string)
	if upstream == "" {
		http.Error(w, "a2a_upstream_unconfigured", 502)
		return
	}
	target, err := url.Parse(upstream)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	started := time.Now()
	body, _ := io.ReadAll(r.Body)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target.String(), strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("X-Forge-Principal", pat.DeveloperSub)
	req.Header.Set("X-Forge-Tenant", pat.TenantID.String())
	req.Header.Set("X-Forge-Workspace", pat.AssumeWorkspaceID.String())
	req.Header.Set("X-Forge-Correlation-Id", correlationFromContext(r))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
	s.events.EmitInvocation(r.Context(), events.InvocationEvent{
		Route:         "a2a",
		AssetID:       asset.ID,
		AssetVersion:  asset.Version,
		TenantID:      asset.TenantID.String(),
		WorkspaceID:   pat.AssumeWorkspaceID.String(),
		DeveloperSub:  pat.DeveloperSub,
		Outcome:       outcomeForStatus(resp.StatusCode),
		StatusCode:    resp.StatusCode,
		LatencyMS:     time.Since(started).Milliseconds(),
		CorrelationID: correlationFromContext(r),
	})
}

// --- /v1/gateway/tokens -----------------------------------------------

func (s *Server) handleIssueToken(w http.ResponseWriter, r *http.Request) {
	caller := patFromContext(r)
	var req auth.IssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	// Token issuance is allowed only for the caller's own developer_sub.
	if req.DeveloperSub == "" {
		req.DeveloperSub = caller.DeveloperSub
	}
	if req.DeveloperSub != caller.DeveloperSub {
		http.Error(w, "forbidden", 403)
		return
	}
	if req.TenantID == uuid.Nil {
		req.TenantID = caller.TenantID
	}
	req.CreatedBy = caller.DeveloperSub
	issued, err := s.auth.Issue(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 201, issued)
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	caller := patFromContext(r)
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		http.Error(w, "invalid token id", 400)
		return
	}
	if err := s.auth.Revoke(r.Context(), id, caller.DeveloperSub); err != nil {
		if errors.Is(err, auth.ErrUnknown) {
			http.Error(w, "not_found", 404)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- /v1/gateway/auth/device --- OIDC device-code stubs ----------------
// These delegate to the platform's Keycloak. The CLI starts with /device
// to obtain a user_code + verification_uri, then polls /token until the
// user completes the browser flow.

func (s *Server) handleAuthDevice(w http.ResponseWriter, r *http.Request) {
	// Real implementation calls keycloak's device endpoint and returns its
	// payload verbatim. For now expose a 501 with the contract so callers
	// can discover the missing piece in dev.
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error":      "not_implemented",
		"hint":       "wire to keycloak's /auth/realms/forge/protocol/openid-connect/auth/device endpoint",
		"see_design": "openspec/changes/add-developer-skill-gateway/design.md decision 6",
	})
}

func (s *Server) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "not_implemented",
		"hint":  "wire to keycloak's /auth/realms/forge/protocol/openid-connect/token (grant_type=urn:ietf:params:oauth:grant-type:device_code)",
	})
}
