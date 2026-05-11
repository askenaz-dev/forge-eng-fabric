package onboarding

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tplBaseDir(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "forge-templates", "templates")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("forge-templates not found")
	return ""
}

func newTestService(t *testing.T) (*Service, *MemorySink, *StubGitHubMCP) {
	t.Helper()
	store := NewStore()
	sink := &MemorySink{}
	stub := &StubGitHubMCP{}
	svc := NewService(store, sink)
	svc.Catalog = FilesystemCatalog{BaseDir: tplBaseDir(t)}
	svc.GitHub = stub
	svc.WorkOutDir = t.TempDir()
	return svc, sink, stub
}

func TestDefaultServiceCatalogFindsRepoTemplatesFromServiceDir(t *testing.T) {
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalCWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	if err := os.Chdir(filepath.Join(tplBaseDir(t), "..", "..", "services", "app-onboarding")); err != nil {
		t.Fatalf("chdir service dir: %v", err)
	}

	svc := NewService(NewStore(), &MemorySink{})
	templates, err := svc.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("expected templates")
	}
}

func waitFor(t *testing.T, store *Store, id string, want Status, timeout time.Duration) *Request {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r, ok := store.Get(id); ok && r.Status == want {
			return r
		}
		time.Sleep(20 * time.Millisecond)
	}
	if r, ok := store.Get(id); ok {
		t.Fatalf("timeout waiting for %s, got status=%s reason=%s", want, r.Status, r.StatusReason)
	}
	t.Fatalf("request not found")
	return nil
}

func TestSubmitFullFlow(t *testing.T) {
	svc, sink, stub := newTestService(t)
	req := &Request{
		WorkspaceID:        "ws-1",
		TenantID:           "tn-1",
		RepoOrg:            "org-a",
		RepoName:           "svc-foo",
		TemplateID:         "go-microservice",
		TemplateVersion:    "1.0.0",
		Owners:             []string{"@team-a"},
		Criticality:        "high",
		DataClassification: "internal",
		RequestedBy:        "alice",
	}
	out, err := svc.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	got := waitFor(t, svc.Store, out.ID, StatusCompleted, 5*time.Second)
	if got.AssetID == "" {
		t.Fatalf("expected asset id set")
	}
	// Stages must include all 9 lifecycle entries
	events := svc.Store.Events(out.ID)
	stages := map[string]int{}
	for _, e := range events {
		stages[e.Stage]++
	}
	for _, s := range []string{"policy.evaluate", "template.resolve", "scaffold.render",
		"github.create_repo", "github.codeowners", "github.pr_template",
		"github.branch_protection", "github.required_checks", "asset.register"} {
		if stages[s] < 2 {
			t.Fatalf("stage %s missing started/completed events (got %d)", s, stages[s])
		}
	}
	// CloudEvents emitted
	if len(sink.ByType("com.forge.app.onboarding_requested.v1")) != 1 {
		t.Fatal("missing requested event")
	}
	if len(sink.ByType("com.forge.app.onboarding_completed.v1")) != 1 {
		t.Fatal("missing completed event")
	}
	if len(sink.ByType("com.forge.repo.created.v1")) != 1 {
		t.Fatal("missing repo.created event")
	}
	if len(sink.ByType("com.forge.branch_protection_applied.v1")) != 1 {
		t.Fatal("missing branch_protection event")
	}
	// Critical-tier branch protection: 2 reviewers, signed commits
	var bpCall *StubCall
	for i := range stub.Calls {
		if stub.Calls[i].Method == "SetBranchProtection" {
			c := stub.Calls[i]
			bpCall = &c
		}
	}
	if bpCall == nil {
		t.Fatal("SetBranchProtection not called")
	}
	rules, _ := bpCall.Params["rules"].(map[string]any)
	if rules["min_reviewers"].(int) != 2 {
		t.Fatalf("expected 2 reviewers for high criticality, got %v", rules["min_reviewers"])
	}
	if rules["signed_commits"].(bool) != true {
		t.Fatalf("expected signed_commits true for high criticality")
	}
}

