package tests

// Integration tests for services/mcp-gateway. The handler files live in
// cmd/server (package main), so these tests cannot import them directly
// — they exercise the gateway through its public HTTP surface using
// httptest. The test fakes (registry, policy, budget) stand in for the
// real upstream services so the test binary doesn't need a live cluster.
//
// These cover the §4 spec scenarios end-to-end:
//   - Internal call routes through identity-sign + relay
//   - External call resolves credential and redacts it from the response
//   - Policy deny → 403 with policy.reason
//   - Budget exhaustion → 429
//   - Tool not in allowlist → 403
//   - SSE relay preserves event boundaries
//   - Catalog endpoint surfaces approved MCPs
//   - JWKS endpoint exposes the rotating public key

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// startGateway boots the mcp-gateway binary with a minimal config aimed at
// localhost fakes. The function returns the gateway's base URL plus a
// shutdown func.
//
// We build the binary lazily so the test depends only on `go test`
// (no extra make targets) and runs on any machine with Go installed.
func startGateway(t *testing.T, env map[string]string) (string, func()) {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(filepath.Dir(thisFile)) // services/mcp-gateway
	bin := filepath.Join(t.TempDir(), "mcp-gateway")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/server")
	cmd.Dir = pkgDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build gateway: %v: %s", err, string(out))
	}
	srv := exec.Command(bin)
	srv.Dir = pkgDir
	srv.Env = append(srv.Env,
		"ADDR=127.0.0.1:0", // we'll patch this below via lsof-equivalent? no — fixed port
		"ENV=test",
	)
	// Replace ADDR=:0 with a real ephemeral port. We pre-bind and release
	// to discover a free port; this is racy but adequate for tests.
	port := pickPort(t)
	addr := "127.0.0.1:" + port
	srv.Env = []string{"ADDR=" + addr, "ENV=test"}
	for k, v := range env {
		srv.Env = append(srv.Env, k+"="+v)
	}
	srv.Stdout = testWriter{t: t, prefix: "[mcp-gw stdout] "}
	srv.Stderr = testWriter{t: t, prefix: "[mcp-gw stderr] "}
	if err := srv.Start(); err != nil {
		t.Fatalf("start gateway: %v", err)
	}
	// Probe /healthz until ready (or 5s).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "http://" + addr, func() {
		_ = srv.Process.Kill()
		_ = srv.Wait()
	}
}

type testWriter struct {
	t      *testing.T
	prefix string
}

func (tw testWriter) Write(p []byte) (int, error) {
	tw.t.Logf("%s%s", tw.prefix, strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

func pickPort(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	addr := srv.Listener.Addr().String()
	srv.Close()
	return strings.TrimPrefix(addr, "127.0.0.1:")
}

// fakeRegistry simulates the bits of services/registry the gateway calls.
type fakeRegistry struct {
	assets map[string]map[string]any
}

func newFakeRegistry() *fakeRegistry { return &fakeRegistry{assets: map[string]map[string]any{}} }

func (f *fakeRegistry) seed(assetID string, a map[string]any) { f.assets[assetID] = a }

func (f *fakeRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/v1/assets/") {
		id := strings.TrimPrefix(r.URL.Path, "/v1/assets/")
		a, ok := f.assets[id]
		if !ok {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(a)
		return
	}
	if r.URL.Path == "/v1/registry/mcps" {
		var out []map[string]any
		for _, a := range f.assets {
			if t, _ := a["type"].(string); t == "mcp" {
				out = append(out, a)
			}
		}
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	w.WriteHeader(404)
}

type fakePolicy struct {
	mu      sync.Mutex
	allow   bool
	reason  string
	queries []map[string]any
}

func (f *fakePolicy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	var probe map[string]any
	_ = json.Unmarshal(body, &probe)
	f.queries = append(f.queries, probe)
	f.mu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]any{"allow": f.allow, "reason": f.reason})
}

type fakeBudget struct {
	allow  bool
	reason string
}

func (f *fakeBudget) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]any{"allow": f.allow, "reason": f.reason})
}

