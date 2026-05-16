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
//
// As part of the catalog reconciliation (ai-flow-authoring change), Parse
// applies in-memory migrations for deprecated step kinds so the rest of the
// pipeline (schema validation, lint, runtime) only ever sees the canonical
// shape. Migrations performed:
//
//   - StepPrompt -> StepPromptTemplate  (aliased on read)
//   - StepEventTrigger -> entry in spec.Triggers (TODO: lands with the
//     triggers-block migration in tasks 2.1-2.4)
//
// The lint package emits a deprecated_step_kind warning for any step that
// was migrated so authors see the deprecation at publish time.
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
	migrateDeprecatedStepKinds(wf)
	return wf, nil
}

// migrateDeprecatedStepKinds rewrites in-place any step whose type was
// deprecated in the catalog reconciliation. The original type is preserved
// in step.MigratedFrom (or trigger.MigratedFrom) so the lint layer can
// emit a deprecated_step_kind warning citing the original shape.
//
// Two migrations are performed:
//   - StepPrompt -> StepPromptTemplate (aliased in-place)
//   - StepEventTrigger -> entry in Spec.Triggers (moved out of Spec.Steps)
//
// Nested steps (branches, loop bodies, on_failure) are traversed for the
// prompt -> prompt-template alias but NOT for the event-trigger move:
// triggers are a top-level concept, and a nested event-trigger step would
// be semantically broken anyway.
func migrateDeprecatedStepKinds(wf *ast.Workflow) {
	// Pass 1: extract top-level event-trigger steps into Spec.Triggers.
	kept := wf.Spec.Steps[:0]
	for _, s := range wf.Spec.Steps {
		if s.Type == ast.StepEventTrigger {
			wf.Spec.Triggers = append(wf.Spec.Triggers, eventTriggerToTrigger(s))
			continue
		}
		kept = append(kept, s)
	}
	wf.Spec.Steps = kept

	// Pass 2: alias prompt -> prompt-template recursively.
	for i := range wf.Spec.Steps {
		migrateStep(&wf.Spec.Steps[i])
	}
	for i := range wf.Spec.OnFailure {
		migrateStep(&wf.Spec.OnFailure[i])
	}
}

// eventTriggerToTrigger converts a legacy `event-trigger` step into a
// Trigger entry. The shape of EventPattern maps cleanly onto either a
// webhook-in (when Source looks like an HTTP/URL source) or an event-bus
// trigger (otherwise).
func eventTriggerToTrigger(s ast.Step) ast.Trigger {
	t := ast.Trigger{
		ID:           s.ID,
		MigratedFrom: ast.StepEventTrigger,
		Config:       map[string]any{},
	}
	if s.EventPattern == nil {
		// Defensive: no pattern at all. Treat as a manual trigger so the
		// flow remains parseable; lint surfaces the migration warning.
		t.Type = ast.TriggerManual
		return t
	}
	src := s.EventPattern.Source
	if isHTTPLikeSource(src) {
		t.Type = ast.TriggerWebhookIn
		if s.EventPattern.Type != "" {
			t.Config["event_type"] = s.EventPattern.Type
		}
		if src != "" {
			t.Config["source"] = src
		}
	} else {
		t.Type = ast.TriggerEventBus
		if s.EventPattern.Type != "" {
			t.Config["topic"] = s.EventPattern.Type
		}
		if src != "" {
			t.Config["source"] = src
		}
	}
	if len(s.EventPattern.Filter) > 0 {
		t.Config["filter"] = s.EventPattern.Filter
	}
	return t
}

func isHTTPLikeSource(s string) bool {
	if s == "" {
		return false
	}
	switch {
	case len(s) >= 7 && s[:7] == "http://":
		return true
	case len(s) >= 8 && s[:8] == "https://":
		return true
	case len(s) >= 8 && s[:8] == "webhook:":
		return true
	}
	return false
}

func migrateStep(s *ast.Step) {
	if replacement, ok := ast.DeprecatedStepTypes()[s.Type]; ok && replacement != "" {
		s.MigratedFrom = s.Type
		s.Type = replacement
	}
	// branches and loop bodies recurse.
	for bi := range s.Branches {
		for si := range s.Branches[bi].Steps {
			migrateStep(&s.Branches[bi].Steps[si])
		}
	}
	for i := range s.Body {
		migrateStep(&s.Body[i])
	}
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
