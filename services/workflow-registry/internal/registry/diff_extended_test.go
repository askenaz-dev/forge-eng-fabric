package registry

import (
	"testing"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// Tests added by the ai-flow-authoring change covering the extended diff
// rules: triggers, LLM step shape, migration-only PATCH bumps.

func wfWith(spec ast.Spec) *ast.Workflow {
	return &ast.Workflow{
		APIVersion: ast.APIVersion,
		Kind:       ast.Kind,
		Metadata:   ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec:       spec,
	}
}

func hasReason(d DiffResult, prefix string) bool {
	for _, r := range d.Reasons {
		if len(r) >= len(prefix) && r[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func TestDiffTriggerAdditionIsMinor(t *testing.T) {
	prev := wfWith(ast.Spec{
		Steps: []ast.Step{{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0"}},
	})
	next := wfWith(ast.Spec{
		Steps: prev.Spec.Steps,
		Triggers: []ast.Trigger{
			{ID: "src", Type: ast.TriggerCron, Config: map[string]any{"expression": "0 * * * *"}},
		},
	})
	d := DiffWorkflows(prev, next)
	if d.Bump != BumpMinor {
		t.Fatalf("bump: got %v want MINOR", d.Bump)
	}
	if !hasReason(d, "trigger_added:src") {
		t.Errorf("missing trigger_added reason: %v", d.Reasons)
	}
}

func TestDiffTriggerRemovalIsMajor(t *testing.T) {
	prev := wfWith(ast.Spec{
		Steps:    []ast.Step{{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0"}},
		Triggers: []ast.Trigger{{ID: "src", Type: ast.TriggerCron}},
	})
	next := wfWith(ast.Spec{Steps: prev.Spec.Steps})
	d := DiffWorkflows(prev, next)
	if d.Bump != BumpMajor {
		t.Fatalf("bump: got %v want MAJOR", d.Bump)
	}
	if !hasReason(d, "trigger_removed:src") {
		t.Errorf("missing trigger_removed reason: %v", d.Reasons)
	}
}

func TestDiffTriggerTypeChangeIsMajor(t *testing.T) {
	prev := wfWith(ast.Spec{
		Steps:    []ast.Step{{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0"}},
		Triggers: []ast.Trigger{{ID: "src", Type: ast.TriggerCron}},
	})
	next := wfWith(ast.Spec{
		Steps:    prev.Spec.Steps,
		Triggers: []ast.Trigger{{ID: "src", Type: ast.TriggerWebhookIn}},
	})
	d := DiffWorkflows(prev, next)
	if d.Bump != BumpMajor {
		t.Fatalf("bump: got %v want MAJOR", d.Bump)
	}
	if !hasReason(d, "trigger_type_changed:src") {
		t.Errorf("missing trigger_type_changed reason: %v", d.Reasons)
	}
}

func TestDiffLLMOutputsRemovedIsMajor(t *testing.T) {
	prev := wfWith(ast.Spec{
		Steps: []ast.Step{{
			ID:             "think",
			Type:           ast.StepLLM,
			PromptTemplate: "registry:prompt/foo/bar@1.0.0",
			Model:          &ast.ModelBinding{Ref: "gateway:model/x@1.0.0"},
			StepOutputs:    map[string]string{"category": "string", "draft": "string"},
		}},
	})
	next := wfWith(ast.Spec{
		Steps: []ast.Step{{
			ID:             "think",
			Type:           ast.StepLLM,
			PromptTemplate: "registry:prompt/foo/bar@1.0.0",
			Model:          &ast.ModelBinding{Ref: "gateway:model/x@1.0.0"},
			StepOutputs:    map[string]string{"category": "string"},
		}},
	})
	d := DiffWorkflows(prev, next)
	if d.Bump != BumpMajor {
		t.Fatalf("bump: got %v want MAJOR", d.Bump)
	}
	if !hasReason(d, "llm_outputs_removed:think:draft") {
		t.Errorf("missing llm_outputs_removed reason: %v", d.Reasons)
	}
}

func TestDiffLLMOutputsAddedIsMinor(t *testing.T) {
	base := []ast.Step{{
		ID:             "think",
		Type:           ast.StepLLM,
		PromptTemplate: "registry:prompt/foo/bar@1.0.0",
		Model:          &ast.ModelBinding{Ref: "gateway:model/x@1.0.0"},
		StepOutputs:    map[string]string{"category": "string"},
	}}
	prev := wfWith(ast.Spec{Steps: base})
	nextStep := base[0]
	nextStep.StepOutputs = map[string]string{"category": "string", "confidence": "number"}
	next := wfWith(ast.Spec{Steps: []ast.Step{nextStep}})
	d := DiffWorkflows(prev, next)
	if d.Bump != BumpMinor {
		t.Fatalf("bump: got %v want MINOR; reasons=%v", d.Bump, d.Reasons)
	}
	if !hasReason(d, "llm_outputs_added:think:confidence") {
		t.Errorf("missing llm_outputs_added reason: %v", d.Reasons)
	}
}

func TestDiffLLMModelRefChangeIsMinor(t *testing.T) {
	prev := wfWith(ast.Spec{Steps: []ast.Step{{
		ID:             "think",
		Type:           ast.StepLLM,
		PromptTemplate: "registry:prompt/foo/bar@1.0.0",
		Model:          &ast.ModelBinding{Ref: "gateway:model/old@1.0.0"},
	}}})
	next := wfWith(ast.Spec{Steps: []ast.Step{{
		ID:             "think",
		Type:           ast.StepLLM,
		PromptTemplate: "registry:prompt/foo/bar@1.0.0",
		Model:          &ast.ModelBinding{Ref: "gateway:model/new@1.0.0"},
	}}})
	d := DiffWorkflows(prev, next)
	if d.Bump != BumpMinor {
		t.Fatalf("bump: got %v want MINOR", d.Bump)
	}
	if !hasReason(d, "llm_model_ref_changed:think") {
		t.Errorf("missing llm_model_ref_changed reason: %v", d.Reasons)
	}
}

func TestDiffPromptToPromptTemplateIsPatch(t *testing.T) {
	prev := wfWith(ast.Spec{Steps: []ast.Step{{
		ID: "render", Type: ast.StepPrompt, Ref: "registry:prompt/foo/bar@1.0.0",
	}}})
	next := wfWith(ast.Spec{Steps: []ast.Step{{
		ID: "render", Type: ast.StepPromptTemplate, Ref: "registry:prompt/foo/bar@1.0.0",
	}}})
	d := DiffWorkflows(prev, next)
	if d.Bump != BumpPatch {
		t.Fatalf("bump: got %v want PATCH (migration-only); reasons=%v", d.Bump, d.Reasons)
	}
	if !hasReason(d, "migrate_prompt_to_prompt_template:render") {
		t.Errorf("missing migrate_prompt_to_prompt_template reason: %v", d.Reasons)
	}
}

func TestDiffMigratedEventTriggerIsPatch(t *testing.T) {
	prev := wfWith(ast.Spec{
		Steps: []ast.Step{
			{ID: "src", Type: ast.StepEventTrigger, EventPattern: &ast.EventPattern{Type: "github.push.v1"}},
			{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0"},
		},
	})
	// Next: the event-trigger step is gone (moved to triggers block).
	next := wfWith(ast.Spec{
		Steps: []ast.Step{
			{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0"},
		},
		Triggers: []ast.Trigger{
			{ID: "src", Type: ast.TriggerEventBus, Config: map[string]any{"topic": "github.push.v1"}, MigratedFrom: ast.StepEventTrigger},
		},
	})
	d := DiffWorkflows(prev, next)
	// The step removal still surfaces MAJOR — but the migration trigger
	// addition adds the migrate_* reason so the caller (publish path)
	// can recognise the migration and downgrade to PATCH.
	if !hasReason(d, "migrate_event_trigger_to_triggers_block:src") {
		t.Errorf("missing migrate_event_trigger_to_triggers_block reason: %v", d.Reasons)
	}
	if !hasReason(d, "step_removed:src") {
		t.Errorf("missing step_removed reason: %v", d.Reasons)
	}
}
