// Package ast defines the canonical AST for Forge workflows.
// Both the YAML DSL and the visual editor produce the same AST.
package ast

// APIVersion is the canonical apiVersion for forge workflows.
const APIVersion = "forge.workflows/v1"

// Kind is the kind of object — currently only "Workflow".
const Kind = "Workflow"

// StepType enumerates supported step node types.
type StepType string

const (
	StepSkill        StepType = "skill"
	StepMCP          StepType = "mcp"
	StepPrompt       StepType = "prompt"
	StepBranch       StepType = "branch"
	StepLoop         StepType = "loop"
	StepHumanInLoop  StepType = "human-in-the-loop"
	StepSubWorkflow  StepType = "sub-workflow"
	StepEventTrigger StepType = "event-trigger"
)

// AllStepTypes returns the canonical list of supported step types.
func AllStepTypes() []StepType {
	return []StepType{
		StepSkill,
		StepMCP,
		StepPrompt,
		StepBranch,
		StepLoop,
		StepHumanInLoop,
		StepSubWorkflow,
		StepEventTrigger,
	}
}

// Visibility tier for marketplace listings.
type Visibility string

const (
	VisibilityPrivate        Visibility = "private"
	VisibilityWorkspace      Visibility = "workspace"
	VisibilityTenant         Visibility = "tenant"
	VisibilityForgeCertified Visibility = "forge-certified"
)

// Criticality echoes the asset registry conventions.
type Criticality string

const (
	CriticalityLow      Criticality = "low"
	CriticalityMedium   Criticality = "medium"
	CriticalityHigh     Criticality = "high"
	CriticalityCritical Criticality = "critical"
)

// Workflow is the top-level AST node.
type Workflow struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`
}

// Metadata contains identifying and governance information.
type Metadata struct {
	ID            string      `yaml:"id" json:"id"`
	Name          string      `yaml:"name" json:"name"`
	Version       string      `yaml:"version" json:"version"`
	Owners        []string    `yaml:"owners,omitempty" json:"owners,omitempty"`
	Visibility    Visibility  `yaml:"visibility,omitempty" json:"visibility,omitempty"`
	Criticality   Criticality `yaml:"criticality,omitempty" json:"criticality,omitempty"`
	OpenSpecIDs   []string    `yaml:"openspec_ids,omitempty" json:"openspec_ids,omitempty"`
	Tags          []string    `yaml:"tags,omitempty" json:"tags,omitempty"`
	Description   string      `yaml:"description,omitempty" json:"description,omitempty"`
	SuccessMetric string      `yaml:"success_metric,omitempty" json:"success_metric,omitempty"`
}

// Spec contains the executable definition.
type Spec struct {
	Inputs    []IOField `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Outputs   []IOField `yaml:"outputs,omitempty" json:"outputs,omitempty"`
	Steps     []Step    `yaml:"steps" json:"steps"`
	OnFailure []Step    `yaml:"on_failure,omitempty" json:"on_failure,omitempty"`
}