func TestSubmitIdempotency(t *testing.T) {
	svc, _, _ := newTestService(t)
	build := func() *Request {
		return &Request{
			WorkspaceID:     "ws-1",
			TenantID:        "tn-1",
			RepoOrg:         "org-a",
			RepoName:        "dup-svc",
			TemplateID:      "go-microservice",
			TemplateVersion: "1.0.0",
			Owners:          []string{"@team-a"},
			Criticality:     "low",
		}
	}
	first, err := svc.Submit(context.Background(), build())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	second, err := svc.Submit(context.Background(), build())
	if err != nil {
		t.Fatalf("submit2: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected idempotent ID match, got %s vs %s", first.ID, second.ID)
	}
}

func TestPolicyDeniedFails(t *testing.T) {
	svc, _, _ := newTestService(t)
	svc.Policy = denyPolicy{}
	req := &Request{
		WorkspaceID:     "ws-1",
		TenantID:        "tn-1",
		RepoOrg:         "org-a",
		RepoName:        "denied-svc",
		TemplateID:      "go-microservice",
		TemplateVersion: "1.0.0",
		Owners:          []string{"@team-a"},
	}
	out, err := svc.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	got := waitFor(t, svc.Store, out.ID, StatusFailed, 3*time.Second)
	if !strings.Contains(got.StatusReason, "policy denied") {
		t.Fatalf("expected policy denied reason, got %q", got.StatusReason)
	}
}

type denyPolicy struct{}

func (denyPolicy) CheckOnboarding(_ context.Context, _ *Request) (PolicyDecision, error) {
	return PolicyDecision{Decision: "deny", Rationale: "test denied"}, nil
}

