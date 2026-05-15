package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// JSONRPCRequest is the wire envelope shared by every A2A operation. We
// validate the basic shape at the gateway boundary so the inner Forward
// step always sees a well-formed body.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

const (
	methodTasksSend          = "tasks/send"
	methodTasksGet           = "tasks/get"
	methodTasksCancel        = "tasks/cancel"
	methodTasksSendSubscribe = "tasks/sendSubscribe"
)

var validA2AMethods = map[string]struct{}{
	methodTasksSend:          {},
	methodTasksGet:           {},
	methodTasksCancel:        {},
	methodTasksSendSubscribe: {},
}

// invocationEvent matches the canonical CloudEvents shape used elsewhere
// in the platform.
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

const eventTypeA2AInvocation = "com.forge.a2a.invocation.v1"

// invokeHandler implements POST /v1/gw/a2a/{assetID}. The handler
// distinguishes inbound (X-Forge-Partner-Auth set) from outbound (Bearer
// JWT identity) and runs the appropriate flow. Both flows converge on
// the same relay + audit emit path.
func (s *server) invokeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	assetID := urlParam(r, "assetID")
	if assetID == "" {
		writeJSONErr(w, 400, "missing_asset_id", "asset id is required in the path")
		return
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSONErr(w, 400, "invalid_body", err.Error())
		return
	}
	_ = r.Body.Close()

	// Parse the JSON-RPC envelope for routing + dispatch decisions.
	var env JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &env); err != nil {
		writeJSONErr(w, 400, "invalid_jsonrpc", "request body must be JSON-RPC 2.0")
		return
	}
	if env.JSONRPC != "2.0" {
		writeJSONErr(w, 400, "invalid_jsonrpc", "jsonrpc field must be 2.0")
		return
	}
	if _, ok := validA2AMethods[env.Method]; !ok {
		writeJSONErr(w, 400, "unknown_method", "method must be one of tasks/send, tasks/get, tasks/cancel, tasks/sendSubscribe")
		return
	}

	// Branch on auth direction.
	if partnerAuth := r.Header.Get(HeaderPartnerAuth); partnerAuth != "" {
		s.handleInbound(w, r, start, assetID, bodyBytes, env, partnerAuth)
		return
	}
	s.handleOutbound(w, r, start, assetID, bodyBytes, env)
}

