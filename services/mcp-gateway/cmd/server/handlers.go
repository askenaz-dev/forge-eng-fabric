package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// invocationEvent is the CloudEvents-shaped envelope the gateway emits
// on every MCP invocation, regardless of outcome. The per-asset
// observability spec ingests this into the platform's metrics roll-up.
type invocationEvent struct {
	Specversion        string         `json:"specversion"`
	ID                 string         `json:"id"`
	Source             string         `json:"source"`
	Type               string         `json:"type"`
	Subject            string         `json:"subject"`
	Time               string         `json:"time"`
	Datacontenttype    string         `json:"datacontenttype"`
	ForgeTenantID      string         `json:"forgetenantid"`
	ForgeWorkspaceID   string         `json:"forgeworkspaceid"`
	ForgeActor         string         `json:"forgeactor"`
	ForgeCorrelationID string         `json:"forgecorrelationid"`
	Data               map[string]any `json:"data"`
}

const eventTypeMCPInvocation = "com.forge.mcp.invocation.v1"

// invokeHandler implements POST /v1/gw/mcp/{asset_id}. It resolves the
// asset's active_surface from the registry, evaluates policy + budget +
// rate limits, brokers credentials for external MCPs, injects the signed
// identity headers, and proxies the call (HTTP or SSE) through the relay.
func (s *server) invokeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	assetID := chiURLParam(r, "assetID")
	if assetID == "" {
		writeJSONErr(w, 400, "missing_asset_id", "asset id is required in the path")
		return
	}
	principal, _ := r.Context().Value(ctxKeyPrincipal).(string)
	tenant, _ := r.Context().Value(ctxKeyTenant).(string)
	workspace, _ := r.Context().Value(ctxKeyWorkspace).(string)
	correlationID, _ := r.Context().Value(ctxKeyCorrelationID).(string)
	toolName := r.URL.Query().Get("tool")
	if toolName == "" {
		toolName = r.Header.Get("X-Forge-MCP-Tool")
	}

	// Rate limit first — cheapest gate, refuses overload before we hit
	// the registry or policy engine.
	if rl := s.rateLimiter.Allow(r.Context(), tenant, workspace); !rl.Allowed {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(time.Until(rl.ResetAt).Seconds())))
		s.metrics.Errors.Inc(tenant, assetID, rl.Reason)
		s.metrics.observe(tenant, workspace, assetID, "unknown", "rate_limited", time.Since(start))
		writeJSONErr(w, 429, "rate_limit_exceeded", rl.Reason)
		return
	}

	asset, err := s.registry.GetAsset(r.Context(), assetID)
	if errors.Is(err, errAssetNotFound) {
		s.metrics.Errors.Inc(tenant, assetID, "asset_not_found")
		writeJSONErr(w, 404, "asset_not_found", "no asset for id="+assetID)
		return
	}
	if err != nil {
		s.metrics.Errors.Inc(tenant, assetID, "registry_unreachable")
		writeJSONErr(w, 502, "registry_unreachable", err.Error())
		return
	}
	if asset.LifecycleState != "approved" {
		s.metrics.Errors.Inc(tenant, assetID, "asset_not_approved")
		writeJSONErr(w, 409, "asset_not_approved", "asset must be approved; got "+asset.LifecycleState)
		return
	}

	// Drift detection hook: on every tools/list request (session start) we
	// compare the live tool list against the registry cache asynchronously.
	if toolName == "tools/list" && s.driftDetector != nil {
		endpoint, _ := s.resolveOutboundURL(asset, "")
		sessionID := correlationID
		if sessionID == "" {
			sessionID = newUUID()
		}
		go func() {
			ctx2, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := s.driftDetector.CheckOnSessionStart(ctx2, assetID, endpoint, sessionID); err != nil {
				log.Printf("[drift] check error asset=%s: %v", assetID, err)
			}
		}()
	}

	if asset.Type != "mcp" {
		s.metrics.Errors.Inc(tenant, assetID, "asset_type_mismatch")
		writeJSONErr(w, 400, "asset_type_mismatch", "asset is type="+asset.Type+", not mcp")
		return
	}
	if asset.TenantID != "" && asset.TenantID != tenant {
		s.metrics.Errors.Inc(tenant, assetID, "cross_tenant_denied")
		writeJSONErr(w, 403, "cross_tenant_denied", "asset belongs to a different tenant")
		return
	}

	source := "internal"
	if asset.Provenance == "external" {
		source = "external_proxy"
		// Tool allowlist enforcement happens here, before the credential
		// broker, so we never fetch a secret for a request that will be
		// denied anyway.
		if len(asset.Allowlist) > 0 && toolName != "" && !contains(asset.Allowlist, toolName) {
			s.metrics.Errors.Inc(tenant, assetID, "tool_not_allowlisted")
			s.emitInvocationEvent(r.Context(), invocationEvent{
				Type: eventTypeMCPInvocation, ForgeTenantID: tenant, ForgeWorkspaceID: workspace,
				ForgeActor: principal, ForgeCorrelationID: correlationID,
				Subject: "asset/" + assetID,
				Data: map[string]any{
					"asset_id": assetID, "tool_name": toolName, "source": source,
					"outcome": "tool_not_allowlisted",
				},
			})
			writeJSONErr(w, 403, "tool_not_allowlisted", "tool="+toolName+" not in tenant's allowlist for "+assetID)
			return
		}
	}

	// OPA policy decision.
	dec, err := s.policy.Evaluate(r.Context(), PolicyInput{
		Action: "mcp.invoke", Principal: principal, TenantID: tenant, WorkspaceID: workspace,
		AssetID: assetID, ToolName: toolName, Provenance: asset.Provenance, CorrelationID: correlationID,
	})
	if err != nil {
		s.metrics.Errors.Inc(tenant, assetID, "policy_unreachable")
		writeJSONErr(w, 503, "policy_unreachable", err.Error())
		return
	}
	if !dec.Allow {
		s.metrics.Errors.Inc(tenant, assetID, "policy_denied")
		writeJSONErr(w, 403, "policy_denied", dec.Reason)
		return
	}

	// Tenant-budget probe.
	bd, _ := s.budget.Check(r.Context(), tenant, "mcp", 0)
	if !bd.Allow {
		s.metrics.BudgetBlocks.Inc(tenant, assetID)
		writeJSONErr(w, 429, "budget_exhausted", bd.Reason)
		return
	}

	// Resolve the outbound URL. Internal assets carry the gateway's own
	// endpoint in active_surface; for those we still need a backend URL.
	// In the design, the registry's `mcp_internal_endpoint` (per asset)
	// holds it. For now, internal assets read from `metadata.endpoint_url`
	// — operators populate this at asset publish time.
	target, err := s.resolveOutboundURL(asset, toolName)
	if err != nil {
		s.metrics.Errors.Inc(tenant, assetID, "endpoint_resolution_failed")
		writeJSONErr(w, 502, "endpoint_resolution_failed", err.Error())
		return
	}

	// Sign identity headers.
	payload := IdentityPayload{
		Principal: principal, Tenant: tenant, Workspace: workspace,
		CorrelationID: correlationID, Timestamp: time.Now().Unix(),
	}
	sig, kid, ts, err := s.km.Sign(payload)
	if err != nil {
		s.metrics.Errors.Inc(tenant, assetID, "identity_sign_failed")
		writeJSONErr(w, 500, "identity_sign_failed", err.Error())
		return
	}
	payload.Timestamp = ts

	outReq, _ := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	for k, vv := range r.Header {
		if shouldCopyInboundHeader(k) {
			for _, v := range vv {
				outReq.Header.Add(k, v)
			}
		}
	}
	applyIdentityHeaders(outReq, payload, sig, kid)

	// Credential broker for external MCPs: resolve the credential ref
	// only at the moment of the outbound call; redact from any error
	// returned to the caller.
	if asset.Provenance == "external" && asset.CredentialRef != "" {
		secret, ferr := s.secrets.Fetch(r.Context(), asset.CredentialRef)
		if ferr != nil {
			s.metrics.Errors.Inc(tenant, assetID, "credential_fetch_failed")
			// Do NOT include the credential ref in the error response.
			writeJSONErr(w, 502, "credential_fetch_failed", "could not resolve tenant credential for external MCP")
			return
		}
		outReq.Header.Set("authorization", "Bearer "+string(secret))
		// Wipe local copy ASAP.
		for i := range secret {
			secret[i] = 0
		}
	}

	res, ferr := s.relay.Forward(r.Context(), outReq, w, s.cfg.SSEBufferSize)
	outcome := "ok"
	if ferr != nil || res.Status/100 != 2 {
		outcome = "error"
		s.metrics.Errors.Inc(tenant, assetID, fmt.Sprintf("upstream_%d", res.Status))
	}
	s.metrics.observe(tenant, workspace, assetID, source, outcome, time.Since(start))
	s.emitInvocationEvent(r.Context(), invocationEvent{
		Type: eventTypeMCPInvocation, ForgeTenantID: tenant, ForgeWorkspaceID: workspace,
		ForgeActor: principal, ForgeCorrelationID: correlationID,
		Subject: "asset/" + assetID,
		Data: map[string]any{
			"asset_id":  assetID,
			"tool_name": toolName,
			"source":    source,
			"outcome":   outcome,
			"upstream_status": res.Status,
			"streaming":       res.Streaming,
			"bytes_out":       res.BytesOut,
			"latency_ms":      time.Since(start).Milliseconds(),
		},
	})
}

