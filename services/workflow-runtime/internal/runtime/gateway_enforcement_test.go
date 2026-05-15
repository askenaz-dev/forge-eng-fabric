package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// fakeMCPClient records every InvokeTool call and returns canned bytes.
type fakeMCPClient struct {
	mu       sync.Mutex
	calls    []fakeMCPCall
	body     []byte
	status   int
	err      error
}

type fakeMCPCall struct {
	AssetID  string
	ToolName string
	Body     any
}

func (f *fakeMCPClient) InvokeTool(_ context.Context, assetID, toolName string, body any) ([]byte, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeMCPCall{AssetID: assetID, ToolName: toolName, Body: body})
	if f.err != nil {
		return nil, 0, f.err
	}
	status := f.status
	if status == 0 {
		status = 200
	}
	bb := f.body
	if bb == nil {
		bb = []byte(`{"ok":true}`)
	}
	return bb, status, nil
}

func (f *fakeMCPClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// fakeA2AClient mirrors fakeMCPClient for the agent path.
type fakeA2AClient struct {
	mu     sync.Mutex
	calls  []string
	body   []byte
	status int
}

func (f *fakeA2AClient) Send(_ context.Context, assetID, _ string, _ any) ([]byte, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, assetID)
	bb := f.body
	if bb == nil {
		bb = []byte(`{"jsonrpc":"2.0","result":{"task_id":"t-1"}}`)
	}
	status := f.status
	if status == 0 {
		status = 200
	}
	return bb, status, nil
}

// mkWorkflow builds a one-step workflow that invokes the asset id with
// the given step type. Used by the enforcement tests.
func mkWorkflow(stepType ast.StepType, ref, tool string) *ast.Workflow {
	return &ast.Workflow{
		Metadata: ast.Metadata{ID: "wf-test", Version: "0.1.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{{
				ID:   "s1",
				Type: stepType,
				Ref:  ref,
				Tool: tool,
			}},
		},
	}
}

func waitForStatus(t *testing.T, e *InMemoryEngine, id string, status ExecutionStatus, why string) *Execution {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		exec, _ := e.GetExecution(context.Background(), "t1", id)
		if exec != nil && exec.Status == status {
			return exec
		}
		time.Sleep(5 * time.Millisecond)
	}
	exec, _ := e.GetExecution(context.Background(), "t1", id)
	t.Fatalf("execution did not reach %s within timeout (%s) — got %+v", status, why, exec)
	return nil
}

// --- 6.1 — MCP shim wiring ---------------------------------------------

