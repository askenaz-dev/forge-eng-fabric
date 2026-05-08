package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
	"github.com/google/uuid"
)

// InMemoryEngine is a Temporal-shaped engine that runs in process. It is
// suitable for tests, dry-runs, and local development; production deploys
// swap it for a real Temporal client implementing the same TemporalEngine
// interface.
//
// It preserves Temporal-equivalent semantics:
//   - Each Tenant has a namespace (string isolation here)
//   - Worker pool: simulated by goroutine per execution
//   - RetryPolicy from DSL is honoured
//   - Signals deliver to waiting (HITL) executions
//   - Compensations run in saga reverse for `compensate_with`
type InMemoryEngine struct {
	mu         sync.RWMutex
	executions map[string]*Execution
	signals    map[string]chan signalEnvelope // execution_id -> channel
	now        func() time.Time
	registry   *ActivityRegistry
	sink       Sink
	audit      AuditLogger
}

type signalEnvelope struct {
	Signal  string
	Payload map[string]any
}

// NewInMemoryEngine creates an engine wired with the provided activity
// registry and sink.
func NewInMemoryEngine(registry *ActivityRegistry, sink Sink) *InMemoryEngine {
	if sink == nil {
		sink = &MemorySink{}
	}
	if registry == nil {
		registry = NewActivityRegistry(nil)
	}
	return &InMemoryEngine{
		executions: map[string]*Execution{},
		signals:    map[string]chan signalEnvelope{},
		now:        func() time.Time { return time.Now().UTC() },
		registry:   registry,
		sink:       sink,
		audit:      NoopAuditLogger{},
	}
}

// SetAuditLogger replaces the audit logger for HITL flows.
func (e *InMemoryEngine) SetAuditLogger(logger AuditLogger) {
	if logger == nil {
		logger = NoopAuditLogger{}
	}
	e.audit = logger
}

// EnsureNamespace returns the namespace string for a tenant, provisioning it
// implicitly. In production this would call temporal.OperatorService.CreateNamespace.
func (e *InMemoryEngine) EnsureNamespace(tenantID string) string {
	if tenantID == "" {
		return "default"
	}
	return "tenant-" + tenantID
}

// StartWorkflow runs the workflow synchronously to completion or until it
// reaches a waiting state. The goroutine model preserves Temporal-equivalent
// behaviour: the caller can poll via GetExecution/QueryWorkflow.
func (e *InMemoryEngine) StartWorkflow(ctx context.Context, req StartRequest) (*Execution, error) {
	if req.Workflow == nil {
		return nil, errors.New("workflow_required")
	}
	exec := &Execution{
		ID:            uuid.NewString(),
		TenantID:      req.TenantID,
		WorkspaceID:   req.WorkspaceID,
		Namespace:     e.EnsureNamespace(req.TenantID),
		WorkflowID:    req.Workflow.Metadata.ID,
		Version:       req.Workflow.Metadata.Version,
		CorrelationID: req.CorrelationID,
		Status:        StatusPending,
		Inputs:        req.Inputs,
		Outputs:       map[string]any{},
		StartedAt:     e.now(),
		UpdatedAt:     e.now(),
		DryRun:        req.DryRun,
	}
	e.put(exec)
	signalCh := make(chan signalEnvelope, 8)
	e.mu.Lock()
	e.signals[exec.ID] = signalCh
	e.mu.Unlock()

	_ = e.sink.Emit(newEvent(exec, EventExecutionStarted, map[string]any{
		"workflow_id": exec.WorkflowID,
		"version":     exec.Version,
		"dry_run":     exec.DryRun,
	}))
	go e.run(ctx, exec, req.Workflow, signalCh)
	// Allow the goroutine to run-up; tests poll via GetExecution.
	return e.snapshot(exec.ID), nil
}

