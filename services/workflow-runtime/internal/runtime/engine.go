package runtime

import (
	"context"
	"errors"
)

// TemporalEngine abstracts a Temporal-shaped workflow engine. The production
// implementation wraps the Temporal Go SDK; the in-memory implementation is
// used for tests and local development.
type TemporalEngine interface {
	StartWorkflow(ctx context.Context, req StartRequest) (*Execution, error)
	SignalWorkflow(ctx context.Context, req SignalRequest) (*Execution, error)
	CancelWorkflow(ctx context.Context, tenantID, executionID string) (*Execution, error)
	QueryWorkflow(ctx context.Context, req QueryRequest) (any, error)
	GetExecution(ctx context.Context, tenantID, executionID string) (*Execution, error)
	ListExecutions(ctx context.Context, tenantID, workspaceID string) []*Execution
	EnsureNamespace(tenantID string) string
}

// Activity is a unit of work invoked by the engine for a step.
//
// Activities are by-type: every step.Type is dispatched to the activity
// registered for it. Activities MUST be idempotent or have a paired
// compensation declared via `compensate_with`.
type Activity interface {
	Execute(ctx context.Context, in ActivityInput) (ActivityOutput, error)
}

// ActivityInput carries the resolved inputs for a step.
type ActivityInput struct {
	TenantID    string
	WorkspaceID string
	ExecutionID string
	Step        StepRuntimeInfo
	Inputs      map[string]any
	DryRun      bool
}

// StepRuntimeInfo is the static portion of a step that activities need.
type StepRuntimeInfo struct {
	ID        string
	Type      string
	Ref       string
	Tool      string
	Timeout   string
	Approver  string
	OnTimeout string
}

// ActivityOutput is the result of an activity.
type ActivityOutput struct {
	Outputs map[string]any
	Wait    bool   // true if the activity should suspend execution awaiting a signal (HITL)
	Reason  string // free-form reason when Wait=true
}

// ErrCrossTenantAccess is returned when an actor in tenant A attempts to
// signal/cancel a workflow in tenant B.
var ErrCrossTenantAccess = errors.New("cross_tenant_access_denied")

// ErrExecutionNotFound is returned when an execution id is unknown.
var ErrExecutionNotFound = errors.New("execution_not_found")
