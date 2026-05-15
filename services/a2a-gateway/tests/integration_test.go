package tests

// Integration tests for services/a2a-gateway. The test boots the real
// binary and drives it via HTTP, with httptest fakes standing in for
// services/registry, services/policy-engine and downstream A2A agents.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

func startGateway(t *testing.T, env map[string]string) (string, func()) {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(filepath.Dir(thisFile))
	bin := filepath.Join(t.TempDir(), "a2a-gateway")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/server")
	cmd.Dir = pkgDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build gateway: %v: %s", err, string(out))
	}
	port := pickPort(t)
	addr := "127.0.0.1:" + port
	srv := exec.Command(bin)
	srv.Dir = pkgDir
	srv.Env = []string{"ADDR=" + addr, "ENV=test"}
	for k, v := range env {
		srv.Env = append(srv.Env, k+"="+v)
	}
	srv.Stdout = testWriter{t: t, prefix: "[a2a-gw stdout] "}
	srv.Stderr = testWriter{t: t, prefix: "[a2a-gw stderr] "}
	if err := srv.Start(); err != nil {
		t.Fatalf("start gateway: %v", err)
	}
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

type fakeRegistry struct {
	assets map[string]map[string]any
}

func newFakeRegistry() *fakeRegistry { return &fakeRegistry{assets: map[string]map[string]any{}} }
func (f *fakeRegistry) seed(id string, a map[string]any) { f.assets[id] = a }

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
	if r.URL.Path == "/v1/registry/agents" {
		var out []map[string]any
		for _, a := range f.assets {
			if t, _ := a["type"].(string); t == "agent" {
				out = append(out, a)
			}
		}
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	w.WriteHeader(404)
}

type fakePolicy struct {
	mu     sync.Mutex
	allow  bool
	reason string
}

func (f *fakePolicy) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]any{"allow": f.allow, "reason": f.reason})
}

type fakeAgent struct {
	mu       sync.Mutex
	headers  http.Header
	body     []byte
	respBody []byte
	stream   func(w http.ResponseWriter)
}

func newFakeAgent() *fakeAgent {
	return &fakeAgent{respBody: []byte(`{"jsonrpc":"2.0","result":{"task_id":"t-1","status":"completed"}}`)}
}

func (f *fakeAgent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.headers = r.Header.Clone()
	f.body, _ = io.ReadAll(r.Body)
	stream := f.stream
	body := f.respBody
	f.mu.Unlock()
	if stream != nil {
		stream(w)
		return
	}
	w.Header().Set("content-type", "application/json")
	_, _ = w.Write(body)
}

// helper: post JSON-RPC body with auth token
func postRPC(t *testing.T, base, path, token string, env map[string]any) *http.Response {
	t.Helper()
	body, _ := json.Marshal(env)
	req, _ := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	if token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return resp
}

// --- tests --------------------------------------------------------------

func TestOutboundExternalA2A_CredentialRedaction(t *testing.T) {
	agent := newFakeAgent()
	agentSrv := httptest.NewServer(agent)
	defer agentSrv.Close()
	reg := newFakeRegistry()
	reg.seed("partner-a", map[string]any{
		"id": "partner-a", "type": "agent", "lifecycle_state": "approved", "provenance": "external",
		"endpoint": agentSrv.URL, "credential_ref": "env://FORGE_FAKE_CREDENTIAL",
		"task_allowlist": []string{"send"},
	})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	pol := &fakePolicy{allow: true}
	polSrv := httptest.NewServer(pol)
	defer polSrv.Close()
	gw, stop := startGateway(t, map[string]string{
		"REGISTRY_URL": regSrv.URL, "POLICY_ENGINE_URL": polSrv.URL,
		"KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled",
		"FORGE_FAKE_CREDENTIAL": "shh-this-must-not-leak",
	})
	defer stop()
	resp := postRPC(t, gw, "/v1/gw/a2a/partner-a", "alice.acme.ws1", map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tasks/send",
		"params": map[string]any{"task": map[string]any{"text": "hi"}},
	})
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200; got %d body=%s", resp.StatusCode, string(body))
	}
	if auth := agent.headers.Get("authorization"); !strings.HasPrefix(auth, "Bearer ") {
		t.Fatalf("upstream Authorization header missing; got %q", auth)
	}
	if agent.headers.Get("X-Forge-Identity-Signature") == "" {
		t.Fatalf("missing identity signature on upstream")
	}
	if agent.headers.Get("X-Forge-Principal-Kind") != "service" {
		t.Fatalf("principal_kind=%q", agent.headers.Get("X-Forge-Principal-Kind"))
	}
	if strings.Contains(string(body), "shh-this-must-not-leak") {
		t.Fatalf("credential leaked in response body")
	}
}