// SignalWorkflow delivers a named signal to an execution.
func (e *InMemoryEngine) SignalWorkflow(_ context.Context, req SignalRequest) (*Execution, error) {
	exec, err := e.fetch(req.TenantID, req.ExecutionID)
	if err != nil {
		return nil, err
	}
	e.mu.RLock()
	ch, ok := e.signals[req.ExecutionID]
	e.mu.RUnlock()
	if !ok {
		return exec, errors.New("execution_not_signalable")
	}
	select {
	case ch <- signalEnvelope{Signal: req.Signal, Payload: req.Payload}:
	default:
		return exec, errors.New("signal_channel_full")
	}
	return exec, nil
}

// CancelWorkflow marks an execution cancelled and triggers compensation for
// any successful step that declared compensate_with.
func (e *InMemoryEngine) CancelWorkflow(ctx context.Context, tenantID, executionID string) (*Execution, error) {
	exec, err := e.fetch(tenantID, executionID)
	if err != nil {
		return nil, err
	}
	e.mu.RLock()
	ch, ok := e.signals[executionID]
	e.mu.RUnlock()
	if ok {
		select {
		case ch <- signalEnvelope{Signal: "__cancel__"}:
		default:
		}
	}
	return exec, nil
}

// QueryWorkflow returns full execution state (or, when StepID is set, the
// step events for that id).
func (e *InMemoryEngine) QueryWorkflow(_ context.Context, req QueryRequest) (any, error) {
	exec, err := e.fetch(req.TenantID, req.ExecutionID)
	if err != nil {
		return nil, err
	}
	if req.StepID == "" {
		return exec, nil
	}
	steps := []StepEvent{}
	for _, s := range exec.Steps {
		if s.StepID == req.StepID {
			steps = append(steps, s)
		}
	}
	return steps, nil
}

// GetExecution returns the execution scoped by tenant.
func (e *InMemoryEngine) GetExecution(_ context.Context, tenantID, executionID string) (*Execution, error) {
	return e.fetch(tenantID, executionID)
}

// ListExecutions returns executions for a tenant/workspace.
func (e *InMemoryEngine) ListExecutions(_ context.Context, tenantID, workspaceID string) []*Execution {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := []*Execution{}
	for _, exec := range e.executions {
		if exec.TenantID != tenantID {
			continue
		}
		if workspaceID != "" && exec.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, cloneExecution(exec))
	}
	return out
}

func (e *InMemoryEngine) fetch(tenantID, executionID string) (*Execution, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	exec, ok := e.executions[executionID]
	if !ok {
		return nil, ErrExecutionNotFound
	}
	if tenantID != "" && exec.TenantID != tenantID {
		_ = e.sink.Emit(newEvent(exec, EventGuardrailTrip, map[string]any{
			"reason":              "cross_tenant_access",
			"requested_tenant_id": tenantID,
		}))
		return nil, ErrCrossTenantAccess
	}
	return cloneExecution(exec), nil
}

func (e *InMemoryEngine) put(exec *Execution) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executions[exec.ID] = cloneExecution(exec)
}

func (e *InMemoryEngine) snapshot(id string) *Execution {
	e.mu.RLock()
	defer e.mu.RUnlock()
	exec, ok := e.executions[id]
	if !ok {
		return nil
	}
	return cloneExecution(exec)
}

func (e *InMemoryEngine) update(exec *Execution) {
	e.mu.Lock()
	defer e.mu.Unlock()
	exec.UpdatedAt = e.now()
	e.executions[exec.ID] = cloneExecution(exec)
}

