package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// Tests for the trigger-payload binding and stub LLM activity introduced
// by the ai-flow-authoring change.

func TestStartWorkflowBindsTriggerPayloadToStepInputs(t *testing.T) {
	engine := NewInMemoryEngine(NewActivityRegistry(nil), &MemorySink{})
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Triggers: []ast.Trigger{
				{ID: "email", Type: ast.TriggerEmailInbound, Outputs: map[string]string{"from": "string", "body": "string"}},
			},
			Steps: []ast.Step{
				{ID: "log", Type: ast.StepSkill, Ref: "registry:skill/x/y@1.0.0",
					Inputs: map[string]any{"author": "$triggers.email.from"}},
			},
		},
	}
	ctx := context.Background()
	exec, err := engine.StartWorkflow(ctx, StartRequest{
		TenantID:    "t1",
		WorkspaceID: "w1",
		Workflow:    wf,
		DryRun:      true,
		TriggerEvent: &TriggerEvent{
			TriggerID: "email",
			FiredAt:   time.Now(),
			Payload:   map[string]any{"from": "alice@acme.com", "body": "hello"},
		},
	})
	if err != nil {
		t.Fatalf("StartWorkflow: %v", err)
	}
	waitFor(t, func() bool {
		got, _ := engine.GetExecution(ctx, "t1", exec.ID)
		return got != nil && got.Status == StatusCompleted
	})
	got, _ := engine.GetExecution(ctx, "t1", exec.ID)
	var seenInputs map[string]any
	for _, ev := range got.Steps {
		if ev.StepID == "log" {
			seenInputs = ev.Inputs
		}
	}
	if seenInputs == nil {
		t.Fatalf("no step event for `log`: %+v", got.Steps)
	}
	if v := seenInputs["author"]; v != "alice@acme.com" {
		t.Errorf("step `log` author: got %v want alice@acme.com", v)
	}
}

func TestStartWorkflowFailsStepOnUnboundTriggerReference(t *testing.T) {
	engine := NewInMemoryEngine(NewActivityRegistry(nil), &MemorySink{})
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{
				{ID: "log", Type: ast.StepSkill, Ref: "registry:skill/x/y@1.0.0",
					Inputs: map[string]any{"author": "$triggers.email.from"}},
			},
		},
	}
	ctx := context.Background()
	exec, err := engine.StartWorkflow(ctx, StartRequest{
		TenantID: "t1", WorkspaceID: "w1", Workflow: wf, DryRun: true,
	})
	if err != nil {
		t.Fatalf("StartWorkflow: %v", err)
	}
	waitFor(t, func() bool {
		got, _ := engine.GetExecution(ctx, "t1", exec.ID)
		return got != nil && got.Status == StatusFailed
	})
	got, _ := engine.GetExecution(ctx, "t1", exec.ID)
	if got.Status != StatusFailed {
		t.Fatalf("expected failed, got %q", got.Status)
	}
	if !strings.Contains(got.FailureReason, "unbound_trigger_reference") {
		t.Errorf("expected unbound_trigger_reference in failure reason, got %q", got.FailureReason)
	}
	if !strings.Contains(got.FailureReason, "$triggers.email.from") {
		t.Errorf("failure reason should mention the offending reference: %q", got.FailureReason)
	}
	_ = errors.New // keep import
}

func TestLLMStepReturnsDryRunOutputsInDryMode(t *testing.T) {
	engine := NewInMemoryEngine(NewActivityRegistry(nil), &MemorySink{})
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{{
				ID:             "think",
				Type:           ast.StepLLM,
				PromptTemplate: "registry:prompt/foo/bar@1.0.0",
				Model:          &ast.ModelBinding{Ref: "gateway:model/x@1.0.0"},
				StepOutputs:    map[string]string{"category": "string"},
			}},
		},
	}
	ctx := context.Background()
	exec, err := engine.StartWorkflow(ctx, StartRequest{
		TenantID: "t1", WorkspaceID: "w1", Workflow: wf, DryRun: true,
	})
	if err != nil {
		t.Fatalf("StartWorkflow dry-run LLM: %v", err)
	}
	waitFor(t, func() bool {
		got, _ := engine.GetExecution(ctx, "t1", exec.ID)
		return got != nil && got.Status == StatusCompleted
	})
	got, _ := engine.GetExecution(ctx, "t1", exec.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed, got %q", got.Status)
	}
}

func TestLLMStepFailsNonDryRunWithStepTypeNotYetImplemented(t *testing.T) {
	engine := NewInMemoryEngine(NewActivityRegistry(nil), &MemorySink{})
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{{
				ID:             "think",
				Type:           ast.StepLLM,
				PromptTemplate: "registry:prompt/foo/bar@1.0.0",
				Model:          &ast.ModelBinding{Ref: "gateway:model/x@1.0.0"},
			}},
		},
	}
	ctx := context.Background()
	exec, err := engine.StartWorkflow(ctx, StartRequest{
		TenantID: "t1", WorkspaceID: "w1", Workflow: wf, DryRun: false,
	})
	if err != nil {
		t.Fatalf("StartWorkflow: %v", err)
	}
	waitFor(t, func() bool {
		got, _ := engine.GetExecution(ctx, "t1", exec.ID)
		return got != nil && got.Status == StatusFailed
	})
	got, _ := engine.GetExecution(ctx, "t1", exec.ID)
	if !strings.Contains(got.FailureReason, "step_type_not_yet_implemented") {
		t.Errorf("expected step_type_not_yet_implemented in failure reason, got %q", got.FailureReason)
	}
}

func TestNewStepTypesRegisteredInActivityRegistry(t *testing.T) {
	reg := NewActivityRegistry(nil)
	cases := []ast.StepType{
		ast.StepLLM, ast.StepAgent, ast.StepPromptTemplate,
		ast.StepWebhookOut, ast.StepGithubAction, ast.StepDeployAction,
		ast.StepApprovalAction, ast.StepNotificationAction, ast.StepEval,
		ast.StepCustom,
	}
	for _, tc := range cases {
		if _, err := reg.Resolve(tc); err != nil {
			t.Errorf("Resolve(%q): %v", tc, err)
		}
	}
}
