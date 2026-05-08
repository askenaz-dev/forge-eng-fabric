package drift

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunOnceCreatesFindingAndEventForExitCodeTwo(t *testing.T) {
	store := NewStore()
	sink := &MemorySink{}
	svc := NewService(store, sink)
	svc.Planner = fakePlanner{result: PlanResult{ExitCode: 2, Changes: []PlanChange{{Resource: "google_container_node_pool.main", Field: "node_count", Expected: "3", Actual: "5"}}}}
	store.UpsertWorkspace(Workspace{ID: "iac-1", TenantID: "tenant-a", WorkspaceID: "ws-1", RuntimeID: "rt-1", RepoPath: t.TempDir()})
	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.Findings()) != 1 {
		t.Fatalf("expected one finding, got %d", len(store.Findings()))
	}
	if len(sink.ByType("iac.drift.detected.v1")) != 1 {
		t.Fatalf("expected iac.drift.detected.v1")
	}
}

func TestIAMDriftClassifiedHigh(t *testing.T) {
	sev := classify(PlanChange{Resource: "google_project_iam_binding.deploy", Field: "members"})
	if sev != SeverityHigh {
		t.Fatalf("expected high severity, got %s", sev)
	}
}

func TestRejectSuppressionWithoutExpiration(t *testing.T) {
	_, err := ParseIgnoreFile([]byte(`suppressions:
  - resource_pattern: "*"
    field_pattern: "labels.*"
    reason: "accepted label drift"
`))
	if !errors.Is(err, ErrSuppressionMissingExpiration) {
		t.Fatalf("expected suppression_missing_expiration, got %v", err)
	}
}

func TestSuppressionSuppressesFinding(t *testing.T) {
	dir := t.TempDir()
	ignore := `suppressions:
  - resource_pattern: "google_container_node_pool.*"
    field_pattern: "node_count"
    reason: "temporary autoscaler test"
    expires_at: "2099-01-01T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(dir, ".forge-drift-ignore.yaml"), []byte(ignore), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore()
	sink := &MemorySink{}
	svc := NewService(store, sink)
	svc.Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	svc.Planner = fakePlanner{result: PlanResult{ExitCode: 2, Changes: []PlanChange{{Resource: "google_container_node_pool.main", Field: "node_count"}}}}
	store.UpsertWorkspace(Workspace{ID: "iac-1", TenantID: "tenant-a", WorkspaceID: "ws-1", RuntimeID: "rt-1", RepoPath: dir})
	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !store.Findings()[0].Suppressed {
		t.Fatalf("expected finding suppressed")
	}
	if len(sink.ByType("iac.drift.detected.v1")) != 0 {
		t.Fatalf("suppressed finding should not emit detected event")
	}
}

func TestArtificialDriftTriggersRemediationProposal(t *testing.T) {
	store := NewStore()
	sink := &MemorySink{}
	svc := NewService(store, sink)
	svc.Planner = fakePlanner{result: PlanResult{ExitCode: 2, Changes: []PlanChange{{Resource: "google_container_node_pool.main", Field: "node_count", Expected: "3", Actual: "5"}}}}
	svc.Remediator = fakeRemediator{url: "https://github.com/forge/infra/pull/123"}
	store.UpsertWorkspace(Workspace{ID: "iac-1", TenantID: "tenant-a", WorkspaceID: "ws-1", RuntimeID: "rt-1", RepoPath: t.TempDir()})
	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sink.ByType("iac.drift.detected.v1")) != 1 {
		t.Fatalf("expected detection event")
	}
	if len(sink.ByType("iac.drift.remediation.proposed.v1")) != 1 {
		t.Fatalf("expected remediation proposed event")
	}
	findings := store.Findings()
	if findings[0].RemediationPRURL != "https://github.com/forge/infra/pull/123" {
		t.Fatalf("expected remediation PR URL on finding, got %+v", findings[0])
	}
}

type fakePlanner struct {
	result PlanResult
	err    error
}

func (f fakePlanner) Plan(context.Context, Workspace) (PlanResult, error) {
	return f.result, f.err
}

type fakeRemediator struct {
	url string
	err error
}

func (f fakeRemediator) Propose(context.Context, Finding) (string, error) {
	return f.url, f.err
}