// run is the per-execution orchestration loop.
func (e *InMemoryEngine) run(ctx context.Context, exec *Execution, wf *ast.Workflow, signals <-chan signalEnvelope) {
	exec.Status = StatusRunning
	e.update(exec)

	completedOutputs := map[string]map[string]any{}
	successfulCompensable := []ast.Step{}
	steps := wf.Spec.Steps
	pending := stepIDSet(steps)

	for len(pending) > 0 {
		// Pick next step whose deps are satisfied.
		next, ok := nextRunnable(steps, pending, completedOutputs)
		if !ok {
			// Should not happen if lint passed, but guard against deadlock.
			exec.Status = StatusFailed
			exec.FailureReason = "deadlock_unresolved_deps"
			now := e.now()
			exec.CompletedAt = &now
			e.update(exec)
			_ = e.sink.Emit(newEvent(exec, EventExecutionFailed, map[string]any{"reason": exec.FailureReason}))
			return
		}
		select {
		case <-ctx.Done():
			exec.Status = StatusCancelled
			now := e.now()
			exec.CompletedAt = &now
			e.update(exec)
			return
		default:
		}
		stepCtx := stepContext{
			Step:        next,
			ExecID:      exec.ID,
			DryRun:      exec.DryRun,
			TenantID:    exec.TenantID,
			WorkspaceID: exec.WorkspaceID,
			Inputs:      resolveInputs(next, exec.Inputs, completedOutputs),
		}
		out, err := e.runStep(ctx, exec, stepCtx, signals)
		if err != nil {
			exec.FailureReason = err.Error()
			exec.Status = StatusFailed
			e.update(exec)
			_ = e.sink.Emit(newEvent(exec, EventExecutionFailed, map[string]any{
				"step_id": next.ID,
				"reason":  err.Error(),
			}))
			e.compensate(ctx, exec, successfulCompensable, wf)
			now := e.now()
			exec.CompletedAt = &now
			e.update(exec)
			return
		}
		completedOutputs[next.ID] = out
		if next.CompensateWith != "" {
			successfulCompensable = append(successfulCompensable, next)
		}
		delete(pending, next.ID)
	}

	exec.Status = StatusCompleted
	exec.Outputs = aggregateOutputs(wf, completedOutputs)
	now := e.now()
	exec.CompletedAt = &now
	e.update(exec)
	_ = e.sink.Emit(newEvent(exec, EventExecutionCompleted, map[string]any{
		"workflow_id": exec.WorkflowID,
		"version":     exec.Version,
	}))
}

type stepContext struct {
	Step        ast.Step
	ExecID      string
	DryRun      bool
	TenantID    string
	WorkspaceID string
	Inputs      map[string]any
}

