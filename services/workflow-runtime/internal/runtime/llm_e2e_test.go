package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// Tests for the LLM step executor wired end-to-end against stub
// PromptRenderer + ModelResolver + LLMProvider collaborators.

type fakeRenderer struct {
	rendered RenderedPrompt
	called   bool
}

func (f *fakeRenderer) Render(_ context.Context, _ string, _ map[string]any) (RenderedPrompt, error) {
	f.called = true
	return f.rendered, nil
}

type fakeResolver struct {
	resolved ResolvedModel
	called   bool
}

func (f *fakeResolver) Resolve(_ context.Context, _, _ string) (ResolvedModel, error) {
	f.called = true
	return f.resolved, nil
}

type fakeProvider struct {
	calls []CompletionRequest
	resp  CompletionResponse
}

func (f *fakeProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	f.calls = append(f.calls, req)
	return f.resp, nil
}

func newLLMEngine(opts LLMOptions, sink Sink) *InMemoryEngine {
	reg := NewActivityRegistryWithOptions(RegistryOptions{LLM: opts, EventSink: sink})
	return NewInMemoryEngine(reg, sink)
}

func TestLLMStepWiredEndToEnd(t *testing.T) {
	renderer := &fakeRenderer{rendered: RenderedPrompt{System: "be brief", User: "hello", TokenEstimate: 5}}
	resolver := &fakeResolver{resolved: ResolvedModel{ModelID: "stub-1", Provider: "stub", PricingPerToken: 0.0001}}
	provider := &fakeProvider{resp: CompletionResponse{
		OutputJSON:   map[string]any{"category": "billing", "draft": "yo"},
		InputTokens:  10,
		OutputTokens: 5,
	}}
	sink := &MemorySink{}
	eng := newLLMEngine(LLMOptions{Renderer: renderer, Resolver: resolver, Provider: provider}, sink)
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{{
				ID:             "think",
				Type:           ast.StepLLM,
				PromptTemplate: "registry:prompt/x/y@1.0.0",
				Model:          &ast.ModelBinding{Ref: "gateway:model/stub-1@latest-stable"},
				StepOutputs:    map[string]string{"category": "string", "draft": "string"},
			}},
		},
	}
	ctx := context.Background()
	exec, err := eng.StartWorkflow(ctx, StartRequest{TenantID: "t1", WorkspaceID: "w1", Workflow: wf})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, func() bool {
		got, _ := eng.GetExecution(ctx, "t1", exec.ID)
		return got != nil && got.Status == StatusCompleted
	})
	if !renderer.called || !resolver.called {
		t.Errorf("renderer/resolver not called: r=%v res=%v", renderer.called, resolver.called)
	}
	if len(provider.calls) != 1 {
		t.Errorf("expected one provider call, got %d", len(provider.calls))
	}
	got, _ := eng.GetExecution(ctx, "t1", exec.ID)
	for _, s := range got.Steps {
		if s.StepID == "think" && s.Status == StepStatusCompleted {
			if s.Outputs["category"] != "billing" {
				t.Errorf("outputs.category: %v", s.Outputs["category"])
			}
			meta, _ := s.Outputs["_meta"].(map[string]any)
			if meta == nil {
				t.Errorf("missing _meta on outputs")
			} else {
				if meta["model"] != "stub-1" {
					t.Errorf("_meta.model: %v", meta["model"])
				}
			}
		}
	}
}

func TestLLMStepFailsWhenOutputSchemaMissed(t *testing.T) {
	renderer := &fakeRenderer{}
	resolver := &fakeResolver{}
	provider := &fakeProvider{resp: CompletionResponse{
		OutputJSON: map[string]any{"category": "billing"}, // missing "draft"
	}}
	sink := &MemorySink{}
	eng := newLLMEngine(LLMOptions{Renderer: renderer, Resolver: resolver, Provider: provider}, sink)
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{Steps: []ast.Step{{
			ID: "think", Type: ast.StepLLM,
			PromptTemplate: "registry:prompt/x/y@1.0.0",
			Model:          &ast.ModelBinding{Ref: "gateway:model/x@1"},
			StepOutputs:    map[string]string{"category": "string", "draft": "string"},
		}}},
	}
	ctx := context.Background()
	exec, _ := eng.StartWorkflow(ctx, StartRequest{TenantID: "t1", WorkspaceID: "w1", Workflow: wf})
	waitFor(t, func() bool {
		got, _ := eng.GetExecution(ctx, "t1", exec.ID)
		return got != nil && got.Status == StatusFailed
	})
	got, _ := eng.GetExecution(ctx, "t1", exec.ID)
	if !strings.Contains(got.FailureReason, "missing declared output") {
		t.Errorf("expected schema mismatch in failure: %q", got.FailureReason)
	}
}

