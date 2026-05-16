// Package ast defines the canonical AST for Forge workflows.
// Both the YAML DSL and the visual editor produce the same AST.
package ast

// APIVersion is the canonical apiVersion for forge workflows.
const APIVersion = "forge.workflows/v1"

// Kind is the kind of object — currently only "Workflow".
const Kind = "Workflow"

// StepType enumerates supported step node types. The Go enum is the single
// source of truth: the TS CanonicalStepType mirror lives at
// portal/src/lib/ast-canvas-adapter/index.ts and a parity test in
// pkg/workflow/ast/parity_test.go fails CI on drift in either direction.
type StepType string

const (
	StepSkill              StepType = "skill"
	StepMCP                StepType = "mcp"
	StepLLM                StepType = "llm"
	StepAgent              StepType = "agent"
	StepPromptTemplate     StepType = "prompt-template"
	StepBranch             StepType = "branch"
	StepLoop               StepType = "loop"
	StepHumanInLoop        StepType = "human-in-the-loop"
	StepSubWorkflow        StepType = "sub-workflow"
	StepWebhookOut         StepType = "webhook"
	StepGithubAction       StepType = "github-action"
	StepDeployAction       StepType = "deploy-action"
	StepApprovalAction     StepType = "approval-action"
	StepNotificationAction StepType = "notification-action"
	StepEval               StepType = "eval"
	StepCustom             StepType = "custom"

	// Deprecated: use StepPromptTemplate. Parser auto-aliases on load and
	// the DSL layer emits a deprecated_step_kind lint warning; the form is
	// migrated to prompt-template on next save.
	StepPrompt StepType = "prompt"
	// Deprecated: model triggers as entries in spec.Triggers instead. Parser
	// auto-migrates existing event-trigger steps into the triggers block and
	// emits a deprecated_step_kind lint warning.
	StepEventTrigger StepType = "event-trigger"
)

// AllStepTypes returns the canonical list of supported step types.
// Includes the deprecated values so legacy workflows continue to parse.
func AllStepTypes() []StepType {
	return []StepType{
		StepSkill,
		StepMCP,
		StepLLM,
		StepAgent,
		StepPromptTemplate,
		StepBranch,
		StepLoop,
		StepHumanInLoop,
		StepSubWorkflow,
		StepWebhookOut,
		StepGithubAction,
		StepDeployAction,
		StepApprovalAction,
		StepNotificationAction,
		StepEval,
		StepCustom,
		StepPrompt,       // deprecated alias of StepPromptTemplate
		StepEventTrigger, // deprecated; migrated into spec.Triggers
	}
}