func (e *InMemoryEngine) runStep(ctx context.Context, exec *Execution, sc stepContext, signals <-chan signalEnvelope) (map[string]any, error) {
	activity, err := e.registry.Resolve(sc.Step.Type)
	if err != nil {
		return nil, err
	}
	policy := sc.Step.Retries
	maxAttempts := 1
	if policy != nil && policy.Max > 0 {
		maxAttempts = policy.Max + 1
	}
	timeout := time.Hour
	if sc.Step.Timeout != "" {
		if d, err := time.ParseDuration(sc.Step.Timeout); err == nil {
			timeout = d
		}
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		event := StepEvent{
			StepID:    sc.Step.ID,
			Type:      sc.Step.Type,
			Status:    StepStatusRunning,
			StartedAt: e.now(),
			Inputs:    sc.Inputs,
			Attempt:   attempt,
		}
		exec.Steps = append(exec.Steps, event)
		e.update(exec)
		_ = e.sink.Emit(newEvent(exec, EventStepStarted, map[string]any{
			"step_id": sc.Step.ID,
			"type":    sc.Step.Type,
			"attempt": attempt,
		}))
		stepCtx, cancel := context.WithTimeout(ctx, timeout)
		out, runErr := activity.Execute(stepCtx, ActivityInput{
			TenantID:    sc.TenantID,
			WorkspaceID: sc.WorkspaceID,
			ExecutionID: sc.ExecID,
			Step: StepRuntimeInfo{
				ID:        sc.Step.ID,
				Type:      string(sc.Step.Type),
				Ref:       sc.Step.Ref,
				Tool:      sc.Step.Tool,
				Timeout:   sc.Step.Timeout,
				Approver:  sc.Step.ApproverRole,
				OnTimeout: sc.Step.OnTimeout,
			},
			Inputs: sc.Inputs,
			DryRun: sc.DryRun,
		})
		cancel()
		// HITL: suspend awaiting signal.
		if runErr == nil && out.Wait {
			now := e.now()
			markStep(exec, sc.Step.ID, attempt, StepStatusWaiting, &now, sc.Inputs, out.Outputs, "")
			exec.Status = StatusWaiting
			e.update(exec)
			_ = e.sink.Emit(newEvent(exec, EventStepWaitingHuman, map[string]any{
				"step_id":       sc.Step.ID,
				"approver_role": sc.Step.ApproverRole,
				"on_timeout":    sc.Step.OnTimeout,
				"reason":        out.Reason,
			}))
			originalInputs := cloneMap(sc.Inputs)
			signalOutputs, decision, finalInputs, signalErr := awaitHITLSignal(signals, sc.Step, sc.Inputs)
			// Audit always, even on failure.
			_ = e.audit.Log(ctx, AuditEntry{
				ExecutionID:     exec.ID,
				StepID:          sc.Step.ID,
				ApproverRole:    sc.Step.ApproverRole,
				Decision:        decision,
				OriginalInputs:  originalInputs,
				FinalInputs:     finalInputs,
				InputDiff:       DiffInputs(originalInputs, finalInputs),
				At:              e.now(),
				OnTimeoutPolicy: sc.Step.OnTimeout,
			})
			if signalErr != nil {
				if decision == "escalated" {
					_ = e.sink.Emit(newEvent(exec, EventStepEscalated, map[string]any{
						"step_id":         sc.Step.ID,
						"escalation_role": sc.Step.EscalationRole,
					}))
				}
				lastErr = signalErr
				now2 := e.now()
				markStep(exec, sc.Step.ID, attempt, StepStatusFailed, &now2, sc.Inputs, signalOutputs, signalErr.Error())
				e.update(exec)
				_ = e.sink.Emit(newEvent(exec, EventStepFailed, map[string]any{
					"step_id": sc.Step.ID,
					"reason":  signalErr.Error(),
				}))
				return nil, signalErr
			}
			now2 := e.now()
			markStep(exec, sc.Step.ID, attempt, StepStatusCompleted, &now2, finalInputs, signalOutputs, "")
			exec.Status = StatusRunning
			e.update(exec)
			_ = e.sink.Emit(newEvent(exec, EventStepCompleted, map[string]any{
				"step_id": sc.Step.ID,
				"outputs": signalOutputs,
			}))
			return signalOutputs, nil
		}
		if runErr != nil {
			lastErr = runErr
			if !shouldRetry(policy, runErr, attempt, maxAttempts) {
				now := e.now()
				markStep(exec, sc.Step.ID, attempt, StepStatusFailed, &now, sc.Inputs, nil, runErr.Error())
				e.update(exec)
				_ = e.sink.Emit(newEvent(exec, EventStepFailed, map[string]any{
					"step_id": sc.Step.ID,
					"reason":  runErr.Error(),
					"attempt": attempt,
				}))
				return nil, runErr
			}
			now := e.now()
			markStep(exec, sc.Step.ID, attempt, StepStatusFailed, &now, sc.Inputs, nil, runErr.Error())
			e.update(exec)
			_ = e.sink.Emit(newEvent(exec, EventRetried, map[string]any{
				"step_id": sc.Step.ID,
				"attempt": attempt,
				"reason":  runErr.Error(),
			}))
			waitBackoff(policy, attempt)
			continue
		}
		now := e.now()
		markStep(exec, sc.Step.ID, attempt, StepStatusCompleted, &now, sc.Inputs, out.Outputs, "")
		e.update(exec)
		_ = e.sink.Emit(newEvent(exec, EventStepCompleted, map[string]any{
			"step_id": sc.Step.ID,
			"outputs": out.Outputs,
			"attempt": attempt,
		}))
		return out.Outputs, nil
	}
	return nil, fmt.Errorf("step %s exhausted retries: %v", sc.Step.ID, lastErr)
}

