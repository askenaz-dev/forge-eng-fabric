package dsl

import (
	"reflect"
	"strings"
	"testing"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

const sampleYAML = `apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: refine-and-pr
  name: Refine and Open PR
  version: 1.0.0
  visibility: workspace
  criticality: medium
spec:
  inputs:
    - name: story
      type: string
      required: true
  steps:
    - id: refine
      type: skill
      ref: registry:skill/sdlc-product/refine-user-story@1.2.0
      inputs:
        story: $inputs.story
      retries:
        max: 3
        backoff: exponential
      timeout: 60s
    - id: human-approval
      type: human-in-the-loop
      approver_role: product-owner
      on_timeout: escalate
      escalation_role: engineering-manager
      depends_on:
        - refine
    - id: open-pr
      type: mcp
      ref: registry:mcp/github@write
      tool: create_pr
      depends_on:
        - human-approval
      inputs:
        title: $steps.refine.outputs.refined
  on_failure:
    - id: notify
      type: skill
      ref: registry:skill/sdlc-devops/post-incident-note@1.0.0
`

func TestParseAndMarshalRoundTrip(t *testing.T) {
	wf, err := Parse([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if wf.Metadata.ID != "refine-and-pr" {
		t.Fatalf("metadata id: %q", wf.Metadata.ID)
	}
	out, err := Marshal(wf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "apiVersion: forge.workflows/v1") {
		t.Fatalf("missing apiVersion: %s", out)
	}
	wf2, err := Parse(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !reflect.DeepEqual(wf, wf2) {
		t.Fatalf("round-trip mismatch:\nA=%+v\nB=%+v", wf, wf2)
	}
}

func TestParseRejectsBadAPIVersion(t *testing.T) {
	bad := strings.Replace(sampleYAML, "forge.workflows/v1", "forge.workflows/v999", 1)
	_, err := Parse([]byte(bad))
	if err == nil {
		t.Fatalf("expected error")
	}
}

// TestTargetsFieldRoundTrip verifies that a step-level `targets:` map survives
// a Parse → Marshal → Parse round-trip unchanged (task 8.3 from sdlc-end-to-end).
func TestTargetsFieldRoundTrip(t *testing.T) {
	yaml := `apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: targets-test
  name: Targets Test
  version: 1.0.0
spec:
  steps:
    - id: iac-step
      type: skill
      ref: registry:skill/sdlc-iac/generate-terraform@1.0.0
      targets:
        iac: required
        sre: optional
`
	wf, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(wf.Spec.Steps) == 0 {
		t.Fatal("no steps parsed")
	}
	step := wf.Spec.Steps[0]
	if step.Targets["iac"] != "required" {
		t.Fatalf("targets.iac: got %q want required", step.Targets["iac"])
	}
	if step.Targets["sre"] != "optional" {
		t.Fatalf("targets.sre: got %q want optional", step.Targets["sre"])
	}
	out, err := Marshal(wf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wf2, err := Parse(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !reflect.DeepEqual(wf.Spec.Steps[0].Targets, wf2.Spec.Steps[0].Targets) {
		t.Fatalf("targets round-trip mismatch: %v vs %v", wf.Spec.Steps[0].Targets, wf2.Spec.Steps[0].Targets)
	}
}

// TestPromptStepMigratesToPromptTemplate verifies that the catalog
// reconciliation (ai-flow-authoring change) migrates legacy `prompt` step
// kinds to `prompt-template` on Parse and records the original in
// MigratedFrom for lint to surface.
func TestPromptStepMigratesToPromptTemplate(t *testing.T) {
	yaml := `apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: legacy-prompt
  name: Legacy Prompt
  version: 1.0.0
spec:
  steps:
    - id: classify
      type: prompt
      ref: registry:prompt/sdlc-product/classify@1.0.0
`
	wf, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := wf.Spec.Steps[0].Type; got != ast.StepPromptTemplate {
		t.Fatalf("expected step type migrated to %q, got %q", ast.StepPromptTemplate, got)
	}
	if got := wf.Spec.Steps[0].MigratedFrom; got != ast.StepPrompt {
		t.Fatalf("expected MigratedFrom %q, got %q", ast.StepPrompt, got)
	}
}

// TestNewStepTypesParse verifies the newly enumerated step types from the
// catalog reconciliation are accepted by Parse + the standard sample shape.
func TestNewStepTypesParse(t *testing.T) {
	yaml := `apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: catalog
  name: Catalog
  version: 1.0.0
spec:
  steps:
    - id: think
      type: llm
      ref: registry:prompt/foo/bar@1.0.0
    - id: send
      type: notification-action
      ref: registry:mcp/email@write
      tool: send
    - id: emit
      type: webhook
      ref: registry:mcp/http@write
      tool: post
`
	wf, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wantTypes := []ast.StepType{ast.StepLLM, ast.StepNotificationAction, ast.StepWebhookOut}
	for i, want := range wantTypes {
		if got := wf.Spec.Steps[i].Type; got != want {
			t.Fatalf("step[%d]: got %q want %q", i, got, want)
		}
	}
}

// TestEventTriggerStepMigratesToTriggersBlock verifies that the legacy
// `event-trigger` step kind is moved into spec.Triggers on Parse, with the
// EventPattern fields mapped onto the trigger's Config.
func TestEventTriggerStepMigratesToTriggersBlock(t *testing.T) {
	yaml := `apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: legacy-event
  name: Legacy Event
  version: 1.0.0
spec:
  steps:
    - id: src
      type: event-trigger
      event_pattern:
        type: github.push.v1
        source: github
        filter:
          repo: acme/*
    - id: do
      type: skill
      ref: registry:skill/a/x@1.0.0
      depends_on:
        - src
`
	wf, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(wf.Spec.Triggers) != 1 {
		t.Fatalf("expected one migrated trigger, got %d: %+v", len(wf.Spec.Triggers), wf.Spec.Triggers)
	}
	tr := wf.Spec.Triggers[0]
	if tr.ID != "src" {
		t.Errorf("trigger.ID: got %q want %q", tr.ID, "src")
	}
	if tr.Type != ast.TriggerEventBus {
		t.Errorf("trigger.Type: got %q want %q (non-HTTP source maps to event-bus)", tr.Type, ast.TriggerEventBus)
	}
	if tr.MigratedFrom != ast.StepEventTrigger {
		t.Errorf("trigger.MigratedFrom: got %q want %q", tr.MigratedFrom, ast.StepEventTrigger)
	}
	if topic, _ := tr.Config["topic"].(string); topic != "github.push.v1" {
		t.Errorf("trigger.Config.topic: got %v want github.push.v1", tr.Config["topic"])
	}
	// The event-trigger step should be gone from Spec.Steps.
	for _, s := range wf.Spec.Steps {
		if s.Type == ast.StepEventTrigger {
			t.Errorf("event-trigger step %q should have been removed from spec.steps", s.ID)
		}
	}
}

// TestEventTriggerWithHTTPSourceMapsToWebhookIn verifies the source-based
// routing in eventTriggerToTrigger.
func TestEventTriggerWithHTTPSourceMapsToWebhookIn(t *testing.T) {
	yaml := `apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: legacy-http
  name: Legacy HTTP
  version: 1.0.0
spec:
  steps:
    - id: hook
      type: event-trigger
      event_pattern:
        type: incoming.payload
        source: https://example.com/hook
    - id: do
      type: skill
      ref: registry:skill/a/x@1.0.0
`
	wf, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(wf.Spec.Triggers) != 1 || wf.Spec.Triggers[0].Type != ast.TriggerWebhookIn {
		t.Fatalf("expected webhook-in trigger, got %+v", wf.Spec.Triggers)
	}
}

func TestParseDefaultsAPIVersion(t *testing.T) {
	yaml := `metadata:
  id: x
  name: x
  version: 1.0.0
spec:
  steps:
    - id: a
      type: skill
      ref: registry:skill/x/y@1.0.0
`
	wf, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if wf.APIVersion != ast.APIVersion {
		t.Fatalf("apiVersion default: %q", wf.APIVersion)
	}
	if wf.Kind != ast.Kind {
		t.Fatalf("kind default: %q", wf.Kind)
	}
}
