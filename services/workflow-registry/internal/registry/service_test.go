package registry

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

const baseYAML = `apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: wf-1
  name: wf
  version: 1.0.0
spec:
  inputs:
    - name: story
      type: string
      required: true
  steps:
    - id: refine
      type: skill
      ref: registry:skill/x/y@1.0.0
      inputs:
        story: $inputs.story
`

const breakingYAML = `apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: wf-1
  name: wf
  version: 1.1.0
spec:
  steps:
    - id: refine
      type: skill
      ref: registry:skill/x/y@1.0.0
`

func newTestSvc(t *testing.T) *Service {
	t.Helper()
	svc := NewService(&MemorySink{})
	if _, err := svc.CreateWorkflow(context.Background(), CreateWorkflowRequest{
		ID: "wf-1", TenantID: "t1", WorkspaceID: "w1", Name: "wf",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	return svc
}

func TestPublishHappyPath(t *testing.T) {
	svc := newTestSvc(t)
	v, err := svc.PublishVersion(context.Background(), PublishVersionRequest{
		WorkflowID: "wf-1", WorkflowYAML: baseYAML,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if v.Version != "1.0.0" {
		t.Fatalf("version: %s", v.Version)
	}
}

func TestPublishImmutability(t *testing.T) {
	svc := newTestSvc(t)
	if _, err := svc.PublishVersion(context.Background(), PublishVersionRequest{WorkflowID: "wf-1", WorkflowYAML: baseYAML}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	_, err := svc.PublishVersion(context.Background(), PublishVersionRequest{WorkflowID: "wf-1", WorkflowYAML: baseYAML})
	if !errors.Is(err, ErrVersionAlreadyExists) {
		t.Fatalf("expected version_already_exists, got %v", err)
	}
}

func TestPublishBreakingChangeRequiresMajorBump(t *testing.T) {
	svc := newTestSvc(t)
	if _, err := svc.PublishVersion(context.Background(), PublishVersionRequest{WorkflowID: "wf-1", WorkflowYAML: baseYAML}); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := svc.PublishVersion(context.Background(), PublishVersionRequest{WorkflowID: "wf-1", WorkflowYAML: breakingYAML})
	if !errors.Is(err, ErrBreakingChange) {
		t.Fatalf("expected breaking change error, got %v", err)
	}
	if !strings.Contains(err.Error(), "input_removed:story") {
		t.Fatalf("expected input_removed reason in error, got %v", err)
	}
}

func TestPublishAutoBumpClassifiesMajor(t *testing.T) {
	svc := newTestSvc(t)
	if _, err := svc.PublishVersion(context.Background(), PublishVersionRequest{WorkflowID: "wf-1", WorkflowYAML: baseYAML}); err != nil {
		t.Fatalf("first: %v", err)
	}
	v, err := svc.PublishVersion(context.Background(), PublishVersionRequest{WorkflowID: "wf-1", WorkflowYAML: breakingYAML, AutoBump: true})
	if err != nil {
		t.Fatalf("auto bump: %v", err)
	}
	if v.Version != "2.0.0" {
		t.Fatalf("expected 2.0.0 auto-bumped, got %s", v.Version)
	}
	if v.DiffPrev == nil || !v.DiffPrev.Major {
		t.Fatalf("expected major diff, got %+v", v.DiffPrev)
	}
}

func TestDiffOutputAddedIsMinor(t *testing.T) {
	prev := &ast.Workflow{Spec: ast.Spec{Outputs: []ast.IOField{{Name: "a", Type: "string"}}, Steps: []ast.Step{{ID: "s1"}}}}
	next := &ast.Workflow{Spec: ast.Spec{Outputs: []ast.IOField{{Name: "a", Type: "string"}, {Name: "b", Type: "int"}}, Steps: []ast.Step{{ID: "s1"}}}}
	d := DiffWorkflows(prev, next)
	if d.Bump != BumpMinor {
		t.Fatalf("expected minor, got %s reasons=%v", d.Bump, d.Reasons)
	}
}
