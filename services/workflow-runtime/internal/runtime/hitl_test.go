package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

type captureAudit struct {
	mu      sync.Mutex
	entries []AuditEntry
}

func (c *captureAudit) Log(_ context.Context, e AuditEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, e)
	return nil
}

func (c *captureAudit) snapshot() []AuditEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]AuditEntry(nil), c.entries...)
}

func TestHITLAuditRecordsModifiedInputs(t *testing.T) {
	audit := &captureAudit{}
	engine := NewInMemoryEngine(NewActivityRegistry(nil), &MemorySink{})
	engine.SetAuditLogger(audit)
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf-h", Name: "h", Version: "1.0.0"},
		Spec: ast.Spec{
			Inputs: []ast.IOField{{Name: "title", Type: "string"}},
			Steps: []ast.Step{
				{ID: "h", Type: ast.StepHumanInLoop, ApproverRole: "po", Timeout: "5s",
					Inputs: map[string]any{"title": "$inputs.title"}},
			},
		},
	}
	exec, err := engine.StartWorkflow(context.Background(), StartRequest{
		TenantID: "t", Workflow: wf, Inputs: map[string]any{"title": "draft"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, _ := engine.GetExecution(context.Background(), "t", exec.ID)
		if got != nil && got.Status == StatusWaiting {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := engine.SignalWorkflow(context.Background(), SignalRequest{
		TenantID: "t", ExecutionID: exec.ID, Signal: "approve",
		Payload: map[string]any{
			"by":     "alice",
			"inputs": map[string]any{"title": "approved-title"},
		},
	}); err != nil {
		t.Fatalf("signal: %v", err)
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, _ := engine.GetExecution(context.Background(), "t", exec.ID)
		if got != nil && got.Status == StatusCompleted {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	entries := audit.snapshot()
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Decision != "approved" {
		t.Fatalf("decision: %s", e.Decision)
	}
	if e.OriginalInputs["title"] != "draft" {
		t.Fatalf("original: %+v", e.OriginalInputs)
	}
	if e.FinalInputs["title"] != "approved-title" {
		t.Fatalf("final: %+v", e.FinalInputs)
	}
	if e.InputDiff["title"] == nil {
		t.Fatalf("diff missing")
	}
}

func TestHITLEscalateOnTimeout(t *testing.T) {
	audit := &captureAudit{}
	engine := NewInMemoryEngine(NewActivityRegistry(nil), &MemorySink{})
	engine.SetAuditLogger(audit)
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf-h", Name: "h", Version: "1.0.0"},
		Spec: ast.Spec{Steps: []ast.Step{
			{ID: "h", Type: ast.StepHumanInLoop, ApproverRole: "po",
				Timeout: "10ms", OnTimeout: "escalate", EscalationRole: "em"},
		}},
	}
	exec, err := engine.StartWorkflow(context.Background(), StartRequest{TenantID: "t", Workflow: wf})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := engine.GetExecution(context.Background(), "t", exec.ID)
		if got != nil && got.Status == StatusFailed {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	got, _ := engine.GetExecution(context.Background(), "t", exec.ID)
	if got.Status != StatusFailed {
		t.Fatalf("status: %s", got.Status)
	}
	entries := audit.snapshot()
	if len(entries) != 1 || entries[0].Decision != "escalated" {
		t.Fatalf("audit entries: %+v", entries)
	}
}
