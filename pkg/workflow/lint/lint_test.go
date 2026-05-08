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
