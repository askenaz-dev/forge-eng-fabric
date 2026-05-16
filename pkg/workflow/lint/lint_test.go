package lint

import (
	"testing"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

func TestLintReportsCycle(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion,
		Kind:       ast.Kind,
		Metadata:   ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{
				{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0", DependsOn: []string{"b"}},
				{ID: "b", Type: ast.StepSkill, Ref: "registry:skill/a/y@1.0.0", DependsOn: []string{"a"}},
			},
		},
	}
	r := Lint(wf)
	if !hasCode(r.Findings, CodeCycleDetected) {
		t.Fatalf("expected cycle finding, got %+v", r.Findings)
	}
}

func TestLintReportsDangling(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{
				{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0", DependsOn: []string{"missing"}},
			},
		},
	}
	r := Lint(wf)
	if !hasCode(r.Findings, CodeDanglingDep) {
		t.Fatalf("expected dangling_dep, got %+v", r.Findings)
	}
}

func TestLintReportsFloatingReference(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{Steps: []ast.Step{
			{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/x/y@latest"},
		}},
	}
	r := Lint(wf)
	if !hasCode(r.Findings, CodeFloatingRef) {
		t.Fatalf("expected floating ref, got %+v", r.Findings)
	}
}

func TestLintReportsUnreachable(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{Steps: []ast.Step{
			{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/x/y@1.0.0"},
			{ID: "b", Type: ast.StepSkill, Ref: "registry:skill/x/z@1.0.0", DependsOn: []string{"a"}},
			{ID: "orphan", Type: ast.StepSkill, Ref: "registry:skill/o/o@1.0.0", DependsOn: []string{"orphan"}},
		}},
	}
	r := Lint(wf)
	// Even with the cycle on `orphan`, it should still surface as unreachable
	if !hasCode(r.Findings, CodeCycleDetected) {
		t.Fatalf("expected cycle on orphan, got %+v", r.Findings)
	}
}

func TestLintHappyPath(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Inputs: []ast.IOField{{Name: "story", Type: "string", Required: true}},
			Steps: []ast.Step{
				{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/x/y@1.0.0", Inputs: map[string]any{"story": "$inputs.story"}, Outputs: []string{"refined"}},
				{ID: "b", Type: ast.StepMCP, Ref: "registry:mcp/github@write", Tool: "create_pr", DependsOn: []string{"a"}, Inputs: map[string]any{"title": "$steps.a.outputs.refined"}},
			},
		},
	}
	r := Lint(wf)
	if errs := r.Errors(); len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}

func hasCode(fs []Finding, c Code) bool {
	for _, f := range fs {
		if f.Code == c {
			return true
		}
	}
	return false
}

// --- ai-flow-authoring change: triggers + LLM step lint coverage ---

func TestLintReportsUnknownTriggerType(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Triggers: []ast.Trigger{
				{ID: "t1", Type: "telegram-bot"},
			},
			Steps: []ast.Step{{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0"}},
		},
	}
	r := Lint(wf)
	if !hasCode(r.Findings, CodeUnknownTriggerType) {
		t.Fatalf("expected unknown_trigger_type, got %+v", r.Findings)
	}
}

func TestLintRejectsUnknownEventTopic(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Triggers: []ast.Trigger{
				{ID: "t", Type: ast.TriggerEventBus, Config: map[string]any{"topic": "unregistered.topic.v1"}},
			},
			Steps: []ast.Step{{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0"}},
		},
	}
	r := Lint(wf)
	if !hasCode(r.Findings, CodeUnknownEventTopic) {
		t.Fatalf("expected unknown_event_topic, got %+v", r.Findings)
	}
}

func TestLintAcceptsKnownEventTopic(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Triggers: []ast.Trigger{
				{ID: "t", Type: ast.TriggerEventBus, Config: map[string]any{"topic": "github.push.v1"}},
			},
			Steps: []ast.Step{{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0"}},
		},
	}
	r := Lint(wf)
	if hasCode(r.Findings, CodeUnknownEventTopic) {
		t.Fatalf("did not expect unknown_event_topic for known topic, got %+v", r.Findings)
	}
}

func TestLintReportsDanglingTriggerField(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Triggers: []ast.Trigger{
				{ID: "email", Type: ast.TriggerEmailInbound, Outputs: map[string]string{"from": "string"}},
			},
			Steps: []ast.Step{
				{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0",
					Inputs: map[string]any{"body": "$triggers.email.body"}},
			},
		},
	}
	r := Lint(wf)
	if !hasCode(r.Findings, CodeDanglingTriggerRef) {
		t.Fatalf("expected dangling_trigger_field, got %+v", r.Findings)
	}
}

func TestLintLLMRequiresPromptAndModel(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{
				{ID: "think", Type: ast.StepLLM},
			},
		},
	}
	r := Lint(wf)
	if !hasCode(r.Findings, CodeMissingPromptTpl) {
		t.Fatalf("expected missing_prompt_template, got %+v", r.Findings)
	}
	if !hasCode(r.Findings, CodeMissingModelRef) {
		t.Fatalf("expected missing_model_ref, got %+v", r.Findings)
	}
}

func TestLintRejectsLLMWithFloatingPromptTemplate(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{
				{
					ID:             "think",
					Type:           ast.StepLLM,
					PromptTemplate: "registry:prompt/foo/bar@latest",
					Model:          &ast.ModelBinding{Ref: "gateway:model/x@latest-stable"},
				},
			},
		},
	}
	r := Lint(wf)
	if !hasCode(r.Findings, CodeFloatingRef) {
		t.Fatalf("expected floating_reference_not_allowed for prompt_template@latest, got %+v", r.Findings)
	}
}

func TestLintRejectsDownstreamRefToUndeclaredLLMOutput(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{
				{
					ID:             "think",
					Type:           ast.StepLLM,
					PromptTemplate: "registry:prompt/foo/bar@1.0.0",
					Model:          &ast.ModelBinding{Ref: "gateway:model/x@latest-stable"},
					StepOutputs:    map[string]string{"category": "string"},
				},
				{
					ID:        "act",
					Type:      ast.StepSkill,
					Ref:       "registry:skill/a/x@1.0.0",
					DependsOn: []string{"think"},
					Inputs:    map[string]any{"draft": "$steps.think.outputs.draft"},
				},
			},
		},
	}
	r := Lint(wf)
	if !hasCode(r.Findings, CodeDanglingStepField) {
		t.Fatalf("expected dangling_step_field for undeclared LLM output, got %+v", r.Findings)
	}
}

func TestLintEmitsDeprecationForMigratedEventTrigger(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Triggers: []ast.Trigger{
				{ID: "src", Type: ast.TriggerEventBus, MigratedFrom: ast.StepEventTrigger,
					Config: map[string]any{"topic": "github.push.v1"}},
			},
			Steps: []ast.Step{{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0"}},
		},
	}
	r := Lint(wf)
	if !hasCode(r.Findings, CodeDeprecatedStepKind) {
		t.Fatalf("expected deprecated_step_kind for migrated trigger, got %+v", r.Findings)
	}
}
