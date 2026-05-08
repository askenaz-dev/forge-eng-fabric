package runtime

import (
	"context"
	"fmt"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// ActivityRegistry resolves a step type to its activity implementation.
type ActivityRegistry struct {
	byType map[ast.StepType]Activity
}

// NewActivityRegistry builds a registry pre-populated with the default
// in-process activity implementations. Replace activities by calling Register.
func NewActivityRegistry(human HumanInTheLoopActivity) *ActivityRegistry {
	r := &ActivityRegistry{byType: map[ast.StepType]Activity{}}
	r.Register(ast.StepSkill, &SkillActivity{})
	r.Register(ast.StepMCP, &MCPActivity{})
	r.Register(ast.StepPrompt, &PromptActivity{})
	r.Register(ast.StepBranch, &BranchActivity{})
	r.Register(ast.StepLoop, &LoopActivity{})
	r.Register(ast.StepSubWorkflow, &SubWorkflowActivity{})
	r.Register(ast.StepEventTrigger, &EventTriggerActivity{})
	if human == nil {
		human = NoopHumanActivity{}
	}
	r.Register(ast.StepHumanInLoop, human)
	return r
}

// Register adds or replaces an activity for a given step type.
func (r *ActivityRegistry) Register(t ast.StepType, a Activity) { r.byType[t] = a }

// Resolve returns the activity for a step type.
func (r *ActivityRegistry) Resolve(t ast.StepType) (Activity, error) {
	a, ok := r.byType[t]
	if !ok {
		return nil, fmt.Errorf("no_activity_for_type: %s", t)
	}
	return a, nil
}

// SkillActivity calls a Skill via the SDK. The SDK call is intentionally
// abstracted: the production binary plugs in the real client; in dry-run
// the activity returns mock outputs.
type SkillActivity struct{}

func (s *SkillActivity) Execute(_ context.Context, in ActivityInput) (ActivityOutput, error) {
	if in.DryRun {
		return ActivityOutput{Outputs: dryRunOutputs(in.Step.ID)}, nil
	}
	// Production wiring is in cmd/main.go.
	return ActivityOutput{Outputs: map[string]any{"skill_ref": in.Step.Ref}}, nil
}

// MCPActivity calls an MCP tool.
type MCPActivity struct{}

func (m *MCPActivity) Execute(_ context.Context, in ActivityInput) (ActivityOutput, error) {
	if in.DryRun {
		return ActivityOutput{Outputs: dryRunOutputs(in.Step.ID)}, nil
	}
	return ActivityOutput{Outputs: map[string]any{
		"mcp_ref": in.Step.Ref,
		"tool":    in.Step.Tool,
	}}, nil
}

// PromptActivity renders and runs a Prompt template.
type PromptActivity struct{}

func (p *PromptActivity) Execute(_ context.Context, in ActivityInput) (ActivityOutput, error) {
	if in.DryRun {
		return ActivityOutput{Outputs: dryRunOutputs(in.Step.ID)}, nil
	}
	return ActivityOutput{Outputs: map[string]any{"prompt_ref": in.Step.Ref}}, nil
}

// BranchActivity is a no-op; the engine handles branching directly.
type BranchActivity struct{}

func (b *BranchActivity) Execute(_ context.Context, _ ActivityInput) (ActivityOutput, error) {
	return ActivityOutput{Outputs: map[string]any{}}, nil
}

// LoopActivity is a no-op; the engine handles loop iteration directly.
type LoopActivity struct{}

func (l *LoopActivity) Execute(_ context.Context, _ ActivityInput) (ActivityOutput, error) {
	return ActivityOutput{Outputs: map[string]any{}}, nil
}

// SubWorkflowActivity launches a child workflow.
type SubWorkflowActivity struct{}

func (s *SubWorkflowActivity) Execute(_ context.Context, in ActivityInput) (ActivityOutput, error) {
	if in.DryRun {
		return ActivityOutput{Outputs: dryRunOutputs(in.Step.ID)}, nil
	}
	return ActivityOutput{Outputs: map[string]any{"sub_workflow": in.Step.Ref}}, nil
}

// EventTriggerActivity simply records that the trigger fired; the engine
// treats it as a starting node.
type EventTriggerActivity struct{}

func (e *EventTriggerActivity) Execute(_ context.Context, _ ActivityInput) (ActivityOutput, error) {
	return ActivityOutput{Outputs: map[string]any{}}, nil
}

// HumanInTheLoopActivity is the contract the engine relies on for HITL steps.
// The default no-op implementation suspends the execution awaiting an external
// signal that the engine processes via SignalWorkflow.
type HumanInTheLoopActivity interface {
	Execute(ctx context.Context, in ActivityInput) (ActivityOutput, error)
}

// NoopHumanActivity returns Wait=true with no side effects. Production wires
// in a real Approvals Inbox client.
type NoopHumanActivity struct{}

func (NoopHumanActivity) Execute(_ context.Context, in ActivityInput) (ActivityOutput, error) {
	return ActivityOutput{
		Outputs: map[string]any{},
		Wait:    true,
		Reason:  "awaiting_human_approval",
	}, nil
}

func dryRunOutputs(stepID string) map[string]any {
	return map[string]any{
		"dry_run": true,
		"step_id": stepID,
	}
}