func TestInboundEnrolledPartnerSucceeds(t *testing.T) {
	agent := newFakeAgent()
	agentSrv := httptest.NewServer(agent)
	defer agentSrv.Close()
	reg := newFakeRegistry()
	reg.seed("forge-architect", map[string]any{
		"id": "forge-architect", "type": "agent", "lifecycle_state": "approved", "provenance": "internal",
		"active_surface": map[string]any{"family": "a2a", "upstream_endpoint": agentSrv.URL},
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

	// Step 1: enroll partner-b.
	credRaw := []byte("partner-b-static-credential-for-testing")
	credB64 := base64.StdEncoding.EncodeToString(credRaw)
	enrollBody, _ := json.Marshal(map[string]any{
		"name": "partner-b", "workspace_id": "ws-1",
		"allowed_assets":   []string{"forge-architect"},
		"credential_b64":   credB64,
	})
	req, _ := http.NewRequest(http.MethodPost, gw+"/v1/gw/a2a/partners", bytes.NewReader(enrollBody))
	req.Header.Set("authorization", "Bearer admin.acme.ws-1")
	req.Header.Set("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("enroll: status=%d", resp.StatusCode)
	}

	// Step 2: partner-b makes an inbound call.
	rpc := map[string]any{
		"jsonrpc": "2.0", "id": "7", "method": "tasks/send",
		"params": map[string]any{"task": map[string]any{"text": "design us a workflow"}},
	}
	rpcBody, _ := json.Marshal(rpc)
	mac := hmac.New(sha256.New, credRaw)
	_, _ = mac.Write(rpcBody)
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req2, _ := http.NewRequest(http.MethodPost, gw+"/v1/gw/a2a/forge-architect", bytes.NewReader(rpcBody))
	req2.Header.Set("content-type", "application/json")
	req2.Header.Set("X-Forge-Partner-Auth", "partner-b;"+sig)
	req2.Header.Set("X-Forge-Inbound-Tenant", "acme")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("inbound: %v", err)
	}
	body, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("inbound: status=%d body=%s", resp2.StatusCode, string(body))
	}
	if got := agent.headers.Get("X-Forge-Principal-Kind"); got != "external_agent" {
		t.Fatalf("principal_kind on internal agent should be external_agent; got %q", got)
	}
	if got := agent.headers.Get("X-Forge-Principal"); got != "partner-b" {
		t.Fatalf("X-Forge-Principal on internal agent should be partner-b; got %q", got)
	}
}

func TestInboundUnenrolledPartnerRejected(t *testing.T) {
	reg := newFakeRegistry()
	reg.seed("forge-architect", map[string]any{
		"id": "forge-architect", "type": "agent", "lifecycle_state": "approved", "provenance": "internal",
		"active_surface": map[string]any{"family": "a2a", "upstream_endpoint": "http://unused"},
	})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	gw, stop := startGateway(t, map[string]string{
		"REGISTRY_URL":  regSrv.URL,
		"KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled",
	})
	defer stop()
	rpc := map[string]any{"jsonrpc": "2.0", "id": "1", "method": "tasks/send"}
	body, _ := json.Marshal(rpc)
	req, _ := http.NewRequest(http.MethodPost, gw+"/v1/gw/a2a/forge-architect", bytes.NewReader(body))
	req.Header.Set("X-Forge-Partner-Auth", "ghost-partner;"+base64.StdEncoding.EncodeToString([]byte("anything")))
	req.Header.Set("X-Forge-Inbound-Tenant", "acme")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("inbound: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401; got %d", resp.StatusCode)
	}
	var parsed map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	if parsed["code"] != "unknown_partner" {
		t.Fatalf("expected code=unknown_partner; got %v", parsed)
	}
}

