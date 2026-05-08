package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

func waitFor(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within deadline")
}

func newTestEngine() (*InMemoryEngine, *MemorySink) {
	sink := &MemorySink{}
	e := NewInMemoryEngine(NewActivityRegistry(nil), sink)
	return e, sink
}

func sampleWorkflow() *ast.Workflow {
	return &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf-1", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Inputs: []ast.IOField{{Name: "story", Type: "string"}},
			Steps: []ast.Step{
				{ID: "refine", Type: ast.StepSkill, Ref: "registry:skill/x/y@1.0.0", Inputs: map[string]any{"story": "$inputs.story"}},
				{ID: "open-pr", Type: ast.StepMCP, Ref: "registry:mcp/github@write", Tool: "create_pr", DependsOn: []string{"refine"}},
			},
		},
	}
}

func TestStartWorkflowDryRunCompletes(t *testing.T) {
	e, sink := newTestEngine()
	exec, err := e.StartWorkflow(context.Background(), StartRequest{
		TenantID:    "t1",
		WorkspaceID: "w1",
		Workflow:    sampleWorkflow(),
		Inputs:      map[string]any{"story": "hello"},
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, func() bool {
		got, _ := e.GetExecution(context.Background(), "t1", exec.ID)
		return got != nil && got.Status == StatusCompleted
	})
	got, _ := e.GetExecution(context.Background(), "t1", exec.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("status: %s", got.Status)
	}
	completedEvents := sink.ByType(EventStepCompleted)
	if len(completedEvents) < 2 {
		t.Fatalf("expected step completed events, got %d", len(completedEvents))
	}
}

func TestCrossTenantAccessDenied(t *testing.T) {
	e, sink := newTestEngine()
	exec, _ := e.StartWorkflow(context.Background(), StartRequest{
		TenantID: "tA", WorkspaceID: "w", Workflow: sampleWorkflow(), DryRun: true,
	})
	waitFor(t, func() bool {
		got, _ := e.GetExecution(context.Background(), "tA", exec.ID)
		return got != nil && got.Status == StatusCompleted
	})
	_, err := e.GetExecution(context.Background(), "tB", exec.ID)
	if err != ErrCrossTenantAccess {
		t.Fatalf("expected cross_tenant_access, got %v", err)
	}
	guardrails := sink.ByType(EventGuardrailTrip)
	if len(guardrails) == 0 {
		t.Fatalf("expected guardrail trip event")
	}
}

func TestHumanInTheLoopWaitsAndResumes(t *testing.T) {
	e, sink := newTestEngine()
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf-h", Name: "wf-h", Version: "1.0.0"},
		Spec: ast.Spec{Steps: []ast.Step{
			{ID: "approve", Type: ast.StepHumanInLoop, ApproverRole: "product-owner", Timeout: "5s", OnTimeout: "fail"},
		}},
	}
	exec, err := e.StartWorkflow(context.Background(), StartRequest{TenantID: "t", Workflow: wf})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, func() bool {
		got, _ := e.GetExecution(context.Background(), "t", exec.ID)
		return got != nil && got.Status == StatusWaiting
	})
	if _, err := e.SignalWorkflow(context.Background(), SignalRequest{TenantID: "t", ExecutionID: exec.ID, Signal: "approve", Payload: map[string]any{"by": "alice"}}); err != nil {
		t.Fatalf("signal: %v", err)
	}
	waitFor(t, func() bool {
		got, _ := e.GetExecution(context.Background(), "t", exec.ID)
		return got != nil && got.Status == StatusCompleted
	})
	if len(sink.ByType(EventStepWaitingHuman)) == 0 {
		t.Fatalf("expected waiting_human event")
	}
}