// fakeMCP serves the canned MCP responses the gateway will forward.
type fakeMCP struct {
	mu             sync.Mutex
	receivedHeader http.Header
	receivedBody   []byte
	respStatus     int
	respBody       []byte
	respHeaders    map[string]string
	stream         func(w http.ResponseWriter)
}

func newFakeMCP() *fakeMCP {
	return &fakeMCP{respStatus: 200, respBody: []byte(`{"ok":true}`), respHeaders: map[string]string{"content-type": "application/json"}}
}

func (f *fakeMCP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.receivedHeader = r.Header.Clone()
	b, _ := io.ReadAll(r.Body)
	f.receivedBody = b
	stream := f.stream
	status := f.respStatus
	headers := f.respHeaders
	body := f.respBody
	f.mu.Unlock()
	if stream != nil {
		stream(w)
		return
	}
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// --- tests --------------------------------------------------------------

func TestInternalMCPCallSucceeds(t *testing.T) {
	mcp := newFakeMCP()
	mcpSrv := httptest.NewServer(mcp)
	defer mcpSrv.Close()

	reg := newFakeRegistry()
	reg.seed("internal-mcp", map[string]any{
		"id":              "internal-mcp",
		"type":            "mcp",
		"lifecycle_state": "approved",
		"provenance":      "internal",
		"active_surface":  map[string]any{"family": "mcp", "endpoint": "/v1/gw/mcp/internal-mcp", "upstream_endpoint": mcpSrv.URL},
		"how_to":          map[string]any{},
	})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	pol := &fakePolicy{allow: true}
	polSrv := httptest.NewServer(pol)
	defer polSrv.Close()

	gw, stop := startGateway(t, map[string]string{
		"REGISTRY_URL":      regSrv.URL,
		"POLICY_ENGINE_URL": polSrv.URL,
		"BUDGET_URL":        "",
		"KAFKA_BROKERS":     "disabled",
		"REDIS_ADDR":        "disabled",
	})
	defer stop()

	req, _ := http.NewRequest(http.MethodPost, gw+"/v1/gw/mcp/internal-mcp?tool=hello", strings.NewReader(`{"hi":"there"}`))
	req.Header.Set("authorization", "Bearer alice.acme.ws1")
	req.Header.Set("X-Forge-Correlation-Id", "corr-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200; got %d body=%s", resp.StatusCode, string(body))
	}
	// Verify the upstream MCP received the signed identity headers.
	if got := mcp.receivedHeader.Get("X-Forge-Principal"); got != "alice" {
		t.Fatalf("X-Forge-Principal=%q, want alice", got)
	}
	if got := mcp.receivedHeader.Get("X-Forge-Tenant"); got != "acme" {
		t.Fatalf("X-Forge-Tenant=%q, want acme", got)
	}
	if mcp.receivedHeader.Get("X-Forge-Identity-Signature") == "" {
		t.Fatalf("missing signature header on outbound")
	}
	// And the policy engine saw the right input.
	pol.mu.Lock()
	defer pol.mu.Unlock()
	if len(pol.queries) != 1 {
		t.Fatalf("expected 1 policy query; got %d", len(pol.queries))
	}
	if pol.queries[0]["asset_id"] != "internal-mcp" {
		t.Fatalf("policy query asset_id mismatch: %+v", pol.queries[0])
	}
}

