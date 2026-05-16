package registry

// One-time data migration triggered by the ai-flow-authoring change.
//
// Reasons surfaced as the version bump's `reasons` field when the registry
// persists a migrated form:
//
//   - cleanup_active_surface_endpoint: strips the design-time pinned gateway
//     endpoint from saved Step.ActiveSurface. The runtime now always resolves
//     through mcp-gateway / a2a-gateway at dispatch.
//   - migrate_prompt_to_prompt_template: the step type was deprecated and the
//     DSL parser auto-aliases it; this migration materialises the new shape.
//   - migrate_event_trigger_to_triggers_block: the legacy event-trigger step
//     kind was moved into spec.triggers. EventPattern fields map onto
//     trigger.config.{topic,event_type,source,filter}.
//
// Idempotent — running ApplyCatalogMigrations twice yields the same output.
// Returns the list of reasons that were applicable to this workflow, in
// stable order. An empty slice means no migration was needed.

import (
	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

const (
	ReasonCleanupActiveSurfaceEndpoint = "cleanup_active_surface_endpoint"
	ReasonMigratePromptToPromptTemplate = "migrate_prompt_to_prompt_template"
	ReasonMigrateEventTriggerToTriggersBlock = "migrate_event_trigger_to_triggers_block"
)

// ApplyCatalogMigrations rewrites wf in-place to the canonical post-
// ai-flow-authoring shape. Returns the reasons applicable so the caller
// can record them on the version bump.
func ApplyCatalogMigrations(wf *ast.Workflow) []string {
	if wf == nil {
		return nil
	}
	reasons := []string{}
	if stripActiveSurfaceEndpoints(wf) {
		reasons = append(reasons, ReasonCleanupActiveSurfaceEndpoint)
	}
	if aliasPromptToPromptTemplate(wf) {
		reasons = append(reasons, ReasonMigratePromptToPromptTemplate)
	}
	if extractEventTriggers(wf) {
		reasons = append(reasons, ReasonMigrateEventTriggerToTriggersBlock)
	}
	return reasons
}

// stripActiveSurfaceEndpoints removes Step.ActiveSurface.Endpoint from every
// step. Returns true if anything was stripped.
func stripActiveSurfaceEndpoints(wf *ast.Workflow) bool {
	changed := false
	visit := func(s *ast.Step) {
		if s.ActiveSurface != nil && s.ActiveSurface.Endpoint != "" {
			s.ActiveSurface.Endpoint = ""
			changed = true
		}
	}
	walkSteps(wf, visit)
	return changed
}

// aliasPromptToPromptTemplate rewrites any prompt step to prompt-template.
func aliasPromptToPromptTemplate(wf *ast.Workflow) bool {
	changed := false
	walkSteps(wf, func(s *ast.Step) {
		if s.Type == ast.StepPrompt {
			s.Type = ast.StepPromptTemplate
			changed = true
		}
	})
	return changed
}

// extractEventTriggers moves top-level event-trigger steps into spec.Triggers.
// Returns true if any moved. Note: top-level only — a nested event-trigger
// would be semantically broken so we leave it in place.
func extractEventTriggers(wf *ast.Workflow) bool {
	changed := false
	kept := wf.Spec.Steps[:0]
	for _, s := range wf.Spec.Steps {
		if s.Type != ast.StepEventTrigger {
			kept = append(kept, s)
			continue
		}
		wf.Spec.Triggers = append(wf.Spec.Triggers, eventStepToTrigger(s))
		changed = true
	}
	wf.Spec.Steps = kept
	return changed
}

// eventStepToTrigger mirrors dsl.eventTriggerToTrigger but stays in-package
// so the registry can run the migration without depending on dsl's parse
// helpers. Kept in sync by the test suite (see diff_extended_test.go and
// pkg/workflow/dsl tests).
func eventStepToTrigger(s ast.Step) ast.Trigger {
	t := ast.Trigger{
		ID:           s.ID,
		MigratedFrom: ast.StepEventTrigger,
		Config:       map[string]any{},
	}
	if s.EventPattern == nil {
		t.Type = ast.TriggerManual
		return t
	}
	src := s.EventPattern.Source
	if isHTTPLikeSource(src) {
		t.Type = ast.TriggerWebhookIn
		if s.EventPattern.Type != "" {
			t.Config["event_type"] = s.EventPattern.Type
		}
	} else {
		t.Type = ast.TriggerEventBus
		if s.EventPattern.Type != "" {
			t.Config["topic"] = s.EventPattern.Type
		}
	}
	if src != "" {
		t.Config["source"] = src
	}
	if len(s.EventPattern.Filter) > 0 {
		t.Config["filter"] = s.EventPattern.Filter
	}
	return t
}

func isHTTPLikeSource(s string) bool {
	if len(s) >= 7 && s[:7] == "http://" {
		return true
	}
	if len(s) >= 8 && s[:8] == "https://" {
		return true
	}
	if len(s) >= 8 && s[:8] == "webhook:" {
		return true
	}
	return false
}

func walkSteps(wf *ast.Workflow, fn func(*ast.Step)) {
	for i := range wf.Spec.Steps {
		walkStepTree(&wf.Spec.Steps[i], fn)
	}
	for i := range wf.Spec.OnFailure {
		walkStepTree(&wf.Spec.OnFailure[i], fn)
	}
}

func walkStepTree(s *ast.Step, fn func(*ast.Step)) {
	fn(s)
	for bi := range s.Branches {
		for si := range s.Branches[bi].Steps {
			walkStepTree(&s.Branches[bi].Steps[si], fn)
		}
	}
	for i := range s.Body {
		walkStepTree(&s.Body[i], fn)
	}
}
