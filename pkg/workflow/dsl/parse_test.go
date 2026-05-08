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