// handleOutbound is the internal-caller → external-A2A flow.
func (s *server) handleOutbound(w http.ResponseWriter, r *http.Request, start time.Time, assetID string, bodyBytes []byte, env JSONRPCRequest) {
	principal, _ := r.Context().Value(ctxKeyPrincipal).(string)
	tenant, _ := r.Context().Value(ctxKeyTenant).(string)
	workspace, _ := r.Context().Value(ctxKeyWorkspace).(string)
	correlationID, _ := r.Context().Value(ctxKeyCorrelationID).(string)

	if rl := s.rateLimiter.Allow(r.Context(), tenant, workspace); !rl.Allowed {
		s.metrics.TasksFailed.Inc(tenant, assetID, "outbound", rl.Reason)
		writeJSONErr(w, 429, "rate_limit_exceeded", rl.Reason)
		return
	}

	asset, err := s.registry.GetAsset(r.Context(), assetID)
	if errors.Is(err, errAssetNotFound) {
		writeJSONErr(w, 404, "asset_not_found", "no asset for id="+assetID)
		return
	}
	if err != nil {
		writeJSONErr(w, 502, "registry_unreachable", err.Error())
		return
	}
	if asset.LifecycleState != "approved" {
		writeJSONErr(w, 409, "asset_not_approved", "asset must be approved; got "+asset.LifecycleState)
		return
	}
	if asset.Type != "agent" {
		writeJSONErr(w, 400, "asset_type_mismatch", "asset is type="+asset.Type+", not agent")
		return
	}
	source := "internal"
	if asset.Provenance == "external" {
		source = "external_proxy"
		// Task-allowlist enforcement happens before credential resolution.
		if len(asset.TaskAllowlist) > 0 && env.Method == methodTasksSend && !asset.AllowsTaskType(env) {
			s.metrics.TasksFailed.Inc(tenant, assetID, source, "task_not_allowlisted")
			writeJSONErr(w, 403, "task_not_allowlisted", "task type not in tenant's allowlist")
			return
		}
	}

	dec, err := s.policy.Evaluate(r.Context(), PolicyInput{
		Action: "a2a.task", Principal: principal, PrincipalKind: "service",
		TenantID: tenant, WorkspaceID: workspace, AssetID: assetID,
		TaskType: env.Method, Provenance: asset.Provenance, CorrelationID: correlationID,
	})
	if err != nil {
		writeJSONErr(w, 503, "policy_unreachable", err.Error())
		return
	}
	if !dec.Allow {
		s.metrics.TasksFailed.Inc(tenant, assetID, source, "policy_denied")
		writeJSONErr(w, 403, "policy_denied", dec.Reason)
		return
	}

	if bd, _ := s.budget.Check(r.Context(), tenant, "a2a", 0); !bd.Allow {
		s.metrics.BudgetBlocks.Inc(tenant, assetID)
		writeJSONErr(w, 429, "budget_exhausted", bd.Reason)
		return
	}

	target, err := s.resolveOutboundURL(asset)
	if err != nil {
		writeJSONErr(w, 502, "endpoint_resolution_failed", err.Error())
		return
	}

	payload := IdentityPayload{
		Principal: principal, PrincipalKind: "service", Tenant: tenant, Workspace: workspace,
		CorrelationID: correlationID, Timestamp: time.Now().Unix(),
	}
	sig, kid, ts, err := s.km.Sign(payload)
	if err != nil {
		writeJSONErr(w, 500, "identity_sign_failed", err.Error())
		return
	}
	payload.Timestamp = ts

	outReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(bodyBytes))
	outReq.Header.Set("content-type", "application/json")
	applyIdentityHeaders(outReq, payload, sig, kid)

	if asset.Provenance == "external" && asset.CredentialRef != "" {
		secret, ferr := s.secrets.Fetch(r.Context(), asset.CredentialRef)
		if ferr != nil {
			s.metrics.TasksFailed.Inc(tenant, assetID, source, "credential_fetch_failed")
			writeJSONErr(w, 502, "credential_fetch_failed", "could not resolve tenant credential for external A2A agent")
			return
		}
		outReq.Header.Set("authorization", "Bearer "+string(secret))
		for i := range secret {
			secret[i] = 0
		}
	}

	res, ferr := s.relay.Forward(r.Context(), outReq, w, s.cfg.SSEBufferSize)
	outcome := "ok"
	if ferr != nil || res.Status/100 != 2 {
		outcome = "error"
	}
	s.metrics.observe(tenant, workspace, assetID, source, env.Method, outcome, time.Since(start))
	s.emitInvocationEvent(r.Context(), invocationEvent{
		Type: eventTypeA2AInvocation, ForgeTenantID: tenant, ForgeWorkspaceID: workspace,
		ForgeActor: principal, ForgeCorrelationID: correlationID,
		Subject: "asset/" + assetID,
		Data: map[string]any{
			"asset_id":  assetID,
			"method":    env.Method,
			"source":    source,
			"outcome":   outcome,
			"streaming": res.Streaming,
			"bytes_out": res.BytesOut,
		},
	})
}

