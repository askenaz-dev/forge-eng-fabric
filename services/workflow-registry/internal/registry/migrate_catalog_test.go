package registry

import (
	"testing"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

func TestApplyCatalogMigrationsStripsActiveSurfaceEndpoint(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{{
				ID:   "a",
				Type: ast.StepMCP,
				Ref:  "registry:mcp/github@write",
				ActiveSurface: &ast.NodeActiveSurface{
					Family:   "mcp",
					Endpoint: "/v1/gw/mcp/github",
				},
			}},
		},
	}
	reasons := ApplyCatalogMigrations(wf)
	if !containsReason(reasons, ReasonCleanupActiveSurfaceEndpoint) {
		t.Fatalf("expected cleanup reason, got %v", reasons)
	}
	if wf.Spec.Steps[0].ActiveSurface.Endpoint != "" {
		t.Fatalf("endpoint not stripped: %q", wf.Spec.Steps[0].ActiveSurface.Endpoint)
	}
	if wf.Spec.Steps[0].ActiveSurface.Family != "mcp" {
		t.Fatalf("family should be preserved: %q", wf.Spec.Steps[0].ActiveSurface.Family)
	}
}

func TestApplyCatalogMigrationsAliasesPrompt(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{{
				ID:   "render",
				Type: ast.StepPrompt,
				Ref:  "registry:prompt/foo/bar@1.0.0",
			}},
		},
	}
	reasons := ApplyCatalogMigrations(wf)
	if !containsReason(reasons, ReasonMigratePromptToPromptTemplate) {
		t.Fatalf("expected migrate_prompt_to_prompt_template, got %v", reasons)
	}
	if wf.Spec.Steps[0].Type != ast.StepPromptTemplate {
		t.Fatalf("type not aliased: %q", wf.Spec.Steps[0].Type)
	}
}

func TestApplyCatalogMigrationsMovesEventTriggers(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{
				{ID: "src", Type: ast.StepEventTrigger, EventPattern: &ast.EventPattern{Type: "github.push.v1", Source: "github"}},
				{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0"},
			},
		},
	}
	reasons := ApplyCatalogMigrations(wf)
	if !containsReason(reasons, ReasonMigrateEventTriggerToTriggersBlock) {
		t.Fatalf("expected migrate_event_trigger_to_triggers_block, got %v", reasons)
	}
	if len(wf.Spec.Triggers) != 1 {
		t.Fatalf("expected 1 trigger after migration, got %d", len(wf.Spec.Triggers))
	}
	if wf.Spec.Triggers[0].MigratedFrom != ast.StepEventTrigger {
		t.Errorf("migrated_from breadcrumb missing: %q", wf.Spec.Triggers[0].MigratedFrom)
	}
	for _, s := range wf.Spec.Steps {
		if s.Type == ast.StepEventTrigger {
			t.Errorf("event-trigger step %q should have been removed", s.ID)
		}
	}
}

func TestApplyCatalogMigrationsIdempotent(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{{
				ID: "render", Type: ast.StepPrompt, Ref: "registry:prompt/foo/bar@1.0.0",
			}},
		},
	}
	first := ApplyCatalogMigrations(wf)
	if len(first) == 0 {
		t.Fatal("first pass did nothing")
	}
	second := ApplyCatalogMigrations(wf)
	if len(second) != 0 {
		t.Fatalf("second pass should be no-op, got %v", second)
	}
}

func TestApplyCatalogMigrationsNoChangesOnCleanWorkflow(t *testing.T) {
	wf := &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Steps: []ast.Step{{
				ID: "a", Type: ast.StepSkill, Ref: "registry:skill/a/x@1.0.0",
			}},
		},
	}
	reasons := ApplyCatalogMigrations(wf)
	if len(reasons) != 0 {
		t.Fatalf("expected no migrations on clean workflow, got %v", reasons)
	}
}

func containsReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}