// IOField is a typed input/output field declared on the workflow.
type IOField struct {
	Name        string         `yaml:"name" json:"name"`
	Type        string         `yaml:"type" json:"type"`
	Required    bool           `yaml:"required,omitempty" json:"required,omitempty"`
	Default     any            `yaml:"default,omitempty" json:"default,omitempty"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Validations map[string]any `yaml:"validations,omitempty" json:"validations,omitempty"`
}

// Step is a node in the workflow.
//
// Step is intentionally a single struct (not a sum type via interfaces) because
// the canonical AST must be losslessly serialisable to/from YAML and JSON.
// Type-specific fields use omitempty.
type Step struct {
	ID          string         `yaml:"id" json:"id"`
	Type        StepType       `yaml:"type" json:"type"`
	Ref         string         `yaml:"ref,omitempty" json:"ref,omitempty"`
	Tool        string         `yaml:"tool,omitempty" json:"tool,omitempty"`
	DependsOn   []string       `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Inputs      map[string]any `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Outputs     []string       `yaml:"outputs,omitempty" json:"outputs,omitempty"`
	Retries     *RetryPolicy   `yaml:"retries,omitempty" json:"retries,omitempty"`
	Timeout     string         `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	CompensateWith string      `yaml:"compensate_with,omitempty" json:"compensate_with,omitempty"`

	// branch
	Branches []Branch `yaml:"branches,omitempty" json:"branches,omitempty"`

	// loop
	ForEach    string `yaml:"for_each,omitempty" json:"for_each,omitempty"`
	MaxParallel int   `yaml:"max_parallel,omitempty" json:"max_parallel,omitempty"`
	BodyRef    string `yaml:"body_ref,omitempty" json:"body_ref,omitempty"`
	Body       []Step `yaml:"body,omitempty" json:"body,omitempty"`

	// human-in-the-loop
	ApproverRole    string         `yaml:"approver_role,omitempty" json:"approver_role,omitempty"`
	OnTimeout       string         `yaml:"on_timeout,omitempty" json:"on_timeout,omitempty"`
	EscalationRole  string         `yaml:"escalation_role,omitempty" json:"escalation_role,omitempty"`
	ExpectedOutputs map[string]any `yaml:"expected_outputs,omitempty" json:"expected_outputs,omitempty"`

	// sub-workflow
	WorkflowRef     string `yaml:"workflow_ref,omitempty" json:"workflow_ref,omitempty"`
	WorkflowVersion string `yaml:"workflow_version,omitempty" json:"workflow_version,omitempty"`

	// event-trigger
	EventPattern *EventPattern `yaml:"event_pattern,omitempty" json:"event_pattern,omitempty"`

	// active_surface pins the gateway endpoint chosen for this step at
	// design time. Persisted by the visual editor (active-registry-gateways
	// §7.5) so the runtime does not need to re-resolve the surface from
	// the registry on every dispatch. Optional; when absent the runtime
	// resolves the surface lazily as before.
	ActiveSurface *NodeActiveSurface `yaml:"active_surface,omitempty" json:"active_surface,omitempty"`

	// targets declares the SDLC phase-target override for this step.
	// The key is the canonical SDLC phase name (e.g. "iac", "qa"); the value
	// is one of the four allowed policy strings. When present the orchestrator
	// merges this map on top of App.targets at plan-build time subject to the
	// tightening rule: a per-step override MAY only make a phase more strict,
	// never more permissive. See sdlc-end-to-end spec for full semantics.
	Targets map[string]string `yaml:"targets,omitempty" json:"targets,omitempty"`
}

// NodeActiveSurface is the per-step projection of an Asset Registry
// row's active_surface block. The shape mirrors Asset.active_surface so
// round-trips via JSON/YAML preserve the field byte-for-byte.
type NodeActiveSurface struct {
	Family          string `yaml:"family" json:"family"`
	Endpoint        string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	ArtifactPointer string `yaml:"artifact_pointer,omitempty" json:"artifact_pointer,omitempty"`
	Digest          string `yaml:"digest,omitempty" json:"digest,omitempty"`
	SignatureID     string `yaml:"signature_id,omitempty" json:"signature_id,omitempty"`
}

// Branch is one arm of a `branch` node.
type Branch struct {
	When  string `yaml:"when" json:"when"`
	Goto  string `yaml:"goto,omitempty" json:"goto,omitempty"`
	Steps []Step `yaml:"steps,omitempty" json:"steps,omitempty"`
}

// RetryPolicy maps to Temporal RetryPolicy.
type RetryPolicy struct {
	Max         int    `yaml:"max" json:"max"`
	Backoff     string `yaml:"backoff,omitempty" json:"backoff,omitempty"`
	InitialMS   int    `yaml:"initial_ms,omitempty" json:"initial_ms,omitempty"`
	MaxMS       int    `yaml:"max_ms,omitempty" json:"max_ms,omitempty"`
	NonRetryable []string `yaml:"non_retryable,omitempty" json:"non_retryable,omitempty"`
}

// EventPattern matches CloudEvents that should trigger a workflow.
type EventPattern struct {
	Type        string         `yaml:"type" json:"type"`
	Source      string         `yaml:"source,omitempty" json:"source,omitempty"`
	Filter      map[string]any `yaml:"filter,omitempty" json:"filter,omitempty"`
}

// IsKnownStepType reports whether `s` is a recognised step type.
func IsKnownStepType(s StepType) bool {
	for _, t := range AllStepTypes() {
		if t == s {
			return true
		}
	}
	return false
}
