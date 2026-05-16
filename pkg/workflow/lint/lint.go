// Package lint inspects a parsed workflow AST for structural issues.
//
// Issues reported include:
//   - dangling_dep: a step references a depends_on id that doesn't exist
//   - unreachable_step: a step is never reachable from the entry set
//   - cycle_detected: depends_on graph contains a cycle
//   - type_mismatch: a step input cannot be wired from the producer's type
//   - floating_reference_not_allowed: registry reference uses a non-pinned tag
//   - unknown_asset_format: registry reference cannot be parsed
package lint

import (
	"errors"
	"fmt"
	"strings"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// Severity of a lint finding.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Code identifies a lint rule.
type Code string

const (
	CodeDanglingDep        Code = "dangling_dep"
	CodeUnreachableStep    Code = "unreachable_step"
	CodeCycleDetected      Code = "cycle_detected"
	CodeTypeMismatch       Code = "type_mismatch"
	CodeFloatingRef        Code = "floating_reference_not_allowed"
	CodeUnknownAssetFormat Code = "unknown_asset_format"
	CodeDuplicateStepID    Code = "duplicate_step_id"
	CodeMissingRef         Code = "missing_ref"
	CodeMissingTool        Code = "missing_tool"
	CodeMissingApprover    Code = "missing_approver_role"
	CodeInvalidTargetPhase Code = "invalid_target_phase"
	CodeInvalidTargetValue Code = "invalid_target_value"
	// CodeDeprecatedStepKind is emitted (severity: warning) when the DSL
	// parser applied a deprecated-shape migration. It does not block
	// publish; the registry persists the migrated form on next save.
	CodeDeprecatedStepKind Code = "deprecated_step_kind"

	// New rules introduced by the ai-flow-authoring change.
	CodeUnknownTriggerType  Code = "unknown_trigger_type"
	CodeDanglingTriggerRef  Code = "dangling_trigger_field"
	CodeUnknownEventTopic   Code = "unknown_event_topic"
	CodeToolOutsidePinnedSet Code = "tool_outside_pinned_set"
	CodeDanglingStepField   Code = "dangling_step_field"
	CodeMissingPromptTpl    Code = "missing_prompt_template"
	CodeMissingModelRef     Code = "missing_model_ref"
)

// KnownEventTopics is consulted by checkTriggers when validating event-bus
// trigger subscriptions. This stub registry covers the events emitted by
// in-repo services today; a real implementation will pull from the platform
// event catalog. Extending it requires an OpenSpec change.
var KnownEventTopics = map[string]struct{}{
	"workflow.published.v1":           {},
	"workflow.started.v1":             {},
	"workflow.completed.v1":           {},
	"workflow.failed.v1":              {},
	"workflow.step.started.v1":        {},
	"workflow.step.completed.v1":      {},
	"workflow.trigger.fired.v1":       {},
	"workflow.trigger.dropped.v1":     {},
	"workflow.trigger.failed.v1":      {},
	"workflow.llm.budget_exhausted.v1": {},
	"deployment.completed.v1":         {},
	"deployment.failed.v1":            {},
	"incident.opened.v1":              {},
	"incident.resolved.v1":            {},
	"symptom.detected.v1":             {},
	"github.push.v1":                  {},
	"github.pull_request.opened.v1":   {},
	"github.pull_request.merged.v1":   {},
}

// knownTargetPhases mirrors the canonical set from the application service.
var knownTargetPhases = map[string]struct{}{
	"architect": {}, "design": {}, "development": {}, "qa": {},
	"security": {}, "devops": {}, "iac": {}, "sre": {}, "finops": {}, "observability": {},
}

// knownTargetValues mirrors the allowed values from the application service.
var knownTargetValues = map[string]struct{}{
	"required": {}, "optional": {}, "opt-in": {}, "skipped": {},
}

// Finding is a single lint issue.
type Finding struct {
	Code     Code     `json:"code"`
	Severity Severity `json:"severity"`
	StepID   string   `json:"step_id,omitempty"`
	Message  string   `json:"message"`
}

// Errors-only filter helper.
func (f Finding) IsError() bool { return f.Severity == SeverityError }

// Result is the full set of findings.
type Result struct {
	Findings []Finding `json:"findings"`
}

// Errors returns the subset of findings with severity=error.
func (r Result) Errors() []Finding {
	out := []Finding{}
	for _, f := range r.Findings {
		if f.IsError() {
			out = append(out, f)
		}
	}
	return out
}

// Lint runs all checks on a workflow.
func Lint(wf *ast.Workflow) Result {
	r := Result{Findings: []Finding{}}
	if wf == nil {
		return r
	}
	r.add(checkDuplicateIDs(wf)...)
	r.add(checkDanglingDeps(wf)...)
	r.add(checkCycles(wf)...)
	r.add(checkUnreachable(wf)...)
	r.add(checkRefs(wf)...)
	r.add(checkStepShape(wf)...)
	r.add(checkTypeWiring(wf)...)
	r.add(checkTargets(wf)...)
	r.add(checkDeprecatedStepKinds(wf)...)
	r.add(checkTriggers(wf)...)
	r.add(checkLLMSteps(wf)...)
	return r
}

// checkTriggers validates the spec.triggers block: unknown types, unknown
// event topics, dangling $triggers.<id>.<field> references from steps.
func checkTriggers(wf *ast.Workflow) []Finding {
	out := []Finding{}
	declared := map[string]map[string]struct{}{}
	for _, t := range wf.Spec.Triggers {
		if !ast.IsKnownTriggerType(t.Type) {
			out = append(out, Finding{
				Code:     CodeUnknownTriggerType,
				Severity: SeverityError,
				StepID:   t.ID,
				Message:  fmt.Sprintf("trigger %q has unknown type %q", t.ID, t.Type),
			})
		}
		if t.Type == ast.TriggerEventBus {
			topic, _ := t.Config["topic"].(string)
			if topic != "" {
				if _, ok := KnownEventTopics[topic]; !ok {
					out = append(out, Finding{
						Code:     CodeUnknownEventTopic,
						Severity: SeverityError,
						StepID:   t.ID,
						Message:  fmt.Sprintf("trigger %q subscribes to unknown event topic %q", t.ID, topic),
					})
				}
			}
		}
		fieldSet := map[string]struct{}{}
		for name := range t.Outputs {
			fieldSet[name] = struct{}{}
		}
		declared[t.ID] = fieldSet
	}

	// Scan $triggers.<id>.<field> in step inputs and assert references resolve.
	scan := func(stepID string, v any) {
		ref, ok := v.(string)
		if !ok || !strings.HasPrefix(ref, "$triggers.") {
			return
		}
		parts := strings.SplitN(strings.TrimPrefix(ref, "$triggers."), ".", 2)
		if len(parts) < 2 {
			return
		}
		triggerID, field := parts[0], parts[1]
		fields, hasTrigger := declared[triggerID]
		if !hasTrigger {
			out = append(out, Finding{
				Code:     CodeDanglingTriggerRef,
				Severity: SeverityError,
				StepID:   stepID,
				Message:  fmt.Sprintf("step %q references unknown trigger id %q", stepID, triggerID),
			})
			return
		}
		// If the trigger declared no outputs schema, allow any field (open).
		if len(fields) == 0 {
			return
		}
		if _, ok := fields[field]; !ok {
			out = append(out, Finding{
				Code:     CodeDanglingTriggerRef,
				Severity: SeverityError,
				StepID:   stepID,
				Message:  fmt.Sprintf("step %q references undeclared field %q on trigger %q", stepID, field, triggerID),
			})
		}
	}
	for _, s := range allSteps(wf) {
		for _, v := range s.Inputs {
			scan(s.ID, v)
		}
	}
	return out
}

// checkLLMSteps validates type-specific shape for `llm` steps: required
// prompt_template + model, no floating prompt-template refs, tools subset of
// pinned MCPs (when pinned), downstream references to declared outputs only.
func checkLLMSteps(wf *ast.Workflow) []Finding {
	out := []Finding{}
	llmOutputs := map[string]map[string]struct{}{}
	llmSteps := []ast.Step{}
	for _, s := range allSteps(wf) {
		if s.Type != ast.StepLLM {
			continue
		}
		llmSteps = append(llmSteps, s)
		// Declared output schema.
		fields := map[string]struct{}{}
		for name := range s.StepOutputs {
			fields[name] = struct{}{}
		}
		llmOutputs[s.ID] = fields
	}
	for _, s := range llmSteps {
		if s.PromptTemplate == "" {
			out = append(out, Finding{
				Code: CodeMissingPromptTpl, Severity: SeverityError, StepID: s.ID,
				Message: fmt.Sprintf("step %q (llm) requires prompt_template", s.ID),
			})
		} else {
			_, err := ast.ParseAssetRef(s.PromptTemplate)
			if err != nil && errors.Is(err, ast.ErrFloatingReference) {
				out = append(out, Finding{
					Code: CodeFloatingRef, Severity: SeverityError, StepID: s.ID,
					Message: fmt.Sprintf("step %q prompt_template %q must be pinned to exact SemVer", s.ID, s.PromptTemplate),
				})
			}
		}
		if s.Model == nil || s.Model.Ref == "" {
			out = append(out, Finding{
				Code: CodeMissingModelRef, Severity: SeverityError, StepID: s.ID,
				Message: fmt.Sprintf("step %q (llm) requires model.ref", s.ID),
			})
		}
		// Tool refs each go through floating-ref check.
		for _, tool := range s.Tools {
			_, err := ast.ParseAssetRef(tool)
			if err != nil && errors.Is(err, ast.ErrFloatingReference) {
				out = append(out, Finding{
					Code: CodeFloatingRef, Severity: SeverityError, StepID: s.ID,
					Message: fmt.Sprintf("step %q tool %q must be pinned to exact SemVer", s.ID, tool),
				})
			}
		}
	}

	// Downstream references to $steps.<llm-step-id>.<field> against declared outputs.
	scan := func(stepID string, v any) {
		ref, ok := v.(string)
		if !ok || !strings.HasPrefix(ref, "$steps.") {
			return
		}
		// $steps.<id>.<field> OR $steps.<id>.outputs.<field>
		trimmed := strings.TrimPrefix(ref, "$steps.")
		parts := strings.Split(trimmed, ".")
		if len(parts) < 2 {
			return
		}
		llmID := parts[0]
		fields, isLLM := llmOutputs[llmID]
		if !isLLM {
			return
		}
		// Skip when target has no declared schema (open).
		if len(fields) == 0 {
			return
		}
		field := parts[1]
		if field == "outputs" && len(parts) >= 3 {
			field = parts[2]
		}
		if _, ok := fields[field]; !ok {
			out = append(out, Finding{
				Code: CodeDanglingStepField, Severity: SeverityError, StepID: stepID,
				Message: fmt.Sprintf("step %q references undeclared output %q on llm step %q", stepID, field, llmID),
			})
		}
	}
	for _, s := range allSteps(wf) {
		for _, v := range s.Inputs {
			scan(s.ID, v)
		}
	}
	return out
}

// checkDeprecatedStepKinds emits a warning for any step or trigger whose
// original type was deprecated and migrated in-memory by dsl.Parse. The
// MigratedFrom field is the breadcrumb left by the parser.
func checkDeprecatedStepKinds(wf *ast.Workflow) []Finding {
	out := []Finding{}
	for _, s := range allSteps(wf) {
		if s.MigratedFrom == "" {
			continue
		}
		replacement := ast.DeprecatedStepTypes()[s.MigratedFrom]
		msg := fmt.Sprintf("step %q uses deprecated type %q", s.ID, s.MigratedFrom)
		if replacement != "" {
			msg += fmt.Sprintf("; use %q instead. Auto-migrated on save (PATCH bump)", replacement)
		} else {
			msg += "; this shape will be removed in a follow-up change"
		}
		out = append(out, Finding{
			Code:     CodeDeprecatedStepKind,
			Severity: SeverityWarning,
			StepID:   s.ID,
			Message:  msg,
		})
	}
	for _, t := range wf.Spec.Triggers {
		if t.MigratedFrom == "" {
			continue
		}
		out = append(out, Finding{
			Code:     CodeDeprecatedStepKind,
			Severity: SeverityWarning,
			StepID:   t.ID,
			Message: fmt.Sprintf(
				"trigger %q was migrated from a legacy %q step; the registry will persist the new spec.triggers shape on next save (PATCH bump)",
				t.ID, t.MigratedFrom,
			),
		})
	}
	return out
}

// checkTargets validates that any step-level `targets:` map uses only known
// phase keys and allowed values.
func checkTargets(wf *ast.Workflow) []Finding {
	out := []Finding{}
	for _, s := range allSteps(wf) {
		for phase, val := range s.Targets {
			if _, ok := knownTargetPhases[phase]; !ok {
				out = append(out, Finding{
					Code:     CodeInvalidTargetPhase,
					Severity: SeverityError,
					StepID:   s.ID,
					Message:  fmt.Sprintf("step %q targets: unknown phase %q; allowed: architect design development qa security devops iac sre finops observability", s.ID, phase),
				})
			}
			if _, ok := knownTargetValues[val]; !ok {
				out = append(out, Finding{
					Code:     CodeInvalidTargetValue,
					Severity: SeverityError,
					StepID:   s.ID,
					Message:  fmt.Sprintf("step %q targets.%s: unknown value %q; allowed: required optional opt-in skipped", s.ID, phase, val),
				})
			}
		}
	}
	return out
}

func (r *Result) add(f ...Finding) {
	r.Findings = append(r.Findings, f...)
}

// allSteps walks the AST including bodies of branches/loops/on_failure.
func allSteps(wf *ast.Workflow) []ast.Step {
	out := make([]ast.Step, 0, len(wf.Spec.Steps)+len(wf.Spec.OnFailure))
	out = append(out, wf.Spec.Steps...)
	for _, s := range wf.Spec.Steps {
		out = append(out, walkBranches(s)...)
		out = append(out, s.Body...)
	}
	out = append(out, wf.Spec.OnFailure...)
	return out
}

func walkBranches(s ast.Step) []ast.Step {
	out := []ast.Step{}
	for _, b := range s.Branches {
		out = append(out, b.Steps...)
	}
	return out
}

func checkDuplicateIDs(wf *ast.Workflow) []Finding {
	seen := map[string]int{}
	for _, s := range allSteps(wf) {
		seen[s.ID]++
	}
	out := []Finding{}
	for id, n := range seen {
		if n > 1 {
			out = append(out, Finding{
				Code:     CodeDuplicateStepID,
				Severity: SeverityError,
				StepID:   id,
				Message:  fmt.Sprintf("step id %q appears %d times", id, n),
			})
		}
	}
	return out
}

func checkDanglingDeps(wf *ast.Workflow) []Finding {
	have := map[string]struct{}{}
	for _, s := range allSteps(wf) {
		have[s.ID] = struct{}{}
	}
	out := []Finding{}
	for _, s := range allSteps(wf) {
		for _, d := range s.DependsOn {
			if _, ok := have[d]; !ok {
				out = append(out, Finding{
					Code:     CodeDanglingDep,
					Severity: SeverityError,
					StepID:   s.ID,
					Message:  fmt.Sprintf("step %q depends on unknown step %q", s.ID, d),
				})
			}
		}
	}
	return out
}

func checkCycles(wf *ast.Workflow) []Finding {
	graph := map[string][]string{}
	for _, s := range allSteps(wf) {
		graph[s.ID] = append([]string(nil), s.DependsOn...)
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	state := map[string]int{}
	out := []Finding{}
	var dfs func(id string, path []string) bool
	dfs = func(id string, path []string) bool {
		state[id] = gray
		for _, dep := range graph[id] {
			switch state[dep] {
			case gray:
				out = append(out, Finding{
					Code:     CodeCycleDetected,
					Severity: SeverityError,
					StepID:   id,
					Message:  fmt.Sprintf("cycle detected: %s -> %s", strings.Join(append(path, id), " -> "), dep),
				})
				return true
			case white:
				if dfs(dep, append(path, id)) {
					return true
				}
			}
		}
		state[id] = black
		return false
	}
	for id := range graph {
		if state[id] == white {
			if dfs(id, nil) {
				return out
			}
		}
	}
	return out
}

func checkUnreachable(wf *ast.Workflow) []Finding {
	// Reachable set = transitive closure starting from steps with no
	// inbound dependency, plus event-trigger nodes.
	steps := allSteps(wf)
	indeg := map[string]int{}
	successors := map[string][]string{}
	allIDs := map[string]struct{}{}
	for _, s := range steps {
		allIDs[s.ID] = struct{}{}
	}
	for _, s := range steps {
		for _, d := range s.DependsOn {
			if _, ok := allIDs[d]; ok {
				successors[d] = append(successors[d], s.ID)
				indeg[s.ID]++
			}
		}
	}
	reachable := map[string]bool{}
	queue := []string{}
	for _, s := range steps {
		if indeg[s.ID] == 0 || s.Type == ast.StepEventTrigger {
			reachable[s.ID] = true
			queue = append(queue, s.ID)
		}
	}
	for len(queue) > 0 {
		head := queue[0]
		queue = queue[1:]
		for _, succ := range successors[head] {
			if !reachable[succ] {
				reachable[succ] = true
				queue = append(queue, succ)
			}
		}
	}
	out := []Finding{}
	for id := range allIDs {
		if !reachable[id] {
			out = append(out, Finding{
				Code:     CodeUnreachableStep,
				Severity: SeverityError,
				StepID:   id,
				Message:  fmt.Sprintf("step %q is unreachable", id),
			})
		}
	}
	return out
}

func checkRefs(wf *ast.Workflow) []Finding {
	out := []Finding{}
	for _, s := range allSteps(wf) {
		if s.Ref == "" {
			continue
		}
		_, err := ast.ParseAssetRef(s.Ref)
		if err == nil {
			continue
		}
		switch {
		case errors.Is(err, ast.ErrFloatingReference):
			out = append(out, Finding{
				Code:     CodeFloatingRef,
				Severity: SeverityError,
				StepID:   s.ID,
				Message:  fmt.Sprintf("step %q ref %q must be pinned to exact SemVer", s.ID, s.Ref),
			})
		default:
			out = append(out, Finding{
				Code:     CodeUnknownAssetFormat,
				Severity: SeverityError,
				StepID:   s.ID,
				Message:  fmt.Sprintf("step %q ref %q is not a valid registry reference", s.ID, s.Ref),
			})
		}
	}
	return out
}

func checkStepShape(wf *ast.Workflow) []Finding {
	out := []Finding{}
	for _, s := range allSteps(wf) {
		switch s.Type {
		case ast.StepSkill, ast.StepPromptTemplate, ast.StepPrompt, ast.StepSubWorkflow:
			if s.Ref == "" && s.WorkflowRef == "" {
				out = append(out, Finding{Code: CodeMissingRef, Severity: SeverityError, StepID: s.ID,
					Message: fmt.Sprintf("step %q (%s) requires ref", s.ID, s.Type)})
			}
		case ast.StepMCP:
			if s.Ref == "" {
				out = append(out, Finding{Code: CodeMissingRef, Severity: SeverityError, StepID: s.ID,
					Message: fmt.Sprintf("step %q (mcp) requires ref", s.ID)})
			}
			if s.Tool == "" {
				out = append(out, Finding{Code: CodeMissingTool, Severity: SeverityError, StepID: s.ID,
					Message: fmt.Sprintf("step %q (mcp) requires tool", s.ID)})
			}
		case ast.StepHumanInLoop:
			if s.ApproverRole == "" {
				out = append(out, Finding{Code: CodeMissingApprover, Severity: SeverityError, StepID: s.ID,
					Message: fmt.Sprintf("step %q (human-in-the-loop) requires approver_role", s.ID)})
			}
		}
	}
	return out
}

// checkTypeWiring is intentionally lightweight: it verifies that placeholder
// references like `$inputs.foo` or `$steps.x.outputs.y` resolve to declared
// fields. A richer type system can be layered on later.
func checkTypeWiring(wf *ast.Workflow) []Finding {
	out := []Finding{}
	knownInputs := map[string]struct{}{}
	for _, in := range wf.Spec.Inputs {
		knownInputs[in.Name] = struct{}{}
	}
	stepOutputs := map[string]map[string]struct{}{}
	for _, s := range allSteps(wf) {
		out := map[string]struct{}{}
		for _, name := range s.Outputs {
			out[name] = struct{}{}
		}
		stepOutputs[s.ID] = out
	}
	scanRefs := func(stepID string, in any) {
		ref := stringValue(in)
		if ref == "" {
			return
		}
		if !strings.HasPrefix(ref, "$") {
			return
		}
		parts := strings.Split(strings.TrimPrefix(ref, "$"), ".")
		if len(parts) < 2 {
			return
		}
		switch parts[0] {
		case "inputs":
			if _, ok := knownInputs[parts[1]]; !ok {
				out = append(out, Finding{
					Code: CodeTypeMismatch, Severity: SeverityError, StepID: stepID,
					Message: fmt.Sprintf("step %q references unknown input %q", stepID, parts[1]),
				})
			}
		case "steps":
			if len(parts) < 4 || parts[2] != "outputs" {
				return
			}
			producer, ok := stepOutputs[parts[1]]
			if !ok {
				out = append(out, Finding{
					Code: CodeTypeMismatch, Severity: SeverityError, StepID: stepID,
					Message: fmt.Sprintf("step %q references unknown step %q", stepID, parts[1]),
				})
				return
			}
			if len(producer) == 0 {
				return
			}
			if _, ok := producer[parts[3]]; !ok {
				out = append(out, Finding{
					Code: CodeTypeMismatch, Severity: SeverityError, StepID: stepID,
					Message: fmt.Sprintf("step %q references unknown output %q on %q", stepID, parts[3], parts[1]),
				})
			}
		}
	}
	for _, s := range allSteps(wf) {
		for _, v := range s.Inputs {
			scanRefs(s.ID, v)
		}
	}
	return out
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