func TestHandlerPostAndGet(t *testing.T) {
	svc, _, _ := newTestService(t)
	mux := http.NewServeMux()
	NewHandler(svc).Mount(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := strings.NewReader(`{
		"workspace_id":"ws-1","tenant_id":"tn-1",
		"repo_org":"org-a","repo_name":"http-svc",
		"template_id":"go-microservice","template_version":"1.0.0",
		"owners":["@team-a"],"criticality":"low"
	}`)
	resp, err := http.Post(srv.URL+"/v1/onboarding", "application/json", body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	var created Request
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == "" {
		t.Fatal("missing id in response")
	}

	waitFor(t, svc.Store, created.ID, StatusCompleted, 3*time.Second)

	getResp, err := http.Get(srv.URL + "/v1/onboarding/" + created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if getResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", getResp.StatusCode)
	}
	var fetched Request
	_ = json.NewDecoder(getResp.Body).Decode(&fetched)
	getResp.Body.Close()
	if fetched.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", fetched.Status)
	}
}

func TestHandlerListsTemplatesRequestsGatesAndMetrics(t *testing.T) {
	svc, _, _ := newTestService(t)
	mux := http.NewServeMux()
	NewHandler(svc).Mount(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	templatesResp, err := http.Get(srv.URL + "/v1/templates")
	if err != nil {
		t.Fatalf("templates: %v", err)
	}
	if templatesResp.StatusCode != http.StatusOK {
		t.Fatalf("expected templates 200, got %d", templatesResp.StatusCode)
	}
	var templatesOut struct {
		Templates []TemplateSummary `json:"templates"`
	}
	_ = json.NewDecoder(templatesResp.Body).Decode(&templatesOut)
	templatesResp.Body.Close()
	if len(templatesOut.Templates) == 0 {
		t.Fatal("expected filesystem templates")
	}

	body := strings.NewReader(`{
		"workspace_id":"ws-1","tenant_id":"tn-1",
		"repo_org":"org-a","repo_name":"history-svc",
		"template_id":"go-microservice","template_version":"1.0.0",
		"owners":["@team-a"],"criticality":"low"
	}`)
	resp, err := http.Post(srv.URL+"/v1/onboarding", "application/json", body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	var created Request
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	waitFor(t, svc.Store, created.ID, StatusCompleted, 3*time.Second)

	listResp, err := http.Get(srv.URL + "/v1/onboarding?workspace_id=ws-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var listOut struct {
		Requests []Request `json:"requests"`
	}
	_ = json.NewDecoder(listResp.Body).Decode(&listOut)
	listResp.Body.Close()
	if len(listOut.Requests) != 1 || listOut.Requests[0].ID != created.ID {
		t.Fatalf("unexpected request list: %#v", listOut.Requests)
	}

	svc.Store.RecordGateResult(PipelineGateResult{WorkspaceID: "ws-1", RepoFullName: "org-a/history-svc", PRNumber: 7, CommitSHA: "abc123", Stage: "sast", Tool: "semgrep", Outcome: "pass", ReportURL: "https://logs.example/sast"})
	gatesResp, err := http.Get(srv.URL + "/v1/pipeline-gates?workspace_id=ws-1&repo=org-a/history-svc&pr=7")
	if err != nil {
		t.Fatalf("gates: %v", err)
	}
	var gatesOut struct {
		Results []PipelineGateResult `json:"results"`
	}
	_ = json.NewDecoder(gatesResp.Body).Decode(&gatesOut)
	gatesResp.Body.Close()
	if len(gatesOut.Results) != 1 || gatesOut.Results[0].Stage != "sast" {
		t.Fatalf("unexpected gate results: %#v", gatesOut.Results)
	}

	metricsResp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	metricsBody, _ := io.ReadAll(metricsResp.Body)
	metricsResp.Body.Close()
	metrics := string(metricsBody)
	for _, name := range []string{"onboarding_duration_seconds", "onboarding_success_rate", "pipeline_gate_failure_rate", "pr_openspec_link_coverage", "image_signing_rate", "override_count"} {
		if !strings.Contains(metrics, name) {
			t.Fatalf("metrics missing %s: %s", name, metrics)
		}
	}
}

func TestHTTPRegistryRegistrarCreatesApplicationAsset(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/v1/workspaces/ws-1/assets" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("authorization") != "Bearer test-token" {
			t.Fatalf("missing bearer token")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"application:ws-1:svc-foo"}`))
	}))
	defer srv.Close()

	registrar := NewHTTPRegistryRegistrar(srv.URL, "test-token")
	req := &Request{WorkspaceID: "ws-1", RepoName: "svc-foo", TemplateID: "go-microservice", TemplateVersion: "1.0.0", Owners: []string{"@team-a"}, Parameters: map[string]any{"app_version": "1.2.3"}}
	assetID, err := registrar.RegisterApplication(context.Background(), req, "https://github.com/org/svc-foo", map[string]any{"image_repository": "registry/ws-1/svc-foo"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if assetID != "application:ws-1:svc-foo" {
		t.Fatalf("unexpected asset id %s", assetID)
	}
	if got["type"] != "application" || got["lifecycle_state"] != "proposed" || got["version"] != "1.2.3" {
		t.Fatalf("unexpected registry payload: %#v", got)
	}
	metadata := got["metadata"].(map[string]any)
	if metadata["image_repository"] != "registry/ws-1/svc-foo" {
		t.Fatalf("metadata missing image repository: %#v", metadata)
	}
}

func TestSSEStreamsTerminalState(t *testing.T) {
	svc, _, _ := newTestService(t)
	mux := http.NewServeMux()
	NewHandler(svc).Mount(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := strings.NewReader(`{
		"workspace_id":"ws-1","tenant_id":"tn-1",
		"repo_org":"org-a","repo_name":"sse-svc",
		"template_id":"go-microservice","template_version":"1.0.0",
		"owners":["@team-a"],"criticality":"medium"
	}`)
	resp, _ := http.Post(srv.URL+"/v1/onboarding", "application/json", body)
	var created Request
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// Wait for the workflow to complete — terminal frame replays from buffer.
	waitFor(t, svc.Store, created.ID, StatusCompleted, 3*time.Second)

	stream, err := http.Get(srv.URL + "/v1/onboarding/" + created.ID + "/events")
	if err != nil {
		t.Fatalf("sse: %v", err)
	}
	defer stream.Body.Close()
	all, err := io.ReadAll(stream.Body)
	if err != nil {
		t.Fatalf("read sse: %v", err)
	}
	got := string(all)
	if !strings.Contains(got, "event: terminal") {
		t.Fatalf("expected terminal event, got: %q", got)
	}
	if !strings.Contains(got, "asset.register") {
		t.Fatalf("expected stage events in stream, got: %q", got)
	}
}