func TestLLMStepEmitsBudgetExhausted(t *testing.T) {
	renderer := &fakeRenderer{}
	resolver := &fakeResolver{}
	provider := &fakeProvider{resp: CompletionResponse{
		ToolCalls: []ToolCall{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}},
		Text:      "wanted to call too many tools",
	}}
	sink := &MemorySink{}
	eng := newLLMEngine(LLMOptions{Renderer: renderer, Resolver: resolver, Provider: provider}, sink)
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{Steps: []ast.Step{{
			ID: "think", Type: ast.StepLLM,
			PromptTemplate: "registry:prompt/x/y@1.0.0",
			Model:          &ast.ModelBinding{Ref: "gateway:model/x@1"},
			MaxToolCalls:   2,
		}}},
	}
	ctx := context.Background()
	exec, _ := eng.StartWorkflow(ctx, StartRequest{TenantID: "t1", WorkspaceID: "w1", Workflow: wf})
	waitFor(t, func() bool {
		got, _ := eng.GetExecution(ctx, "t1", exec.ID)
		return got != nil && (got.Status == StatusCompleted || got.Status == StatusFailed)
	})
	if got := sink.ByType("workflow.llm.budget_exhausted.v1"); len(got) != 1 {
		t.Fatalf("expected one budget_exhausted event, got %d", len(got))
	}
}

func TestStartWorkflowEmitsCauseTriggerID(t *testing.T) {
	sink := &MemorySink{}
	eng := NewInMemoryEngine(NewActivityRegistry(nil), sink)
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Triggers: []ast.Trigger{{ID: "src", Type: ast.TriggerWebhookIn}},
			Steps:    []ast.Step{{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/x/y@1.0.0"}},
		},
	}
	exec, err := eng.StartWorkflow(context.Background(), StartRequest{
		TenantID: "t1", WorkspaceID: "w1", Workflow: wf, DryRun: true,
		TriggerEvent: &TriggerEvent{TriggerID: "src", FiredAt: time.Now()},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	_ = exec
	events := sink.ByType(EventExecutionStarted)
	if len(events) == 0 {
		t.Fatal("no started event emitted")
	}
	cause, _ := events[0].Data["cause"].(map[string]any)
	if cause == nil {
		t.Fatal("started event missing cause field")
	}
	if cause["trigger_id"] != "src" {
		t.Errorf("cause.trigger_id: %v", cause["trigger_id"])
	}
}

func TestDropConcurrencyRefusesSecondFire(t *testing.T) {
	// Stall the first execution by pointing it at a step that waits.
	// Easiest: use a fake activity that blocks until released.
	released := make(chan struct{})
	hold := &holdActivity{ready: make(chan struct{}), release: released}
	reg := NewActivityRegistry(nil)
	reg.Register(ast.StepSkill, hold)

	sink := &MemorySink{}
	eng := NewInMemoryEngine(reg, sink)
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Triggers: []ast.Trigger{{ID: "src", Type: ast.TriggerWebhookIn, Concurrency: ast.TriggerConcurrencyDrop}},
			Steps:    []ast.Step{{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/x/y@1.0.0"}},
		},
	}
	ctx := context.Background()
	if _, err := eng.StartWorkflow(ctx, StartRequest{
		TenantID: "t1", WorkspaceID: "w1", Workflow: wf,
		TriggerEvent: &TriggerEvent{TriggerID: "src", FiredAt: time.Now()},
	}); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	// Wait for the first execution to enter the hold.
	<-hold.ready
	// Second fire should be refused.
	_, err := eng.StartWorkflow(ctx, StartRequest{
		TenantID: "t1", WorkspaceID: "w1", Workflow: wf,
		TriggerEvent: &TriggerEvent{TriggerID: "src", FiredAt: time.Now()},
	})
	if err == nil {
		t.Fatalf("expected drop_concurrency, got nil")
	}
	if !strings.Contains(err.Error(), "drop_concurrency") {
		t.Errorf("expected drop_concurrency, got %v", err)
	}
	// Let the first finish so the test exits cleanly.
	close(released)
}

// holdActivity blocks on Execute until release is closed. ready is
// closed when Execute is entered.
type holdActivity struct {
	ready   chan struct{}
	release chan struct{}
	once    bool
}

func (h *holdActivity) Execute(_ context.Context, _ ActivityInput) (ActivityOutput, error) {
	if !h.once {
		h.once = true
		close(h.ready)
	}
	<-h.release
	return ActivityOutput{Outputs: map[string]any{"ok": true}}, nil
}
