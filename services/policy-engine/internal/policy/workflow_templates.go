package policy

import (
	"bytes"
	_ "embed"
	"io"
)

// WorkflowPoliciesYAML is the canonical policy template bundle for Phase 5
// workflow publish/install flows. It can be loaded directly via
// LoadWorkflowTemplates() or merged with project-specific policies.
//
// The canonical source lives at services/policy-engine/templates/workflow-policies.yaml;
// the copy here is what is embedded at build time.
//
//go:embed workflow-policies.yaml
var WorkflowPoliciesYAML []byte

// LoadWorkflowTemplates returns an Engine pre-populated with the Phase 5
// workflow policy templates.
func LoadWorkflowTemplates() (*Engine, error) {
	return LoadYAML(io.Reader(bytes.NewReader(WorkflowPoliciesYAML)))
}