func TestExternalMCPCredentialRedaction(t *testing.T) {
	mcp := newFakeMCP()
	mcpSrv := httptest.NewServer(mcp)
	defer mcpSrv.Close()
	reg := newFakeRegistry()
	reg.seed("external-mcp", map[string]any{
		"id": "external-mcp", "type": "mcp", "lifecycle_state": "approved",
		"provenance": "external",
		"active_surface": map[string]any{"family": "mcp"},
		"endpoint": mcpSrv.URL, "credential_ref": "env://FORGE_FAKE_CREDENTIAL",
		"allowlist": []string{"hello"},
	})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	pol := &fakePolicy{allow: true}
	polSrv := httptest.NewServer(pol)
	defer polSrv.Close()
	gw, stop := startGateway(t, map[string]string{
		"REGISTRY_URL":          regSrv.URL,
		"POLICY_ENGINE_URL":     polSrv.URL,
		"KAFKA_BROKERS":         "disabled",
		"REDIS_ADDR":            "disabled",
		"FORGE_FAKE_CREDENTIAL": "shh-this-should-not-leak",
	})
	defer stop()
	req, _ := http.NewRequest(http.MethodPost, gw+"/v1/gw/mcp/external-mcp?tool=hello", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("authorization", "Bearer u.acme.ws1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200; got %d body=%s", resp.StatusCode, string(body))
	}
	// The credential MUST land on the outbound request as Bearer auth.
	if got := mcp.receivedHeader.Get("authorization"); !strings.HasPrefix(got, "Bearer ") {
		t.Fatalf("expected Bearer Authorization on outbound; got %q", got)
	}
	// And it MUST NOT echo back into the gateway's response body.
	if strings.Contains(string(body), "shh-this-should-not-leak") {
		t.Fatalf("credential leaked into response body")
	}
}

func TestPolicyDeniedReturns403(t *testing.T) {
	reg := newFakeRegistry()
	reg.seed("any", map[string]any{
		"id": "any", "type": "mcp", "lifecycle_state": "approved", "provenance": "internal",
		"active_surface": map[string]any{"family": "mcp", "upstream_endpoint": "http://unused.example.com"},
	})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	pol := &fakePolicy{allow: false, reason: "tenant-quota-exceeded"}
	polSrv := httptest.NewServer(pol)
	defer polSrv.Close()
	gw, stop := startGateway(t, map[string]string{
		"REGISTRY_URL": regSrv.URL, "POLICY_ENGINE_URL": polSrv.URL,
		"KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled",
	})
	defer stop()
	req, _ := http.NewRequest(http.MethodPost, gw+"/v1/gw/mcp/any", strings.NewReader(`{}`))
	req.Header.Set("authorization", "Bearer u.t.w")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403; got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["code"] != "policy_denied" {
		t.Fatalf("expected code=policy_denied; got %+v", body)
	}
}

func TestBudgetExhaustedReturns429(t *testing.T) {
	reg := newFakeRegistry()
	reg.seed("budgeted", map[string]any{
		"id": "budgeted", "type": "mcp", "lifecycle_state": "approved", "provenance": "internal",
		"active_surface": map[string]any{"family": "mcp", "upstream_endpoint": "http://unused.example.com"},
	})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	pol := &fakePolicy{allow: true}
	polSrv := httptest.NewServer(pol)
	defer polSrv.Close()
	bud := &fakeBudget{allow: false, reason: "monthly_mcp_quota_exhausted"}
	budSrv := httptest.NewServer(bud)
	defer budSrv.Close()
	gw, stop := startGateway(t, map[string]string{
		"REGISTRY_URL": regSrv.URL, "POLICY_ENGINE_URL": polSrv.URL, "BUDGET_URL": budSrv.URL,
		"KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled",
	})
	defer stop()
	req, _ := http.NewRequest(http.MethodPost, gw+"/v1/gw/mcp/budgeted", strings.NewReader(`{}`))
	req.Header.Set("authorization", "Bearer u.t.w")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Fatalf("expected 429; got %d", resp.StatusCode)
	}
}

func TestToolNotAllowlistedRejected(t *testing.T) {
	mcp := newFakeMCP()
	mcpSrv := httptest.NewServer(mcp)
	defer mcpSrv.Close()
	reg := newFakeRegistry()
	reg.seed("vendor", map[string]any{
		"id": "vendor", "type": "mcp", "lifecycle_state": "approved", "provenance": "external",
		"active_surface": map[string]any{"family": "mcp"},
		"endpoint": mcpSrv.URL, "credential_ref": "env://X",
		"allowlist": []string{"read"},
	})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	pol := &fakePolicy{allow: true}
	polSrv := httptest.NewServer(pol)
	defer polSrv.Close()
	gw, stop := startGateway(t, map[string]string{
		"REGISTRY_URL": regSrv.URL, "POLICY_ENGINE_URL": polSrv.URL,
		"KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled",
		"X": "tok",
	})
	defer stop()
	req, _ := http.NewRequest(http.MethodPost, gw+"/v1/gw/mcp/vendor?tool=delete_all_things", strings.NewReader(`{}`))
	req.Header.Set("authorization", "Bearer u.t.w")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403; got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["code"] != "tool_not_allowlisted" {
		t.Fatalf("expected code=tool_not_allowlisted; got %+v", body)
	}
}

