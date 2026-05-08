// Package schema exposes the canonical JSON Schema for the Forge workflow DSL
// and a lightweight validator that does not depend on a full JSON Schema
// implementation.
//
// The full JSON Schema document is also exposed so the Portal editor and any
// external tooling can reuse it.
package schema

import (
	_ "embed"
	"errors"
	"fmt"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

//go:embed workflow.schema.json
var workflowSchema []byte

// JSONSchema returns the canonical JSON Schema describing a Workflow document.
func JSONSchema() []byte {
	out := make([]byte, len(workflowSchema))
	copy(out, workflowSchema)
	return out
}

var (
	ErrMissingAPIVersion  = errors.New("missing_api_version")
	ErrMissingKind        = errors.New("missing_kind")
	ErrMissingMetadata    = errors.New("missing_metadata")
	ErrMissingMetadataID  = errors.New("missing_metadata_id")
	ErrMissingMetadataName = errors.New("missing_metadata_name")
	ErrMissingMetadataVersion = errors.New("missing_metadata_version")
	ErrMissingSpec        = errors.New("missing_spec")
	ErrMissingSteps       = errors.New("missing_steps")
	ErrInvalidStepType    = errors.New("invalid_step_type")
	ErrMissingStepID      = errors.New("missing_step_id")
	ErrUnknownVisibility  = errors.New("unknown_visibility")
	ErrUnknownCriticality = errors.New("unknown_criticality")
)

// Validate performs structural validation against the canonical schema.
//
// This catches the same surface area as the embedded JSON Schema while
// avoiding a heavyweight JSON Schema runtime dependency. The schema document
// is still exposed for external tooling such as the Portal editor.
func Validate(wf *ast.Workflow) error {
	if wf == nil {
		return ErrMissingSpec
	}
	if wf.APIVersion == "" {
		return ErrMissingAPIVersion
	}
	if wf.Kind == "" {
		return ErrMissingKind
	}
	if wf.Kind != ast.Kind {
		return fmt.Errorf("%w: %q", ErrMissingKind, wf.Kind)
	}
	if wf.APIVersion != ast.APIVersion {
		return fmt.Errorf("%w: %q", ErrMissingAPIVersion, wf.APIVersion)
	}
	if wf.Metadata.ID == "" {
		return ErrMissingMetadataID
	}
	if wf.Metadata.Name == "" {
		return ErrMissingMetadataName
	}
	if wf.Metadata.Version == "" {
		return ErrMissingMetadataVersion
	}
	if wf.Metadata.Visibility != "" {
		switch wf.Metadata.Visibility {
		case ast.VisibilityPrivate, ast.VisibilityWorkspace, ast.VisibilityTenant, ast.VisibilityForgeCertified:
		default:
			return fmt.Errorf("%w: %q", ErrUnknownVisibility, wf.Metadata.Visibility)
		}
	}
	if wf.Metadata.Criticality != "" {
		switch wf.Metadata.Criticality {
		case ast.CriticalityLow, ast.CriticalityMedium, ast.CriticalityHigh, ast.CriticalityCritical:
		default:
			return fmt.Errorf("%w: %q", ErrUnknownCriticality, wf.Metadata.Criticality)
		}
	}
	if len(wf.Spec.Steps) == 0 {
		return ErrMissingSteps
	}
	for i, step := range wf.Spec.Steps {
		if step.ID == "" {
			return fmt.Errorf("%w: step[%d]", ErrMissingStepID, i)
		}
		if !ast.IsKnownStepType(step.Type) {
			return fmt.Errorf("%w: %q", ErrInvalidStepType, step.Type)
		}
	}
	return nil
}
