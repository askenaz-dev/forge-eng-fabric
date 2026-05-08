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
// Rules (from design D5.4):
//   - Removing or making required an existing input → MAJOR
//   - Removing an output → MAJOR
//   - Changing input/output type → MAJOR
//   - Adding optional input → MINOR
//   - Adding output → MINOR
//   - New step added → MINOR
//   - Step removed → MAJOR (consumers may reference its outputs)
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
			markMajor(&out, fmt.Sprintf("step_type_changed:%s:%s->%s", id, p.Type, n.Type))
		}
		if p.Ref != n.Ref || p.Tool != n.Tool {
			markMinor(&out, fmt.Sprintf("step_ref_changed:%s", id))
		}
		if !reflect.DeepEqual(p.DependsOn, n.DependsOn) {
			markMinor(&out, fmt.Sprintf("step_deps_changed:%s", id))
		}
	}
	if out.Bump == BumpPatch && metadataChanged(prev, next) {
		out.Reasons = append(out.Reasons, "metadata_only_change")
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