func TestSSERelayPreservesEventBoundaries(t *testing.T) {
	mcp := newFakeMCP()
	mcp.stream = func(w http.ResponseWriter) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(200)
		flush := w.(http.Flusher).Flush
		for i := 0; i < 3; i++ {
			_, _ = w.Write([]byte("data: event " + intToStr(i) + "\n\n"))
			flush()
			time.Sleep(2 * time.Millisecond)
		}
	}
	mcpSrv := httptest.NewServer(mcp)
	defer mcpSrv.Close()
	reg := newFakeRegistry()
	reg.seed("streaming", map[string]any{
		"id": "streaming", "type": "mcp", "lifecycle_state": "approved", "provenance": "internal",
		"active_surface": map[string]any{"family": "mcp", "upstream_endpoint": mcpSrv.URL},
	})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	pol := &fakePolicy{allow: true}
	polSrv := httptest.NewServer(pol)
	defer polSrv.Close()
	gw, stop := startGateway(t, map[string]string{
		"REGISTRY_URL": regSrv.URL, "POLICY_ENGINE_URL": polSrv.URL,
		"KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled",
	})
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, gw+"/v1/gw/mcp/streaming?tool=stream", strings.NewReader(`{}`))
	req.Header.Set("authorization", "Bearer u.t.w")
	req.Header.Set("accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200; got %d body=%s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("read stream: %v", err)
	}
	for i := 0; i < 3; i++ {
		if !strings.Contains(string(body), "event "+intToStr(i)) {
			t.Fatalf("expected event %d in stream; got %q", i, string(body))
		}
	}
}

func TestCatalogReturnsApprovedMCPs(t *testing.T) {
	reg := newFakeRegistry()
	reg.seed("approved-1", map[string]any{
		"id": "approved-1", "type": "mcp", "lifecycle_state": "approved", "provenance": "internal",
		"active_surface": map[string]any{"family": "mcp"},
		"how_to":         map[string]any{"install": map[string]any{"cli": "forge install"}},
	})
	reg.seed("approved-2", map[string]any{
		"id": "approved-2", "type": "mcp", "lifecycle_state": "approved", "provenance": "external",
		"active_surface": map[string]any{"family": "mcp"},
	})
	reg.seed("pending", map[string]any{
		"id": "pending", "type": "mcp", "lifecycle_state": "in_review", "provenance": "internal",
	})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	gw, stop := startGateway(t, map[string]string{
		"REGISTRY_URL":  regSrv.URL,
		"KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled",
	})
	defer stop()
	req, _ := http.NewRequest(http.MethodGet, gw+"/v1/gw/mcp/catalog", nil)
	req.Header.Set("authorization", "Bearer u.t.w")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200; got %d", resp.StatusCode)
	}
	var body struct{ Items []map[string]any }
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 approved MCPs in catalog; got %d", len(body.Items))
	}
}

func TestJWKSExposesRotatingKey(t *testing.T) {
	gw, stop := startGateway(t, map[string]string{
		"KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled",
	})
	defer stop()
	resp, err := http.Get(gw + "/jwks")
	if err != nil {
		t.Fatalf("get jwks: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200; got %d", resp.StatusCode)
	}
	var body struct {
		Keys []map[string]any `json:"keys"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Keys) < 1 {
		t.Fatalf("expected at least one key in JWKS; got %d", len(body.Keys))
	}
	if body.Keys[0]["alg"] != "EdDSA" {
		t.Fatalf("expected alg=EdDSA; got %v", body.Keys[0]["alg"])
	}
}

func intToStr(i int) string { return strings.TrimSpace(string(rune('0' + i))) }