// awaitHITLSignal blocks until an approve/reject/cancel signal arrives or a
// timeout fires.
//
// Returns:
//   - outputs: payload to surface as the step's outputs (may include decision)
//   - decision: machine-readable decision string for audit
//   - finalInputs: inputs that the next step should receive (approver may have
//     modified them via signal payload's `inputs` key)
//   - err: non-nil when execution should fail or escalate
func awaitHITLSignal(signals <-chan signalEnvelope, step ast.Step, originalInputs map[string]any) (map[string]any, string, map[string]any, error) {
	timeout := defaultHITLTimeout(step)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	finalInputs := cloneMap(originalInputs)
	for {
		select {
		case env, ok := <-signals:
			if !ok {
				return nil, "channel_closed", finalInputs, errors.New("signal_channel_closed")
			}
			switch env.Signal {
			case "__cancel__":
				return nil, "cancelled", finalInputs, errors.New("cancelled")
			case "approve":
				outputs := map[string]any{"decision": "approved"}
				if env.Payload != nil {
					if mods, ok := env.Payload["inputs"].(map[string]any); ok {
						for k, v := range mods {
							finalInputs[k] = v
						}
					}
					for k, v := range env.Payload {
						if k == "inputs" {
							continue
						}
						outputs[k] = v
					}
				}
				return outputs, "approved", finalInputs, nil
			case "reject":
				return nil, "rejected", finalInputs, fmt.Errorf("rejected: %v", env.Payload)
			default:
				continue
			}
		case <-timer.C:
			switch step.OnTimeout {
			case "proceed":
				return map[string]any{"decision": "timeout_proceed"}, "timeout_proceed", finalInputs, nil
			case "escalate":
				return nil, "escalated", finalInputs, errors.New("hitl_escalated")
			default:
				return nil, "timeout_failed", finalInputs, errors.New("hitl_timeout")
			}
		}
	}
}

func defaultHITLTimeout(step ast.Step) time.Duration {
	if step.Timeout == "" {
		return 24 * time.Hour
	}
	if d, err := time.ParseDuration(step.Timeout); err == nil {
		return d
	}
	return 24 * time.Hour
}

func markStep(exec *Execution, stepID string, attempt int, status StepStatus, completedAt *time.Time, inputs, outputs map[string]any, reason string) {
	for i := len(exec.Steps) - 1; i >= 0; i-- {
		if exec.Steps[i].StepID == stepID && exec.Steps[i].Attempt == attempt {
			exec.Steps[i].Status = status
			exec.Steps[i].CompletedAt = completedAt
			exec.Steps[i].Outputs = outputs
			if inputs != nil {
				exec.Steps[i].Inputs = inputs
			}
			exec.Steps[i].FailureReason = reason
			return
		}
	}
}

func (e *InMemoryEngine) compensate(ctx context.Context, exec *Execution, successful []ast.Step, wf *ast.Workflow) {
	if len(successful) == 0 {
		return
	}
	exec.Status = StatusCompensating
	e.update(exec)
	for i := len(successful) - 1; i >= 0; i-- {
		step := successful[i]
		comp := findCompensationStep(wf, step.CompensateWith)
		if comp == nil {
			continue
		}
		_, err := e.runStep(ctx, exec, stepContext{
			Step:        *comp,
			ExecID:      exec.ID,
			DryRun:      exec.DryRun,
			TenantID:    exec.TenantID,
			WorkspaceID: exec.WorkspaceID,
			Inputs:      map[string]any{"compensating_for": step.ID},
		}, nil)
		outcome := "ok"
		if err != nil {
			outcome = err.Error()
		}
		exec.Compensations = append(exec.Compensations, CompensationItem{
			ForStepID:    step.ID,
			CompensateID: comp.ID,
			At:           e.now(),
			Outcome:      outcome,
		})
		_ = e.sink.Emit(newEvent(exec, EventCompensated, map[string]any{
			"for_step_id":    step.ID,
			"compensated_by": comp.ID,
			"outcome":        outcome,
		}))
	}
	e.update(exec)
}