// catalogHandler implements GET /v1/gw/mcp/catalog. Lists approved MCPs
// visible to the calling tenant with their provenance, active_surface,
// and how_to blocks, so consumers can build a runtime tool list without
// re-querying the registry directly.
func (s *server) catalogHandler(w http.ResponseWriter, r *http.Request) {
	tenant, _ := r.Context().Value(ctxKeyTenant).(string)
	assets, err := s.registry.ListApprovedMCPs(r.Context(), tenant)
	if err != nil {
		writeJSONErr(w, 502, "registry_unreachable", err.Error())
		return
	}
	type item struct {
		AssetID       string         `json:"asset_id"`
		Provenance    string         `json:"provenance"`
		ActiveSurface map[string]any `json:"active_surface"`
		HowTo         map[string]any `json:"how_to"`
	}
	out := make([]item, 0, len(assets))
	for _, a := range assets {
		out = append(out, item{
			AssetID:       a.ID,
			Provenance:    a.Provenance,
			ActiveSurface: a.ActiveSurface,
			HowTo:         a.HowTo,
		})
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}

// resolveOutboundURL returns the full URL to forward the request to.
// External MCPs have `endpoint` populated from external_mcp_endpoint;
// internal MCPs use the endpoint stored in active_surface.metadata or
// fall back to `endpoint_url` in the asset's metadata.
func (s *server) resolveOutboundURL(a AssetView, toolName string) (string, error) {
	endpoint := a.Endpoint
	if endpoint == "" {
		// Active surface may carry a relative path; in that case we'd
		// need a backend lookup. For internal assets the operator stores
		// the actual endpoint in active_surface.upstream_endpoint
		// (free-form field).
		if a.ActiveSurface != nil {
			if v, ok := a.ActiveSurface["upstream_endpoint"].(string); ok && v != "" {
				endpoint = v
			}
		}
	}
	if endpoint == "" {
		return "", errors.New("no upstream endpoint configured for asset")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("bad endpoint url: %w", err)
	}
	if toolName != "" {
		q := u.Query()
		q.Set("tool", toolName)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// emitInvocationEvent ships an invocation event to Kafka via the
// configured publisher. Fire-and-forget; transient failures are logged
// but do not propagate to the caller.
func (s *server) emitInvocationEvent(ctx context.Context, e invocationEvent) {
	e.Specversion = "1.0"
	e.Source = "forge://service/mcp-gateway"
	e.Datacontenttype = "application/json"
	if e.Time == "" {
		e.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if e.ID == "" {
		e.ID = newUUID()
	}
	body, _ := json.Marshal(e)
	_ = s.publisher.Publish(ctx, e.Type, []byte(e.ForgeTenantID), body)
}

// shouldCopyInboundHeader is the small filter that strips auth / hop-by-
// hop headers from the request before we replace them with the gateway's
// own.
func shouldCopyInboundHeader(name string) bool {
	switch strings.ToLower(name) {
	case "authorization", "host", "content-length":
		return false
	}
	return !isHopByHop(name)
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// writeJSONErr is the canonical error response shape.
func writeJSONErr(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "message": message})
}

// drainAndClose is a small helper used by integration tests + relay; here
// only to suppress unused-import warnings on iterations during dev.
func drainAndClose(r io.ReadCloser) {
	if r == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r)
	_ = r.Close()
}