func TestMCPActivityRoutesThroughGatewayClient(t *testing.T) {
	mcp := &fakeMCPClient{}
	reg := NewActivityRegistryWithOptions(RegistryOptions{MCP: mcp})
	sink := &MemorySink{}
	eng := NewInMemoryEngine(reg, sink)
	wf := mkWorkflow(ast.StepMCP, "vendor-mcp", "list_docs")
	exec, err := eng.StartWorkflow(context.Background(), StartRequest{
		TenantID: "t1", WorkspaceID: "ws1", Workflow: wf,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForStatus(t, eng, exec.ID, StatusCompleted, "mcp call should complete")
	if mcp.callCount() != 1 {
		t.Fatalf("expected exactly one MCP invocation; got %d", mcp.callCount())
	}
	if mcp.calls[0].AssetID != "vendor-mcp" || mcp.calls[0].ToolName != "list_docs" {
		t.Fatalf("unexpected call: %+v", mcp.calls[0])
	}
}

func TestMCPActivityFallsBackToStubWhenNotEnforced(t *testing.T) {
	reg := NewActivityRegistryWithOptions(RegistryOptions{Enforced: false})
	sink := &MemorySink{}
	eng := NewInMemoryEngine(reg, sink)
	wf := mkWorkflow(ast.StepMCP, "vendor-mcp", "list_docs")
	exec, _ := eng.StartWorkflow(context.Background(), StartRequest{
		TenantID: "t1", WorkspaceID: "ws1", Workflow: wf,
	})
	got := waitForStatus(t, eng, exec.ID, StatusCompleted, "stub path")
	if got.Status != StatusCompleted {
		t.Fatalf("stub path should complete; got %s reason=%s", got.Status, got.FailureReason)
	}
}

func TestMCPActivityFailsClosedWhenEnforcedWithoutClient(t *testing.T) {
	reg := NewActivityRegistryWithOptions(RegistryOptions{Enforced: true})
	sink := &MemorySink{}
	eng := NewInMemoryEngine(reg, sink)
	wf := mkWorkflow(ast.StepMCP, "vendor-mcp", "list_docs")
	exec, _ := eng.StartWorkflow(context.Background(), StartRequest{
		TenantID: "t1", WorkspaceID: "ws1", Workflow: wf,
	})
	got := waitForStatus(t, eng, exec.ID, StatusFailed, "enforced + no client should fail")
	if !strings.Contains(got.FailureReason, "gateway_client_missing") && !errors.Is(errors.New(got.FailureReason), ErrGatewayClientMissing) {
		t.Fatalf("expected gateway_client_missing failure; got %q", got.FailureReason)
	}
}

// --- 6.2 — A2A shim wiring ---------------------------------------------

func TestSubWorkflowRoutesAgentRefsThroughA2AClient(t *testing.T) {
	a2a := &fakeA2AClient{}
	reg := NewActivityRegistryWithOptions(RegistryOptions{A2A: a2a})
	sink := &MemorySink{}
	eng := NewInMemoryEngine(reg, sink)
	wf := mkWorkflow(ast.StepSubWorkflow, "agent-x", "tasks/send")
	exec, _ := eng.StartWorkflow(context.Background(), StartRequest{
		TenantID: "t1", WorkspaceID: "ws1", Workflow: wf,
	})
	waitForStatus(t, eng, exec.ID, StatusCompleted, "a2a call should complete")
	a2a.mu.Lock()
	defer a2a.mu.Unlock()
	if len(a2a.calls) != 1 || a2a.calls[0] != "agent-x" {
		t.Fatalf("expected one a2a call to agent-x; got %+v", a2a.calls)
	}
}

// --- 6.3 — selected_assets enforcement ---------------------------------

func TestSelectedAssetsAllowedAssetSucceeds(t *testing.T) {
	mcp := &fakeMCPClient{}
	reg := NewActivityRegistryWithOptions(RegistryOptions{MCP: mcp})
	sink := &MemorySink{}
	eng := NewInMemoryEngine(reg, sink)
	wf := mkWorkflow(ast.StepMCP, "github", "create_pr")
	exec, _ := eng.StartWorkflow(context.Background(), StartRequest{
		TenantID: "t1", WorkspaceID: "ws1", Workflow: wf,
		SelectedAssets: &SelectedAssets{MCPs: []string{"github", "jira"}},
	})
	waitForStatus(t, eng, exec.ID, StatusCompleted, "pinned asset should run")
	if got := len(sink.ByType(EventGuardrailTrip)); got != 0 {
		t.Fatalf("expected no guardrail trip events; got %d", got)
	}
}

func TestSelectedAssetsOffPinAssetRejectedWithGuardrail(t *testing.T) {
	mcp := &fakeMCPClient{}
	reg := NewActivityRegistryWithOptions(RegistryOptions{MCP: mcp})
	sink := &MemorySink{}
	eng := NewInMemoryEngine(reg, sink)
	wf := mkWorkflow(ast.StepMCP, "rogue-mcp", "delete_everything")
	exec, _ := eng.StartWorkflow(context.Background(), StartRequest{
		TenantID: "t1", WorkspaceID: "ws1", Workflow: wf,
		SelectedAssets: &SelectedAssets{MCPs: []string{"github", "jira"}},
	})
	got := waitForStatus(t, eng, exec.ID, StatusFailed, "off-pin should fail")
	if !strings.Contains(got.FailureReason, "asset_not_pinned") {
		t.Fatalf("expected asset_not_pinned; got %q", got.FailureReason)
	}
	trips := sink.ByType(EventGuardrailTrip)
	if len(trips) == 0 {
		t.Fatal("expected guardrail.trip.v1 event")
	}
	reason, _ := trips[len(trips)-1].Data["reason"].(string)
	if reason != "asset_not_in_pinned_set" {
		t.Fatalf("expected reason=asset_not_in_pinned_set; got %q", reason)
	}
	if mcp.callCount() != 0 {
		t.Fatalf("off-pin step must not have reached the gateway; got %d calls", mcp.callCount())
	}
}

func TestSelectedAssetsEmptyPreservesCurrentBehavior(t *testing.T) {
	mcp := &fakeMCPClient{}
	reg := NewActivityRegistryWithOptions(RegistryOptions{MCP: mcp})
	sink := &MemorySink{}
	eng := NewInMemoryEngine(reg, sink)
	wf := mkWorkflow(ast.StepMCP, "any-mcp", "tool")
	exec, _ := eng.StartWorkflow(context.Background(), StartRequest{
		TenantID: "t1", WorkspaceID: "ws1", Workflow: wf,
		// SelectedAssets nil — no pinning
	})
	waitForStatus(t, eng, exec.ID, StatusCompleted, "no pinning means no enforcement")
	if mcp.callCount() != 1 {
		t.Fatalf("expected call to reach gateway; got %d", mcp.callCount())
	}
}

func TestSelectedAssetsFamilyAwareEnforcement(t *testing.T) {
	// MCP step with an asset_id only listed under Skills should still be
	// rejected — pinning is family-scoped.
	mcp := &fakeMCPClient{}
	reg := NewActivityRegistryWithOptions(RegistryOptions{MCP: mcp})
	sink := &MemorySink{}
	eng := NewInMemoryEngine(reg, sink)
	wf := mkWorkflow(ast.StepMCP, "git", "tool")
	exec, _ := eng.StartWorkflow(context.Background(), StartRequest{
		TenantID: "t1", WorkspaceID: "ws1", Workflow: wf,
		SelectedAssets: &SelectedAssets{Skills: []string{"git"}}, // matching id, wrong family
	})
	got := waitForStatus(t, eng, exec.ID, StatusFailed, "family mismatch should still fail")
	if !strings.Contains(got.FailureReason, "asset_not_pinned") {
		t.Fatalf("expected asset_not_pinned for family mismatch; got %q", got.FailureReason)
	}
}