// DeprecatedStepTypes returns the set of step types that still parse but
// emit a deprecated_step_kind lint warning. The DSL layer migrates these
// shapes to their canonical replacements on save.
func DeprecatedStepTypes() map[StepType]StepType {
	return map[StepType]StepType{
		StepPrompt:       StepPromptTemplate,
		StepEventTrigger: "", // migrated to spec.Triggers, no step replacement
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

	// Triggers declares zero or more event sources that fire executions
	// of this workflow. When empty (default), the workflow is invoke-only
	// — executions are started by direct POST to workflow-runtime. See
	// the automation-triggers capability for the full type set and
	// semantics.
	Triggers []Trigger `yaml:"triggers,omitempty" json:"triggers,omitempty"`
}

// TriggerType enumerates supported trigger sources. Source of truth:
// pkg/workflow/ast/catalog.json (trigger_types). Drift between this enum
// and the catalog is caught by pkg/workflow/ast/parity_test.go.
type TriggerType string

const (
	TriggerManual       TriggerType = "manual"
	TriggerCron         TriggerType = "cron"
	TriggerWebhookIn    TriggerType = "webhook-in"
	TriggerEventBus     TriggerType = "event-bus"
	TriggerEmailInbound TriggerType = "email-inbound"
)

// AllTriggerTypes returns the canonical list of supported trigger types.
func AllTriggerTypes() []TriggerType {
	return []TriggerType{
		TriggerManual,
		TriggerCron,
		TriggerWebhookIn,
		TriggerEventBus,
		TriggerEmailInbound,
	}
}

// IsKnownTriggerType reports whether t is a recognised trigger type.
func IsKnownTriggerType(t TriggerType) bool {
	for _, k := range AllTriggerTypes() {
		if k == t {
			return true
		}
	}
	return false
}

// TriggerConcurrency controls how the runtime handles a new firing while a
// prior execution is still running. See workflow-runtime spec.
type TriggerConcurrency string

const (
	TriggerConcurrencyQueue   TriggerConcurrency = "queue"
	TriggerConcurrencyDrop    TriggerConcurrency = "drop"
	TriggerConcurrencyOverlap TriggerConcurrency = "overlap"
)

// Trigger declares one event source that fires executions of this workflow.
// Steps reference the trigger payload via $triggers.<id>.<field>; references
// to undeclared fields fail lint at publish time.
type Trigger struct {
	ID          string             `yaml:"id" json:"id"`
	Type        TriggerType        `yaml:"type" json:"type"`
	Config      map[string]any     `yaml:"config,omitempty" json:"config,omitempty"`
	Outputs     map[string]string  `yaml:"outputs,omitempty" json:"outputs,omitempty"`
	Concurrency TriggerConcurrency `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`

	// MigratedFrom records the original step shape when this trigger
	// entry was produced by the auto-migration of a legacy `event-trigger`
	// step. Not serialised; in-memory only. Lint uses it to emit a
	// deprecated_step_kind warning citing the migration.
	MigratedFrom StepType `yaml:"-" json:"-"`
}

// ConcurrencyOrDefault returns the trigger's concurrency policy, or the
// canonical default ("queue") when unset.
func (t Trigger) ConcurrencyOrDefault() TriggerConcurrency {
	if t.Concurrency == "" {
		return TriggerConcurrencyQueue
	}
	return t.Concurrency
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

	// event-trigger (deprecated — see Spec.Triggers)
	EventPattern *EventPattern `yaml:"event_pattern,omitempty" json:"event_pattern,omitempty"`

	// llm — LLM step shape (see llm-flow-node capability).
	//
	// PromptTemplate references a prompt-template-service asset
	// (registry:prompt/<scope>/<name>@<semver>). Floating tags are
	// rejected at lint time.
	PromptTemplate string         `yaml:"prompt_template,omitempty" json:"prompt_template,omitempty"`
	// Model resolves to a concrete model + credentials at execution time
	// via the model-gateway.
	Model *ModelBinding `yaml:"model,omitempty" json:"model,omitempty"`
	// Tools are MCP refs auto-bound to the LLM call. Tools MUST be a
	// subset of the workflow's pinned `selected_assets.mcps` when pinned.
	Tools []string `yaml:"tools,omitempty" json:"tools,omitempty"`
	// MaxToolCalls bounds tool-calling per LLM step (default 10 enforced
	// at runtime when zero/unset).
	MaxToolCalls int `yaml:"max_tool_calls,omitempty" json:"max_tool_calls,omitempty"`
	// StepOutputs declares the typed output schema of the step. For LLM
	// steps the runtime validates the model response against it before
	// passing control downstream; for other step types it serves as
	// documentation consumed by the editor's field-mapping UX. Distinct
	// from the legacy free-form Outputs []string above (which carries
	// only output names, no types).
	StepOutputs map[string]string `yaml:"outputs_schema,omitempty" json:"outputs_schema,omitempty"`

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

	// MigratedFrom records the original step type when the DSL parser
	// applied a deprecated-shape migration on read (e.g. prompt ->
	// prompt-template). Not serialised; in-memory only. Lint uses it to
	// emit a deprecated_step_kind warning citing the original shape.
	MigratedFrom StepType `yaml:"-" json:"-"`
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
//
// Deprecated: prefer Spec.Triggers entries of type webhook-in / event-bus.
// The DSL layer auto-migrates EventPattern-bearing event-trigger steps
// into trigger-block entries on parse.
type EventPattern struct {
	Type        string         `yaml:"type" json:"type"`
	Source      string         `yaml:"source,omitempty" json:"source,omitempty"`
	Filter      map[string]any `yaml:"filter,omitempty" json:"filter,omitempty"`
}

// ModelBinding selects an LLM model + credentials via the model-gateway.
// The Ref form is `gateway:model/<model-id>@<channel>` where <channel> is
// one of `latest-stable`, `latest`, or a pinned version. Overrides are
// passed through to the gateway request unchanged (temperature, max_tokens,
// top_p, etc.).
type ModelBinding struct {
	Ref       string         `yaml:"ref" json:"ref"`
	Overrides map[string]any `yaml:"overrides,omitempty" json:"overrides,omitempty"`
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
