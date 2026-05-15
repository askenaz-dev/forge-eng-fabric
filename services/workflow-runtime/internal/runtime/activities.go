package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// MCPGatewayClient is the seam the MCP activity uses to reach the
// platform's mcp-gateway via pkg/mcp-shim. The runtime package keeps the
// surface minimal so it doesn't take a cross-module dependency on the
// shim itself; cmd/main.go wires the real implementation at boot.
type MCPGatewayClient interface {
	InvokeTool(ctx context.Context, assetID, toolName string, body any) ([]byte, int, error)
}

// A2AGatewayClient is the equivalent seam for tasks/send invocations
// against the a2a-gateway. Sub-workflow style agent calls and explicit
// agent steps both go through this client.
type A2AGatewayClient interface {
	Send(ctx context.Context, assetID, method string, params any) ([]byte, int, error)
}

// ErrGatewayClientMissing is returned when an activity is asked to run
// against a real (non-dry-run) input but no gateway client was wired.
// The engine surfaces this as a structured failure on the step.
var ErrGatewayClientMissing = errors.New("gateway_client_missing")

// ActivityRegistry resolves a step type to its activity implementation.
type ActivityRegistry struct {
	byType map[ast.StepType]Activity
}

// RegistryOptions configures the default activity wiring. Production
// passes the gateway clients; tests can pass nil to keep the stub
// behavior and still exercise pinned-set / state-machine paths.
type RegistryOptions struct {
	Human    HumanInTheLoopActivity
	MCP      MCPGatewayClient
	A2A      A2AGatewayClient
	Enforced bool // when true, non-dry-run calls without a gateway client fail closed
}

// NewActivityRegistry builds a registry pre-populated with the default
// in-process activity implementations. Replace activities by calling Register.
func NewActivityRegistry(human HumanInTheLoopActivity) *ActivityRegistry {
	return NewActivityRegistryWithOptions(RegistryOptions{Human: human})
}

