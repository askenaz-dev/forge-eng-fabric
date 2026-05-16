package registry

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// DiffResult describes the change classification between two workflow ASTs.
type DiffResult struct {
	Bump    BumpKind `json:"bump"`
	Reasons []string `json:"reasons"`
	Major   bool     `json:"major"`
	Minor   bool     `json:"minor"`
}

// DiffWorkflows classifies the change between previous and next.
//
// Rules (from design D5.4, extended by ai-flow-authoring):
//   - Removing or making required an existing input → MAJOR
//   - Removing an output → MAJOR
//   - Changing input/output type → MAJOR
//   - Adding optional input → MINOR
//   - Adding output → MINOR
//   - New step added → MINOR
//   - Step removed → MAJOR (consumers may reference its outputs)
//   - New trigger added → MINOR
//   - Trigger removed → MAJOR (external sources may have been pointed at it)
//   - Trigger type changed → MAJOR
//   - LLM step prompt_template / model.ref changed → MINOR
//   - LLM step outputs_schema: field removed → MAJOR; field added → MINOR
//   - Migration-only change (prompt→prompt-template, event-trigger→triggers
//     block) → PATCH with the corresponding migrate_* reason
//   - Retry tweaks / metadata-only changes → PATCH
func DiffWorkflows(prev, next *ast.Workflow) DiffResult {
	out := DiffResult{Bump: BumpPatch, Reasons: []string{}}
	if prev == nil || next == nil {
		out.Bump = BumpMajor
		out.Reasons = append(out.Reasons, "missing_workflow")
		out.Major = true
		return out
	}
	prevInputs := indexIO(prev.Spec.Inputs)
	nextInputs := indexIO(next.Spec.Inputs)
	for name, p := range prevInputs {
		n, ok := nextInputs[name]
		if !ok {
			markMajor(&out, fmt.Sprintf("input_removed:%s", name))
			continue
		}
		if p.Type != n.Type {
			markMajor(&out, fmt.Sprintf("input_type_changed:%s:%s->%s", name, p.Type, n.Type))
		}
		if !p.Required && n.Required {
			markMajor(&out, fmt.Sprintf("input_required_added:%s", name))
		}
	}
	for name := range nextInputs {
		if _, ok := prevInputs[name]; !ok {
			markMinor(&out, fmt.Sprintf("input_added:%s", name))
		}
	}
	prevOutputs := indexIO(prev.Spec.Outputs)
	nextOutputs := indexIO(next.Spec.Outputs)
	for name, p := range prevOutputs {
		n, ok := nextOutputs[name]
		if !ok {
			markMajor(&out, fmt.Sprintf("output_removed:%s", name))
			continue
		}
		if p.Type != n.Type {
			markMajor(&out, fmt.Sprintf("output_type_changed:%s:%s->%s", name, p.Type, n.Type))
		}
	}
	for name := range nextOutputs {
		if _, ok := prevOutputs[name]; !ok {
			markMinor(&out, fmt.Sprintf("output_added:%s", name))
		}
	}
	prevSteps := indexSteps(prev.Spec.Steps)
	nextSteps := indexSteps(next.Spec.Steps)
	for id := range prevSteps {
		if _, ok := nextSteps[id]; !ok {
			markMajor(&out, fmt.Sprintf("step_removed:%s", id))
		}
	}
	for id, n := range nextSteps {
		p, ok := prevSteps[id]
		if !ok {
			markMinor(&out, fmt.Sprintf("step_added:%s", id))
			continue
		}
		if p.Type != n.Type {
			// The prompt → prompt-template alias is migration-only — keep it PATCH.
			if isPromptToTemplateMigration(p.Type, n.Type) {
				markPatchMigration(&out, fmt.Sprintf("migrate_prompt_to_prompt_template:%s", id))
			} else {
				markMajor(&out, fmt.Sprintf("step_type_changed:%s:%s->%s", id, p.Type, n.Type))
			}
		}
		if p.Ref != n.Ref || p.Tool != n.Tool {
			markMinor(&out, fmt.Sprintf("step_ref_changed:%s", id))
		}
		if !reflect.DeepEqual(p.DependsOn, n.DependsOn) {
			markMinor(&out, fmt.Sprintf("step_deps_changed:%s", id))
		}
		// LLM-specific field comparisons. Apply only when both sides are LLM
		// (or one side becomes LLM and the other did not, which is already
		// covered as step_type_changed).
		if p.Type == ast.StepLLM && n.Type == ast.StepLLM {
			diffLLMStep(&out, id, p, n)
		}
	}

	diffTriggers(&out, prev.Spec.Triggers, next.Spec.Triggers)

	if out.Bump == BumpPatch && metadataChanged(prev, next) {
		out.Reasons = append(out.Reasons, "metadata_only_change")
	}
	return out
}

// isPromptToTemplateMigration reports whether the type change is the
// deprecated-prompt-to-prompt-template auto-migration. The DSL layer
// normally applies this on parse, but the registry runs the diff against
// the raw stored AST; an old version may still carry `prompt`.
func isPromptToTemplateMigration(prev, next ast.StepType) bool {
	return prev == ast.StepPrompt && next == ast.StepPromptTemplate
}

