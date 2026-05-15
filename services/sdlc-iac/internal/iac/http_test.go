package iac

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestHandler() (*Handler, *MemorySink) {
	sink := &MemorySink{}
	skills := NewIaCSkills(sink)
	return NewHandler(skills), sink
}

func doPost(t *testing.T, mux *http.ServeMux, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("content-type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestHealthz(t *testing.T) {
	h, _ := newTestHandler()
	mux := http.NewServeMux()
	h.Mount(mux)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGenerateTerraform_AWS(t *testing.T) {
	h, sink := newTestHandler()
	mux := http.NewServeMux()
	h.Mount(mux)

	w := doPost(t, mux, "/v1/skills/generate-terraform", GenerateTerraformInput{
		AppID:    "app-1",
		Slug:     "my-svc",
		Provider: ProviderAWS,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var out GenerateTerraformOutput
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ModulePath != "infra/my-svc/terraform" {
		t.Fatalf("unexpected module_path: %s", out.ModulePath)
	}
	if out.EmittedEvent != "sdlc.iac.generated.v1" {
		t.Fatalf("unexpected event type: %s", out.EmittedEvent)
	}
	if len(sink.Events) == 0 {
		t.Fatal("expected event to be emitted")
	}
	if sink.Events[0].Type != "sdlc.iac.generated.v1" {
		t.Fatalf("unexpected event type: %s", sink.Events[0].Type)
	}
}

func TestGenerateTerraform_UnsupportedProvider(t *testing.T) {
	h, _ := newTestHandler()
	mux := http.NewServeMux()
	h.Mount(mux)

	w := doPost(t, mux, "/v1/skills/generate-terraform", GenerateTerraformInput{
		AppID:    "app-2",
		Slug:     "bad-svc",
		Provider: "digitalocean",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported provider, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "unsupported provider") {
		t.Fatalf("expected unsupported provider error, got: %s", w.Body.String())
	}
}

func TestGenerateHelmValues_Medium(t *testing.T) {
	h, sink := newTestHandler()
	mux := http.NewServeMux()
	h.Mount(mux)

	w := doPost(t, mux, "/v1/skills/generate-helm-values", GenerateHelmInput{
		AppID:       "app-3",
		Slug:        "api-svc",
		Criticality: CriticalityMedium,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var out GenerateHelmOutput
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.ValuesFiles) != 3 {
		t.Fatalf("expected 3 values files (local/staging/prod), got %d", len(out.ValuesFiles))
	}
	for _, f := range out.ValuesFiles {
		if !strings.HasPrefix(f, "infra/api-svc/helm/values-") {
			t.Fatalf("unexpected values file path: %s", f)
		}
	}
	if len(sink.Events) == 0 || sink.Events[0].Type != "sdlc.iac.helm_values.generated.v1" {
		t.Fatal("expected helm_values.generated event")
	}
}

func TestValidateIaC_AllPass(t *testing.T) {
	h, sink := newTestHandler()
	mux := http.NewServeMux()
	h.Mount(mux)

	w := doPost(t, mux, "/v1/skills/validate-iac", ValidateIaCInput{
		AppID:               "app-4",
		TerraformModulePath: "infra/svc/terraform",
		HelmValuesPath:      "infra/svc/helm",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var out ValidateIaCOutput
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Status != "passed" {
		t.Fatalf("expected passed, got %s", out.Status)
	}
	if !out.Report.TerraformFmtPassed || !out.Report.ConftestPassed {
		t.Fatalf("expected all checks to pass: %+v", out.Report)
	}
	if len(sink.Events) == 0 || sink.Events[0].Type != "sdlc.iac.validated.v1" {
		t.Fatal("expected validated event")
	}
}

func TestValidateIaC_ConftestFail(t *testing.T) {
	h, _ := newTestHandler()
	mux := http.NewServeMux()
	h.Mount(mux)

	w := doPost(t, mux, "/v1/skills/validate-iac", ValidateIaCInput{
		AppID:               "app-5",
		TerraformModulePath: "infra/svc/terraform",
		HelmValuesPath:      "FAIL_CONFTEST",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var out ValidateIaCOutput
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Status != "failed" {
		t.Fatalf("expected failed, got %s", out.Status)
	}
	if out.Report.ConftestPassed {
		t.Fatal("expected conftest to fail")
	}
	if len(out.Report.ConftestViolations) == 0 {
		t.Fatal("expected conftest violations")
	}
}

func TestApplyIaC_Normal(t *testing.T) {
	h, sink := newTestHandler()
	mux := http.NewServeMux()
	h.Mount(mux)

	w := doPost(t, mux, "/v1/skills/apply-iac", ApplyIaCInput{
		AppID: "app-6",
		Slug:  "my-svc",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	var out ApplyIaCOutput
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(out.PRUrl, "https://github.com/") {
		t.Fatalf("unexpected pr_url: %s", out.PRUrl)
	}
	if strings.HasPrefix(out.PRTitle, "[BREAK-GLASS]") {
		t.Fatal("normal apply should not have break-glass prefix")
	}
	if len(sink.Events) == 0 || sink.Events[0].Type != "sdlc.iac.applied.v1" {
		t.Fatal("expected applied event")
	}
}

func TestApplyIaC_BreakGlass(t *testing.T) {
	h, sink := newTestHandler()
	mux := http.NewServeMux()
	h.Mount(mux)

	w := doPost(t, mux, "/v1/skills/apply-iac", ApplyIaCInput{
		AppID:      "app-7",
		Slug:       "critical-svc",
		BreakGlass: true,
		BreakGlassApprovals: []BreakGlassApproval{
			{ApproverRole: "security-admin", ApprovedBy: "sec-lead", Reason: "emergency patch"},
			{ApproverRole: "platform-admin", ApprovedBy: "plat-lead", Reason: "emergency patch"},
		},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	var out ApplyIaCOutput
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(out.PRTitle, "[BREAK-GLASS]") {
		t.Fatalf("expected break-glass prefix in title, got: %s", out.PRTitle)
	}
	if len(sink.Events) == 0 || sink.Events[0].Type != "sdlc.iac.break_glass_applied.v1" {
		t.Fatalf("expected break_glass_applied event, got: %v", sink.Events)
	}
}

func TestGenerateTerraform_GCPAndAzure(t *testing.T) {
	h, _ := newTestHandler()
	mux := http.NewServeMux()
	h.Mount(mux)

	for _, provider := range []string{ProviderGCP, ProviderAzure} {
		w := doPost(t, mux, "/v1/skills/generate-terraform", GenerateTerraformInput{
			AppID:    "app-8",
			Slug:     "multi-cloud",
			Provider: provider,
		})
		if w.Code != http.StatusOK {
			t.Fatalf("provider %s: expected 200, got %d body=%s", provider, w.Code, w.Body.String())
		}
	}
}
