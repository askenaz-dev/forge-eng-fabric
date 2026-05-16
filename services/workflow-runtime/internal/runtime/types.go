// Package runtime executes workflows on Temporal-shaped activities.
//
// The package is structured around a TemporalEngine interface so that the
// service can run against a real Temporal cluster in production while staying
// testable in-process via an InMemoryEngine. Each Tenant gets its own
// namespace; cross-namespace operations are denied.
package runtime

import (
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// ExecutionStatus enumerates execution lifecycle states.
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusWaiting   ExecutionStatus = "waiting" // human-in-the-loop or external signal
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
	StatusCancelled ExecutionStatus = "cancelled"
	StatusCompensating ExecutionStatus = "compensating"
)

// StepStatus enumerates per-step lifecycle.
type StepStatus string

const (
	StepStatusPending      StepStatus = "pending"
	StepStatusRunning      StepStatus = "running"
	StepStatusWaiting      StepStatus = "waiting"
	StepStatusCompleted    StepStatus = "completed"
	StepStatusFailed       StepStatus = "failed"
	StepStatusCompensated  StepStatus = "compensated"
	StepStatusSkipped      StepStatus = "skipped"
)

// Execution describes a workflow execution.
type Execution struct {
	ID             string             `json:"id"`
	TenantID       string             `json:"tenant_id"`
	WorkspaceID    string             `json:"workspace_id"`
	Namespace      string             `json:"namespace"`
	WorkflowID     string             `json:"workflow_id"`
	Version        string             `json:"version"`
	CorrelationID  string             `json:"correlation_id,omitempty"`
	Status         ExecutionStatus    `json:"status"`
	Inputs         map[string]any     `json:"inputs,omitempty"`
	Outputs        map[string]any     `json:"outputs,omitempty"`
	Steps          []StepEvent        `json:"steps"`
	StartedAt      time.Time          `json:"started_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	CompletedAt    *time.Time         `json:"completed_at,omitempty"`
	FailureReason  string             `json:"failure_reason,omitempty"`
	Compensations  []CompensationItem `json:"compensations,omitempty"`
	DryRun         bool               `json:"dry_run,omitempty"`
	SelectedAssets *SelectedAssets    `json:"selected_assets,omitempty"`
	TriggerEvent   *TriggerEvent      `json:"trigger_event,omitempty"`
}

// SelectedAssets is the pinned set the wizard / visual editor saved on
// the workflow. When non-empty, the engine refuses to invoke any asset
// not present in the corresponding list. Empty means no pinning — the
// engine reverts to its pre-change behavior.
//
// The id values are matched against `step.ref` for skill / mcp / agent
// steps. Empty SelectedAssets (or nil) preserves the current open
// behavior.
type SelectedAssets struct {
	Skills []string `json:"skills,omitempty"`
	MCPs   []string `json:"mcps,omitempty"`
	Agents []string `json:"agents,omitempty"`
}

// IsEmpty reports whether the pinned set has nothing to enforce.
func (s *SelectedAssets) IsEmpty() bool {
	if s == nil {
		return true
	}
	return len(s.Skills) == 0 && len(s.MCPs) == 0 && len(s.Agents) == 0
}

// Allows reports whether a step with the given type+ref is in the pinned
// set. Empty pinned set always allows (no enforcement); empty sub-list
// for the step's family denies (deny-by-default within a pinned set).
func (s *SelectedAssets) Allows(stepType, ref string) bool {
	if s.IsEmpty() {
		return true
	}
	var list []string
	switch stepType {
	case "skill":
		list = s.Skills
	case "mcp":
		list = s.MCPs
	case "agent", "sub_workflow":
		list = s.Agents
	default:
		// Non-asset steps (branch, loop, prompt, hitl, event) are always
		// allowed; pinning targets asset invocation only.
		return true
	}
	for _, a := range list {
		if a == ref {
			return true
		}
	}
	return false
}

// StepEvent records a single step transition; multiple events for the
// same step are kept in order (running → completed/failed).
type StepEvent struct {
	StepID       string         `json:"step_id"`
	Type         ast.StepType   `json:"type"`
	Status       StepStatus     `json:"status"`
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	Inputs       map[string]any `json:"inputs,omitempty"`
	Outputs      map[string]any `json:"outputs,omitempty"`
	Attempt      int            `json:"attempt"`
	FailureReason string        `json:"failure_reason,omitempty"`
}

// CompensationItem records a saga-reverse activity invocation.
type CompensationItem struct {
	ForStepID    string    `json:"for_step_id"`
	CompensateID string    `json:"compensate_id"`
	At           time.Time `json:"at"`
	Outcome      string    `json:"outcome"`
}

// StartRequest is the input for starting a workflow execution.
type StartRequest struct {
	TenantID       string          `json:"tenant_id"`
	WorkspaceID    string          `json:"workspace_id"`
	Workflow       *ast.Workflow   `json:"workflow"`
	Inputs         map[string]any  `json:"inputs,omitempty"`
	CorrelationID  string          `json:"correlation_id,omitempty"`
	DryRun         bool            `json:"dry_run,omitempty"`
	SelectedAssets *SelectedAssets `json:"selected_assets,omitempty"`
	// TriggerEvent is populated by trigger-router when an execution is
	// fired by a registered trigger. Steps may reference its payload via
	// $triggers.<trigger_id>.<field>. When TriggerEvent is nil, any step
	// expression referencing $triggers.* is rejected at run time with
	// unbound_trigger_reference (see ai-flow-authoring change). Direct-
	// POST executions without triggers behave exactly as before.
	TriggerEvent *TriggerEvent `json:"trigger_event,omitempty"`
}

// TriggerEvent describes the firing that started a trigger-originated
// execution. trigger_id MUST match a Trigger.ID in the workflow AST;
// payload is the event body shaped according to the trigger's declared
// outputs schema.
type TriggerEvent struct {
	TriggerID     string         `json:"trigger_id"`
	FiredAt       time.Time      `json:"fired_at"`
	Payload       map[string]any `json:"payload,omitempty"`
	QueuePosition int            `json:"queue_position,omitempty"`
}

// SignalRequest is the input for delivering a signal to an execution.
type SignalRequest struct {
	TenantID    string         `json:"tenant_id"`
	ExecutionID string         `json:"execution_id"`
	Signal      string         `json:"signal"`
	Payload     map[string]any `json:"payload,omitempty"`
}

// QueryRequest is the input for querying state.
type QueryRequest struct {
	TenantID    string `json:"tenant_id"`
	ExecutionID string `json:"execution_id"`
	StepID      string `json:"step_id,omitempty"`
}
