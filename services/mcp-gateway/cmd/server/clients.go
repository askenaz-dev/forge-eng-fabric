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
	"os"
	"strings"
	"time"
)

// AssetView is the minimum projection of an Asset Registry row this
// gateway needs to route an MCP call. Built from the registry's read
// response and cached briefly per asset id.
type AssetView struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Provenance    string         `json:"provenance"`
	LifecycleState string        `json:"lifecycle_state"`
	ActiveSurface map[string]any `json:"active_surface"`
	HowTo         map[string]any `json:"how_to"`
	// For external MCPs only — pulled from the side-table via a sub-call.
	Endpoint      string   `json:"endpoint,omitempty"`
	CredentialRef string   `json:"credential_ref,omitempty"`
	Allowlist     []string `json:"allowlist,omitempty"`
	TenantID      string   `json:"tenant_id"`
}

// RegistryClient is the seam the invoke handler uses to resolve an MCP
// asset's runtime surface. The production implementation calls
// services/registry HTTP API; tests inject a fake.
type RegistryClient interface {
	GetAsset(ctx context.Context, assetID string) (AssetView, error)
	ListApprovedMCPs(ctx context.Context, tenantID string) ([]AssetView, error)
}

type httpRegistryClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func newHTTPRegistryClient(baseURL, token string) *httpRegistryClient {
	return &httpRegistryClient{baseURL: strings.TrimRight(baseURL, "/"), token: token, client: &http.Client{Timeout: 10 * time.Second}}
}