// handleInbound is the external-partner → internal-agent flow.
func (s *server) handleInbound(w http.ResponseWriter, r *http.Request, start time.Time, assetID string, bodyBytes []byte, env JSONRPCRequest, partnerAuthHeader string) {
	// We need the tenant id to look up the partner. For inbound calls
	// the tenant is identified by a separate header (`X-Forge-Inbound-Tenant`)
	// supplied by the partner's caller config. The integration tests rely
	// on this; production deployments terminate mTLS + look up the
	// tenant from the partner's certificate identity.
	tenant := r.Header.Get("X-Forge-Inbound-Tenant")
	if tenant == "" {
		writeJSONErr(w, 400, "missing_tenant", "X-Forge-Inbound-Tenant header is required for inbound A2A")
		return
	}
	correlationID := r.Header.Get(HeaderCorrelationID)
	if correlationID == "" {
		correlationID = newUUID()
	}

	if rl := s.rateLimiter.Allow(r.Context(), tenant, "inbound"); !rl.Allowed {
		s.metrics.TasksFailed.Inc(tenant, assetID, "inbound_external", rl.Reason)
		writeJSONErr(w, 429, "rate_limit_exceeded", rl.Reason)
		return
	}

	partner, err := s.partners.Authenticate(tenant, partnerAuthHeader, bodyBytes)
	if err != nil {
		s.metrics.TasksFailed.Inc(tenant, assetID, "inbound_external", "unknown_partner")
		writeJSONErr(w, 401, "unknown_partner", err.Error())
		return
	}
	if !partner.AllowsAsset(assetID) {
		s.metrics.TasksFailed.Inc(tenant, assetID, "inbound_external", "asset_not_allowlisted")
		writeJSONErr(w, 403, "asset_not_allowlisted", "partner not permitted to invoke "+assetID)
		return
	}

	asset, err := s.registry.GetAsset(r.Context(), assetID)
	if errors.Is(err, errAssetNotFound) {
		writeJSONErr(w, 404, "asset_not_found", "no internal agent for id="+assetID)
		return
	}
	if err != nil {
		writeJSONErr(w, 502, "registry_unreachable", err.Error())
		return
	}
	if asset.LifecycleState != "approved" {
		writeJSONErr(w, 409, "asset_not_approved", "asset must be approved; got "+asset.LifecycleState)
		return
	}
	if asset.Provenance != "internal" {
		// We reject inbound calls targeted at an external asset record —
		// the gateway should not act as a relay-of-relay.
		writeJSONErr(w, 400, "invalid_inbound_target", "inbound calls must target an internal agent")
		return
	}

	dec, _ := s.policy.Evaluate(r.Context(), PolicyInput{
		Action: "a2a.task", Principal: partner.Name, PrincipalKind: "external_agent",
		TenantID: tenant, WorkspaceID: partner.WorkspaceID, AssetID: assetID,
		TaskType: env.Method, Provenance: "external_inbound", CorrelationID: correlationID,
	})
	if !dec.Allow {
		s.metrics.TasksFailed.Inc(tenant, assetID, "inbound_external", "policy_denied")
		writeJSONErr(w, 403, "policy_denied", dec.Reason)
		return
	}

	target, err := s.resolveOutboundURL(asset)
	if err != nil {
		writeJSONErr(w, 502, "endpoint_resolution_failed", err.Error())
		return
	}

	payload := IdentityPayload{
		Principal: partner.Name, PrincipalKind: "external_agent",
		Tenant: tenant, Workspace: partner.WorkspaceID,
		CorrelationID: correlationID, Timestamp: time.Now().Unix(),
	}
	sig, kid, ts, _ := s.km.Sign(payload)
	payload.Timestamp = ts

	outReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(bodyBytes))
	outReq.Header.Set("content-type", "application/json")
	applyIdentityHeaders(outReq, payload, sig, kid)

	res, ferr := s.relay.Forward(r.Context(), outReq, w, s.cfg.SSEBufferSize)
	outcome := "ok"
	if ferr != nil || res.Status/100 != 2 {
		outcome = "error"
	}
	s.metrics.observe(tenant, partner.WorkspaceID, assetID, "inbound_external", env.Method, outcome, time.Since(start))
	s.emitInvocationEvent(r.Context(), invocationEvent{
		Type: eventTypeA2AInvocation, ForgeTenantID: tenant, ForgeWorkspaceID: partner.WorkspaceID,
		ForgeActor: "external_partner:" + partner.Name, ForgeCorrelationID: correlationID,
		Subject: "asset/" + assetID,
		Data: map[string]any{
			"asset_id": assetID, "method": env.Method, "source": "inbound_external",
			"partner": partner.Name, "outcome": outcome, "streaming": res.Streaming,
		},
	})
}

// AllowsTaskType returns true when the JSON-RPC payload's task type is in
// the asset's allowlist. The wire shape A2A uses for task types is open;
// we look at `params.task.type` first, then fall back to `params.method`
// as a coarse-grained "all tasks/send tasks" gate.
func (a AssetView) AllowsTaskType(env JSONRPCRequest) bool {
	if len(a.TaskAllowlist) == 0 {
		return true
	}
	var params struct {
		Task struct {
			Type string `json:"type"`
		} `json:"task"`
	}
	_ = json.Unmarshal(env.Params, &params)
	want := params.Task.Type
	if want == "" {
		want = strings.TrimPrefix(env.Method, "tasks/")
	}
	for _, t := range a.TaskAllowlist {
		if t == want {
			return true
		}
	}
	return false
}

func (s *server) catalogHandler(w http.ResponseWriter, r *http.Request) {
	tenant, _ := r.Context().Value(ctxKeyTenant).(string)
	assets, err := s.registry.ListApprovedA2A(r.Context(), tenant)
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
		out = append(out, item{AssetID: a.ID, Provenance: a.Provenance, ActiveSurface: a.ActiveSurface, HowTo: a.HowTo})
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}

func (s *server) resolveOutboundURL(a AssetView) (string, error) {
	endpoint := a.Endpoint
	if endpoint == "" && a.ActiveSurface != nil {
		if v, ok := a.ActiveSurface["upstream_endpoint"].(string); ok && v != "" {
			endpoint = v
		}
	}
	if endpoint == "" {
		return "", errors.New("no upstream endpoint configured for asset")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("bad endpoint url: %w", err)
	}
	return u.String(), nil
}

func (s *server) emitInvocationEvent(ctx context.Context, e invocationEvent) {
	e.Specversion = "1.0"
	e.Source = "forge://service/a2a-gateway"
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

func writeJSONErr(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