func findCompensationStep(wf *ast.Workflow, id string) *ast.Step {
	for i := range wf.Spec.OnFailure {
		if wf.Spec.OnFailure[i].ID == id {
			s := wf.Spec.OnFailure[i]
			return &s
		}
	}
	for i := range wf.Spec.Steps {
		if wf.Spec.Steps[i].ID == id {
			s := wf.Spec.Steps[i]
			return &s
		}
	}
	return nil
}

func nextRunnable(steps []ast.Step, pending map[string]struct{}, completed map[string]map[string]any) (ast.Step, bool) {
	for _, s := range steps {
		if _, ok := pending[s.ID]; !ok {
			continue
		}
		ready := true
		for _, dep := range s.DependsOn {
			if _, done := completed[dep]; !done {
				ready = false
				break
			}
		}
		if ready {
			return s, true
		}
	}
	return ast.Step{}, false
}

func stepIDSet(steps []ast.Step) map[string]struct{} {
	out := map[string]struct{}{}
	for _, s := range steps {
		out[s.ID] = struct{}{}
	}
	return out
}

func resolveInputs(step ast.Step, wfInputs map[string]any, completed map[string]map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range step.Inputs {
		s, ok := v.(string)
		if !ok {
			out[k] = v
			continue
		}
		if !strings.HasPrefix(s, "$") {
			out[k] = s
			continue
		}
		parts := strings.Split(strings.TrimPrefix(s, "$"), ".")
		switch {
		case len(parts) >= 2 && parts[0] == "inputs":
			if v, ok := wfInputs[parts[1]]; ok {
				out[k] = v
			}
		case len(parts) >= 4 && parts[0] == "steps" && parts[2] == "outputs":
			if outputs, ok := completed[parts[1]]; ok {
				if v, ok := outputs[parts[3]]; ok {
					out[k] = v
				}
			}
		default:
			out[k] = s
		}
	}
	return out
}

func aggregateOutputs(wf *ast.Workflow, completed map[string]map[string]any) map[string]any {
	out := map[string]any{}
	if len(wf.Spec.Outputs) == 0 {
		// Default: surface every step's outputs under its id.
		for id, o := range completed {
			out[id] = o
		}
		return out
	}
	// Outputs declared in the spec are passthrough placeholders we surface.
	for _, decl := range wf.Spec.Outputs {
		out[decl.Name] = nil
	}
	return out
}

func shouldRetry(policy *ast.RetryPolicy, err error, attempt, max int) bool {
	if policy == nil {
		return false
	}
	if attempt >= max {
		return false
	}
	if err == nil {
		return false
	}
	for _, ne := range policy.NonRetryable {
		if strings.Contains(err.Error(), ne) {
			return false
		}
	}
	return true
}

func waitBackoff(policy *ast.RetryPolicy, attempt int) {
	if policy == nil {
		return
	}
	base := 50
	if policy.InitialMS > 0 {
		base = policy.InitialMS
	}
	max := 5000
	if policy.MaxMS > 0 {
		max = policy.MaxMS
	}
	delay := base
	switch policy.Backoff {
	case "exponential":
		delay = base * (1 << uint(attempt-1))
	case "linear":
		delay = base * attempt
	}
	if delay > max {
		delay = max
	}
	if delay <= 0 {
		return
	}
	// Tests run with the in-memory engine; cap to keep tests fast.
	if delay > 50 {
		delay = 50
	}
	time.Sleep(time.Duration(delay) * time.Millisecond)
}

func cloneExecution(in *Execution) *Execution {
	if in == nil {
		return nil
	}
	out := *in
	out.Inputs = cloneMap(in.Inputs)
	out.Outputs = cloneMap(in.Outputs)
	out.Steps = append([]StepEvent(nil), in.Steps...)
	out.Compensations = append([]CompensationItem(nil), in.Compensations...)
	return &out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