// NewActivityRegistryWithOptions is the explicit constructor that wires
// gateway clients into the MCP / A2A activities. Both options are
// optional in dev / dry-run; production passes both.
func NewActivityRegistryWithOptions(opts RegistryOptions) *ActivityRegistry {
	r := &ActivityRegistry{byType: map[ast.StepType]Activity{}}
	r.Register(ast.StepSkill, &SkillActivity{MCP: opts.MCP, Enforced: opts.Enforced})
	r.Register(ast.StepMCP, &MCPActivity{Client: opts.MCP, Enforced: opts.Enforced})
	r.Register(ast.StepPrompt, &PromptActivity{})
	r.Register(ast.StepBranch, &BranchActivity{})
	r.Register(ast.StepLoop, &LoopActivity{})
	r.Register(ast.StepSubWorkflow, &SubWorkflowActivity{A2A: opts.A2A, Enforced: opts.Enforced})
	r.Register(ast.StepEventTrigger, &EventTriggerActivity{})
	human := opts.Human
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

// SkillActivity calls a Skill — for now via the mcp-gateway path, since
// the platform routes skill invocations through mcp-gateway with the
// `family=skill` active surface metadata. When `Enforced=true` the
// activity refuses to fall back to the stub even in non-dry-run.
type SkillActivity struct {
	MCP      MCPGatewayClient
	Enforced bool
}

func (s *SkillActivity) Execute(ctx context.Context, in ActivityInput) (ActivityOutput, error) {
	if in.DryRun {
		return ActivityOutput{Outputs: dryRunOutputs(in.Step.ID)}, nil
	}
	if s.MCP == nil {
		if s.Enforced {
			return ActivityOutput{}, fmt.Errorf("%w: skill step requires mcp-gateway client when gateway.enforced=true", ErrGatewayClientMissing)
		}
		// Pre-enforcement compatibility: return the stub. The deprecation
		// telemetry the shim emits will fire from the cmd/main.go wiring
		// because that path uses the gateway client.
		return ActivityOutput{Outputs: map[string]any{"skill_ref": in.Step.Ref}}, nil
	}
	body, status, err := s.MCP.InvokeTool(ctx, in.Step.Ref, in.Step.Tool, in.Inputs)
	if err != nil {
		return ActivityOutput{}, fmt.Errorf("skill invoke failed: %w", err)
	}
	if status/100 != 2 {
		return ActivityOutput{}, fmt.Errorf("skill upstream returned status=%d", status)
	}
	return ActivityOutput{Outputs: parseGatewayResponse(body, in.Step.Ref)}, nil
}

// MCPActivity calls an MCP tool through pkg/mcp-shim. The activity goes
// through the gateway whenever a client is wired; without one it returns
// the legacy stub, matching how older callers behaved before the
// active-registry-gateways change.
type MCPActivity struct {
	Client   MCPGatewayClient
	Enforced bool
}

func (m *MCPActivity) Execute(ctx context.Context, in ActivityInput) (ActivityOutput, error) {
	if in.DryRun {
		return ActivityOutput{Outputs: dryRunOutputs(in.Step.ID)}, nil
	}
	if m.Client == nil {
		if m.Enforced {
			return ActivityOutput{}, fmt.Errorf("%w: mcp step requires mcp-gateway client when gateway.enforced=true", ErrGatewayClientMissing)
		}
		return ActivityOutput{Outputs: map[string]any{
			"mcp_ref": in.Step.Ref,
			"tool":    in.Step.Tool,
		}}, nil
	}
	body, status, err := m.Client.InvokeTool(ctx, in.Step.Ref, in.Step.Tool, in.Inputs)
	if err != nil {
		return ActivityOutput{}, fmt.Errorf("mcp invoke failed: %w", err)
	}
	if status/100 != 2 {
		return ActivityOutput{}, fmt.Errorf("mcp upstream returned status=%d", status)
	}
	return ActivityOutput{Outputs: parseGatewayResponse(body, in.Step.Ref)}, nil
}

// parseGatewayResponse normalizes the gateway response body into the
// activity's output map. The gateway transparently relays the upstream
// JSON, so we either return the parsed JSON or wrap raw bytes under a
// `raw` key when they are not JSON-decodable.
func parseGatewayResponse(body []byte, ref string) map[string]any {
	if len(body) == 0 {
		return map[string]any{"asset_id": ref}
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err == nil {
		parsed["_asset_id"] = ref
		return parsed
	}
	return map[string]any{"asset_id": ref, "raw": string(body)}
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

// SubWorkflowActivity launches a child workflow, and for steps that
// reference an A2A agent (`step.tool` set, ref points at an agent asset)
// it invokes the agent through the a2a-gateway via pkg/a2a-shim.
type SubWorkflowActivity struct {
	A2A      A2AGatewayClient
	Enforced bool
}

func (s *SubWorkflowActivity) Execute(ctx context.Context, in ActivityInput) (ActivityOutput, error) {
	if in.DryRun {
		return ActivityOutput{Outputs: dryRunOutputs(in.Step.ID)}, nil
	}
	// Sub-workflow steps without an agent ref preserve the legacy behavior
	// — they remain in-process child workflows scheduled by the engine.
	if in.Step.Tool == "" || s.A2A == nil {
		if s.Enforced && s.A2A == nil && in.Step.Tool != "" {
			return ActivityOutput{}, fmt.Errorf("%w: agent step requires a2a-gateway client when gateway.enforced=true", ErrGatewayClientMissing)
		}
		return ActivityOutput{Outputs: map[string]any{"sub_workflow": in.Step.Ref}}, nil
	}
	method := in.Step.Tool
	if method == "" {
		method = "tasks/send"
	}
	body, status, err := s.A2A.Send(ctx, in.Step.Ref, method, in.Inputs)
	if err != nil {
		return ActivityOutput{}, fmt.Errorf("a2a invoke failed: %w", err)
	}
	if status/100 != 2 {
		return ActivityOutput{}, fmt.Errorf("a2a upstream returned status=%d", status)
	}
	return ActivityOutput{Outputs: parseGatewayResponse(body, in.Step.Ref)}, nil
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
