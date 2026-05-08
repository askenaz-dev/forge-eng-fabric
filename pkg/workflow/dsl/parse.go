// Package dsl parses and serialises the canonical Forge workflow YAML DSL.
package dsl

import (
	"errors"
	"fmt"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
	"gopkg.in/yaml.v3"
)

var (
	ErrEmptyDocument    = errors.New("empty_document")
	ErrUnsupportedKind  = errors.New("unsupported_kind")
	ErrUnsupportedAPIVersion = errors.New("unsupported_api_version")
)

// Parse decodes a YAML document into the canonical AST.
func Parse(raw []byte) (*ast.Workflow, error) {
	if len(raw) == 0 {
		return nil, ErrEmptyDocument
	}
	wf := &ast.Workflow{}
	if err := yaml.Unmarshal(raw, wf); err != nil {
		return nil, fmt.Errorf("yaml_decode: %w", err)
	}
	if wf.APIVersion == "" {
		wf.APIVersion = ast.APIVersion
	}
	if wf.Kind == "" {
		wf.Kind = ast.Kind
	}
	if wf.APIVersion != ast.APIVersion {
		return nil, fmt.Errorf("%w: got %q, want %q", ErrUnsupportedAPIVersion, wf.APIVersion, ast.APIVersion)
	}
	if wf.Kind != ast.Kind {
		return nil, fmt.Errorf("%w: got %q, want %q", ErrUnsupportedKind, wf.Kind, ast.Kind)
	}
	return wf, nil
}

// Marshal serialises a workflow back to YAML, ensuring metadata defaults.
func Marshal(wf *ast.Workflow) ([]byte, error) {
	if wf == nil {
		return nil, ErrEmptyDocument
	}
	if wf.APIVersion == "" {
		wf.APIVersion = ast.APIVersion
	}
	if wf.Kind == "" {
		wf.Kind = ast.Kind
	}
	return yaml.Marshal(wf)
}