// markPatchMigration records a PATCH-level reason for a migration-only
// change. If a MAJOR or MINOR change has already been observed in this
// diff, the bump stays where it is; the reason is added either way.
func markPatchMigration(d *DiffResult, reason string) {
	d.Reasons = appendOnce(d.Reasons, reason)
}

func diffLLMStep(d *DiffResult, id string, p, n ast.Step) {
	if p.PromptTemplate != n.PromptTemplate {
		markMinor(d, fmt.Sprintf("llm_prompt_template_changed:%s", id))
	}
	pModelRef, nModelRef := modelRef(p.Model), modelRef(n.Model)
	if pModelRef != nModelRef {
		markMinor(d, fmt.Sprintf("llm_model_ref_changed:%s", id))
	}
	if !reflect.DeepEqual(p.Tools, n.Tools) {
		markMinor(d, fmt.Sprintf("llm_tools_changed:%s", id))
	}
	// Outputs schema: removed field → MAJOR (consumers downstream may
	// reference it); added field → MINOR.
	for name := range p.StepOutputs {
		if _, ok := n.StepOutputs[name]; !ok {
			markMajor(d, fmt.Sprintf("llm_outputs_removed:%s:%s", id, name))
		}
	}
	for name := range n.StepOutputs {
		if _, ok := p.StepOutputs[name]; !ok {
			markMinor(d, fmt.Sprintf("llm_outputs_added:%s:%s", id, name))
		}
	}
}

func modelRef(m *ast.ModelBinding) string {
	if m == nil {
		return ""
	}
	return m.Ref
}

// diffTriggers classifies trigger-block changes. Migrations from a legacy
// event-trigger step into a triggers entry are not visible at this layer
// (they appear as an event-trigger step removal in steps and a brand-new
// trigger addition with MigratedFrom set) — the caller can detect the
// pair and downgrade to PATCH if needed. For the current API we surface
// the structural diff and let the publish path apply the migrate_*
// downgrade when both sides match exactly.
func diffTriggers(out *DiffResult, prev, next []ast.Trigger) {
	prevIdx := indexTriggers(prev)
	nextIdx := indexTriggers(next)

	// Removals.
	for id := range prevIdx {
		if _, ok := nextIdx[id]; !ok {
			markMajor(out, fmt.Sprintf("trigger_removed:%s", id))
		}
	}

	// Additions and changes.
	for id, n := range nextIdx {
		p, ok := prevIdx[id]
		if !ok {
			if n.MigratedFrom != "" {
				// Auto-migrated from a deprecated step kind — patch-level.
				markPatchMigration(out, fmt.Sprintf("migrate_event_trigger_to_triggers_block:%s", id))
			} else {
				markMinor(out, fmt.Sprintf("trigger_added:%s", id))
			}
			continue
		}
		if p.Type != n.Type {
			markMajor(out, fmt.Sprintf("trigger_type_changed:%s:%s->%s", id, p.Type, n.Type))
		}
		if !reflect.DeepEqual(p.Config, n.Config) {
			markMinor(out, fmt.Sprintf("trigger_config_changed:%s", id))
		}
		// Output schema additions are MINOR; removals are MAJOR.
		for name := range p.Outputs {
			if _, ok := n.Outputs[name]; !ok {
				markMajor(out, fmt.Sprintf("trigger_outputs_removed:%s:%s", id, name))
			}
		}
		for name := range n.Outputs {
			if _, ok := p.Outputs[name]; !ok {
				markMinor(out, fmt.Sprintf("trigger_outputs_added:%s:%s", id, name))
			}
		}
		if p.ConcurrencyOrDefault() != n.ConcurrencyOrDefault() {
			markMinor(out, fmt.Sprintf("trigger_concurrency_changed:%s", id))
		}
	}
}

func indexTriggers(in []ast.Trigger) map[string]ast.Trigger {
	out := map[string]ast.Trigger{}
	for _, t := range in {
		out[t.ID] = t
	}
	return out
}

func metadataChanged(prev, next *ast.Workflow) bool {
	return prev.Metadata.Description != next.Metadata.Description ||
		prev.Metadata.SuccessMetric != next.Metadata.SuccessMetric ||
		!reflect.DeepEqual(prev.Metadata.Owners, next.Metadata.Owners) ||
		!reflect.DeepEqual(prev.Metadata.Tags, next.Metadata.Tags)
}

func markMajor(d *DiffResult, reason string) {
	d.Bump = BumpMajor
	d.Major = true
	d.Reasons = appendOnce(d.Reasons, reason)
}

func markMinor(d *DiffResult, reason string) {
	if d.Bump != BumpMajor {
		d.Bump = BumpMinor
		d.Minor = true
	}
	d.Reasons = appendOnce(d.Reasons, reason)
}

func appendOnce(in []string, v string) []string {
	for _, x := range in {
		if x == v {
			return in
		}
	}
	return append(in, v)
}

func indexIO(in []ast.IOField) map[string]ast.IOField {
	out := map[string]ast.IOField{}
	for _, f := range in {
		out[f.Name] = f
	}
	return out
}

func indexSteps(in []ast.Step) map[string]ast.Step {
	out := map[string]ast.Step{}
	for _, s := range in {
		out[s.ID] = s
	}
	return out
}

// SortedReasons returns a deterministic order for testing.
func (d DiffResult) SortedReasons() []string {
	cp := append([]string(nil), d.Reasons...)
	sort.Strings(cp)
	return cp
}