func TestPolicyDeniedOutbound(t *testing.T) {
	reg := newFakeRegistry()
	reg.seed("partner-a", map[string]any{
		"id": "partner-a", "type": "agent", "lifecycle_state": "approved", "provenance": "external",
		"endpoint": "http://unused", "credential_ref": "env://X",
	})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	pol := &fakePolicy{allow: false, reason: "not-allowed-this-workspace"}
	polSrv := httptest.NewServer(pol)
	defer polSrv.Close()
	gw, stop := startGateway(t, map[string]string{
		"REGISTRY_URL": regSrv.URL, "POLICY_ENGINE_URL": polSrv.URL,
		"KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled", "X": "tok",
	})
	defer stop()
	resp := postRPC(t, gw, "/v1/gw/a2a/partner-a", "u.t.w", map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tasks/send",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403; got %d", resp.StatusCode)
	}
}

func TestSendSubscribeRelaysStream(t *testing.T) {
	agent := newFakeAgent()
	agent.stream = func(w http.ResponseWriter) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(200)
		flush := w.(http.Flusher).Flush
		for i := 0; i < 3; i++ {
			fmt.Fprintf(w, "data: event %d\n\n", i)
			flush()
			time.Sleep(2 * time.Millisecond)
		}
	}
	agentSrv := httptest.NewServer(agent)
	defer agentSrv.Close()
	reg := newFakeRegistry()
	reg.seed("streamer", map[string]any{
		"id": "streamer", "type": "agent", "lifecycle_state": "approved", "provenance": "internal",
		"active_surface": map[string]any{"family": "a2a", "upstream_endpoint": agentSrv.URL},
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
	rpc, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tasks/sendSubscribe",
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, gw+"/v1/gw/a2a/streamer", bytes.NewReader(rpc))
	req.Header.Set("authorization", "Bearer u.t.w")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("subscribe: status=%d body=%s", resp.StatusCode, string(body))
	}
	bb, _ := io.ReadAll(resp.Body)
	for i := 0; i < 3; i++ {
		if !strings.Contains(string(bb), fmt.Sprintf("event %d", i)) {
			t.Fatalf("expected event %d in stream; got %q", i, string(bb))
		}
	}
}

func TestCatalogListsApprovedAgents(t *testing.T) {
	reg := newFakeRegistry()
	reg.seed("a1", map[string]any{"id": "a1", "type": "agent", "lifecycle_state": "approved", "provenance": "internal"})
	reg.seed("a2", map[string]any{"id": "a2", "type": "agent", "lifecycle_state": "approved", "provenance": "external"})
	reg.seed("draft", map[string]any{"id": "draft", "type": "agent", "lifecycle_state": "proposed", "provenance": "internal"})
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	gw, stop := startGateway(t, map[string]string{"REGISTRY_URL": regSrv.URL, "KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled"})
	defer stop()
	req, _ := http.NewRequest(http.MethodGet, gw+"/v1/gw/a2a/catalog", nil)
	req.Header.Set("authorization", "Bearer u.t.w")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200; got %d", resp.StatusCode)
	}
	var body struct{ Items []map[string]any }
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 approved agents; got %d", len(body.Items))
	}
}

func TestUnknownMethodRejected(t *testing.T) {
	reg := newFakeRegistry()
	regSrv := httptest.NewServer(reg)
	defer regSrv.Close()
	gw, stop := startGateway(t, map[string]string{"REGISTRY_URL": regSrv.URL, "KAFKA_BROKERS": "disabled", "REDIS_ADDR": "disabled"})
	defer stop()
	resp := postRPC(t, gw, "/v1/gw/a2a/anything", "u.t.w",
		map[string]any{"jsonrpc": "2.0", "id": "1", "method": "tasks/nuke"})
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400; got %d", resp.StatusCode)
	}
}
