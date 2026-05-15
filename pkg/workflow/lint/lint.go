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
)

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
	return r
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
		case ast.StepSkill, ast.StepPrompt, ast.StepSubWorkflow:
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