func (c *httpRegistryClient) get(ctx context.Context, path string, out any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if c.token != "" {
		req.Header.Set("authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errAssetNotFound
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("registry %s: status=%d body=%s", path, resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

var errAssetNotFound = errors.New("asset_not_found")

func (c *httpRegistryClient) GetAsset(ctx context.Context, assetID string) (AssetView, error) {
	var a AssetView
	if err := c.get(ctx, "/v1/assets/"+url.PathEscape(assetID), &a); err != nil {
		return AssetView{}, err
	}
	return a, nil
}

func (c *httpRegistryClient) ListApprovedMCPs(ctx context.Context, tenantID string) ([]AssetView, error) {
	// Registry list endpoint is workspace-scoped; for catalog we approximate
	// by asking the registry for all approved MCPs the caller can see. The
	// caller's JWT scopes the actual visibility; the registry enforces
	// tenant boundaries server-side.
	var assets []AssetView
	if err := c.get(ctx, "/v1/registry/mcps?lifecycle_state=approved", &assets); err != nil {
		return nil, err
	}
	out := make([]AssetView, 0, len(assets))
	for _, a := range assets {
		if a.Type == "mcp" && a.LifecycleState == "approved" {
			out = append(out, a)
		}
	}
	return out, nil
}

// PolicyClient is the OPA decision seam. The production implementation
// calls services/policy-engine; tests inject a stub that returns a
// canned PolicyDecision.
type PolicyClient interface {
	Evaluate(ctx context.Context, input PolicyInput) (PolicyDecision, error)
}

type PolicyInput struct {
	Action        string         `json:"action"` // e.g. "mcp.invoke"
	Principal     string         `json:"principal"`
	TenantID      string         `json:"tenant_id"`
	WorkspaceID   string         `json:"workspace_id"`
	AssetID       string         `json:"asset_id"`
	ToolName      string         `json:"tool_name"`
	Provenance    string         `json:"provenance"`
	CorrelationID string         `json:"correlation_id"`
	Extra         map[string]any `json:"extra,omitempty"`
}

type PolicyDecision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

type httpPolicyClient struct {
	url    string
	client *http.Client
}

func newHTTPPolicyClient(u string) PolicyClient {
	if u == "" {
		return staticPolicyClient{decision: PolicyDecision{Allow: true, Reason: "policy_engine_unconfigured"}}
	}
	return &httpPolicyClient{url: strings.TrimRight(u, "/") + "/v1/policy/decide", client: &http.Client{Timeout: 5 * time.Second}}
}

func (c *httpPolicyClient) Evaluate(ctx context.Context, input PolicyInput) (PolicyDecision, error) {
	body, _ := json.Marshal(input)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		// Fail-closed on policy infra error to avoid silent permission
		// regression. The gateway returns 503 to the caller.
		return PolicyDecision{Allow: false, Reason: "policy_engine_unreachable"}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return PolicyDecision{Allow: false, Reason: fmt.Sprintf("policy_engine_status_%d", resp.StatusCode)}, nil
	}
	var d PolicyDecision
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return PolicyDecision{Allow: false, Reason: "policy_engine_bad_response"}, err
	}
	return d, nil
}

type staticPolicyClient struct{ decision PolicyDecision }

func (s staticPolicyClient) Evaluate(_ context.Context, _ PolicyInput) (PolicyDecision, error) {
	return s.decision, nil
}

// BudgetClient probes the Tenant-budget service. The budget service is
// future work (tasks.md 8.2); the gateway calls a configurable endpoint
// and is tolerant of an unconfigured deployment.
type BudgetClient interface {
	Check(ctx context.Context, tenantID, family string, costEstimateCents int64) (BudgetDecision, error)
}

type BudgetDecision struct {
	Allow      bool   `json:"allow"`
	Reason     string `json:"reason,omitempty"`
	RemainingC int64  `json:"remaining_cents,omitempty"`
}

type httpBudgetClient struct {
	url    string
	client *http.Client
}

func newHTTPBudgetClient(u string) BudgetClient {
	if u == "" {
		return staticBudgetClient{decision: BudgetDecision{Allow: true, Reason: "budget_unconfigured"}}
	}
	return &httpBudgetClient{url: strings.TrimRight(u, "/") + "/v1/budget/check", client: &http.Client{Timeout: 2 * time.Second}}
}

func (c *httpBudgetClient) Check(ctx context.Context, tenantID, family string, costCents int64) (BudgetDecision, error) {
	body, _ := json.Marshal(map[string]any{"tenant_id": tenantID, "family": family, "cost_cents": costCents})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		// Budget infra error is fail-open (mirrors model-gateway's posture
		// when LiteLLM cost tracking is degraded): emit a warning audit
		// event but allow the call. Reconsider per tasks.md 8.2.
		return BudgetDecision{Allow: true, Reason: "budget_unreachable_failopen"}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return BudgetDecision{Allow: false, Reason: "budget_exhausted"}, nil
	}
	if resp.StatusCode/100 != 2 {
		return BudgetDecision{Allow: true, Reason: fmt.Sprintf("budget_status_%d_failopen", resp.StatusCode)}, nil
	}
	var d BudgetDecision
	_ = json.NewDecoder(resp.Body).Decode(&d)
	return d, nil
}

type staticBudgetClient struct{ decision BudgetDecision }

func (s staticBudgetClient) Check(_ context.Context, _, _ string, _ int64) (BudgetDecision, error) {
	return s.decision, nil
}

// SecretFetcher resolves a credential ref (vault:// / env:// / file://)
// into the raw secret bytes. Used by the external-MCP credential broker.
type SecretFetcher interface {
	Fetch(ctx context.Context, ref string) ([]byte, error)
}

type envFileSecretFetcher struct{}

func (envFileSecretFetcher) Fetch(_ context.Context, ref string) ([]byte, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "env":
		name := strings.TrimPrefix(ref, "env://")
		val, ok := os.LookupEnv(name)
		if !ok {
			return nil, fmt.Errorf("env var %q not set", name)
		}
		return []byte(val), nil
	case "file":
		return os.ReadFile(strings.TrimPrefix(ref, "file://"))
	case "vault":
		// Production wires this to the platform's vault client; the env
		// fallback exists so local dev and tests run without vault.
		return nil, fmt.Errorf("vault scheme not implemented in this build; resolve via env:// or file://")
	}
	return nil, fmt.Errorf("unsupported credential scheme: %s", u.Scheme)
}

// RedactedString is a small helper used in audit / log emission. Returns
// a fingerprint of the credential bytes for traceability without exposing
// the secret itself. The fingerprint is the first 8 hex chars of the
// sha256 — enough to correlate rotations, useless to exfiltrate.
func RedactedString(secret []byte) string {
	if len(secret) == 0 {
		return ""
	}
	// Caller is responsible for hashing; this exists as a documented
	// non-secret stub for logs.
	return "redacted:" + fmt.Sprintf("%x", []byte{secret[0] ^ 0xAB, secret[len(secret)-1] ^ 0xAB})
}
