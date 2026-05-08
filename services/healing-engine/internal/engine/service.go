package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// WorkflowExecutor invokes the underlying Phase-5 workflow for a healing action.
// Production wires this to the workflow-runtime; tests use stub.
type WorkflowExecutor interface {
	Execute(ctx context.Context, ref string, params map[string]any) (runID string, err error)
	Verify(ctx context.Context, runID string) (ok bool, err error)
	Rollback(ctx context.Context, ref string, params map[string]any) (runID string, err error)
}

// ApprovalsClient creates approval inbox entries and waits for the signal.
type ApprovalsClient interface {
	Create(ctx context.Context, payload map[string]any) (approvalID string, err error)
	Wait(ctx context.Context, approvalID string, ttl time.Duration) (approved bool, err error)
}

// Service wires the store, sink, executor and approvals client.
type Service struct {
	Store     *Store
	Sink      Sink
	Workflows WorkflowExecutor
	Approvals ApprovalsClient
	Now       func() time.Time
	ApprovalTTL time.Duration
}

// NewService creates a Service with sensible defaults.
func NewService(store *Store, sink Sink) *Service {
	if sink == nil {
		sink = &MemorySink{}
	}
	return &Service{
		Store:       store,
		Sink:        sink,
		Workflows:   &noopExecutor{},
		Approvals:   &autoApprover{decision: false},
		Now:         func() time.Time { return time.Now().UTC() },
		ApprovalTTL: 30 * time.Minute,
	}
}

// Trigger runs the engine for an incident. It picks the first matching action
// from the incident's suggested list (typed by the diagnosis pipeline) that
// the engine knows about, consults the matching envelope, applies kill-switch
// + rate limits, and runs the appropriate level path.
func (s *Service) Trigger(ctx context.Context, in IncidentInput) (*HealingDecision, error) {
	if in.IncidentID == "" {
		return nil, fmt.Errorf("incident_id required")
	}
	action := s.pickAction(in.SuggestedActions)
	if action == nil {
		return nil, ErrActionNotFound
	}
	envelope := s.Store.GetEnvelope(in.Capability, in.Environment, in.Criticality)
	if envelope == nil {
		return nil, ErrEnvelopeNotFound
	}

	decision := &HealingDecision{
		ID:         "hd-" + uuid.NewString(),
		IncidentID: in.IncidentID,
		ActionID:   action.ID,
		Synthetic:  in.Synthetic,
		CreatedAt:  s.Now(),
	}

	requested := envelope.DefaultLevel
	if requested == "" {
		requested = LevelL2
	}
	decision.RequestedLevel = requested
	decision.Level = requested

	// Kill switch wins over everything: degrade to L1.
	if s.Store.KillSwitch(in.WorkspaceID) {
		decision.Level = LevelL1
		decision.Outcome = OutcomeSuppressed
		decision.Reason = "kill_switch"
		s.emitTriggered(in, decision, envelope, true)
		s.emitLevelDecided(in, decision, "kill_switch")
		s.Store.SaveDecision(decision)
		return decision, nil
	}

	// Envelope cap.
	if !levelAllowed(envelope.AllowedLevels, decision.Level) {
		decision.Level = highestAllowed(envelope.AllowedLevels)
		decision.Reason = "envelope_cap"
		decision.Outcome = OutcomeDegradedByEnv
	}

	// Action allowed_levels_by_env intersection.
	if action.AllowedLevelsByEnv != nil {
		envAllowed := action.AllowedLevelsByEnv[in.Environment]
		if len(envAllowed) > 0 && !levelAllowed(envAllowed, decision.Level) {
			decision.Level = highestAllowed(envAllowed)
			if decision.Reason == "" {
				decision.Reason = "action_env_cap"
			}
			decision.Outcome = OutcomeDegradedByEnv
		}
	}

	// Rate limit per envelope.
	if envelope.MaxActionsPerHour > 0 {
		count := s.Store.trackInvocation(envelope.ID, s.Now())
		if count > envelope.MaxActionsPerHour {
			decision.Outcome = OutcomeBlockedByLimits
			decision.Reason = "rate_limit"
			s.emitTriggered(in, decision, envelope, false)
			s.emitLevelDecided(in, decision, "rate_limit")
			s.Store.SaveDecision(decision)
			return decision, nil
		}
	}

	s.emitTriggered(in, decision, envelope, false)
	s.emitLevelDecided(in, decision, decision.Reason)

	switch decision.Level {
	case LevelL1:
		decision.Outcome = OutcomeNotified
	case LevelL2:
		decision.Outcome = OutcomeSuggested
	case LevelL3:
		s.runL3(ctx, in, action, decision)
	case LevelL4:
		s.runL4(ctx, in, action, decision)
	case LevelL5:
		s.runL5(ctx, in, action, decision)
	default:
		decision.Outcome = OutcomeNotified
	}

	s.Store.SaveDecision(decision)
	return decision, nil
}

func (s *Service) pickAction(suggested []string) *Action {
	for _, id := range suggested {
		if a := s.Store.GetAction(id); a != nil {
			return a
		}
	}
	return nil
}

func (s *Service) runL3(ctx context.Context, in IncidentInput, action *Action, d *HealingDecision) {
	if in.Synthetic {
		// Synthetic flow: short-circuit to the executed path so e2e tests exercise the rest.
		d.Outcome = OutcomeWaitingApproval
		d.ApprovalID = "approval-synthetic-" + uuid.NewString()
		s.emitExecuted(in, d, "waiting_approval")
		return
	}
	approvalID, err := s.Approvals.Create(ctx, map[string]any{
		"incident_id": in.IncidentID,
		"action":      action.ID,
		"workflow":    action.WorkflowRef,
	})
	if err != nil {
		d.Outcome = OutcomeFailed
		d.Reason = "approvals_unreachable: " + err.Error()
		return
	}
	d.ApprovalID = approvalID
	d.Outcome = OutcomeWaitingApproval
	approved, err := s.Approvals.Wait(ctx, approvalID, s.ApprovalTTL)
	if err != nil {
		d.Outcome = OutcomeFailed
		d.Reason = "approval_wait_failed: " + err.Error()
		return
	}
	if !approved {
		d.Outcome = OutcomeFailed
		d.Reason = "approval_denied_or_timeout"
		return
	}
	runID, err := s.Workflows.Execute(ctx, action.WorkflowRef, action.Parameters)
	if err != nil {
		d.Outcome = OutcomeFailed
		d.Reason = "workflow_execute_failed: " + err.Error()
		s.emitExecuted(in, d, "failed")
		return
	}
	d.WorkflowRunID = runID
	d.Outcome = OutcomeExecuted
	s.emitExecuted(in, d, "executed")
}

func (s *Service) runL4(ctx context.Context, in IncidentInput, action *Action, d *HealingDecision) {
	runID, err := s.Workflows.Execute(ctx, action.WorkflowRef, action.Parameters)
	if err != nil {
		d.Outcome = OutcomeFailed
		d.Reason = "workflow_execute_failed: " + err.Error()
		s.emitExecuted(in, d, "failed")
		return
	}
	d.WorkflowRunID = runID
	d.Outcome = OutcomeExecuted
	s.emitExecuted(in, d, "executed")
}

func (s *Service) runL5(ctx context.Context, in IncidentInput, action *Action, d *HealingDecision) {
	if !action.Reversible {
		d.Outcome = OutcomeFailed
		d.Reason = "non_reversible_action_at_L5"
		return
	}
	runID, err := s.Workflows.Execute(ctx, action.WorkflowRef, action.Parameters)
	if err != nil {
		d.Outcome = OutcomeFailed
		d.Reason = "workflow_execute_failed: " + err.Error()
		s.emitExecuted(in, d, "failed")
		return
	}
	d.WorkflowRunID = runID
	ok, err := s.Workflows.Verify(ctx, runID)
	if err == nil && ok {
		d.Outcome = OutcomeExecuted
		s.emitExecuted(in, d, "executed")
		return
	}
	// Verify failed — auto-rollback.
	rollbackID, rbErr := s.Workflows.Rollback(ctx, action.WorkflowRef, action.Parameters)
	d.Outcome = OutcomeRolledBack
	d.Reason = "verify_failed_auto_rollback"
	if rbErr != nil {
		d.Outcome = OutcomeFailed
		d.Reason = "rollback_failed: " + rbErr.Error()
	} else {
		d.WorkflowRunID = runID + "+rb=" + rollbackID
	}
	s.emitExecuted(in, d, "failed")
	s.emitRolledBack(in, d)
	s.emitEscalated(in, d)
}

// PromoteAction implements D6.10 — strict prerequisite gating.
func (s *Service) PromoteAction(req PromotionRequest) error {
	if req.TargetLevel != LevelL4 && req.TargetLevel != LevelL5 {
		return ErrInvalidLevel
	}
	a := s.Store.GetAction(req.ActionID)
	if a == nil {
		return ErrActionNotFound
	}
	if !req.PlatformAdminOK || !req.SecurityOK {
		return ErrPromotionApproval
	}
	stats := s.Store.GetPromotionStats(req.ActionID, req.Environment)
	if stats.EvalPassRateLast50 < 0.95 {
		return fmt.Errorf("%w: eval_pass_rate=%.2f", ErrPromotionPrerequisites, stats.EvalPassRateLast50)
	}
	if stats.SuccessfulL3Runs < 20 {
		return fmt.Errorf("%w: successful_l3_runs=%d", ErrPromotionPrerequisites, stats.SuccessfulL3Runs)
	}
	if stats.DaysSinceLastPostmortem < 30 {
		return fmt.Errorf("%w: days_since_postmortem=%d", ErrPromotionPrerequisites, stats.DaysSinceLastPostmortem)
	}
	current := a.AllowedLevelsByEnv[req.Environment]
	if !levelAllowed(current, req.TargetLevel) {
		current = appendLevel(current, req.TargetLevel)
		if a.AllowedLevelsByEnv == nil {
			a.AllowedLevelsByEnv = map[string][]Level{}
		}
		a.AllowedLevelsByEnv[req.Environment] = current
	}
	s.Store.SetAction(a)
	_ = s.Sink.Emit(newEvent("", "", "healing.action.promoted.v1",
		"action/"+a.ID, map[string]any{
			"action_id":    a.ID,
			"environment":  req.Environment,
			"target_level": req.TargetLevel,
			"requested_by": req.RequestedBy,
		}))
	return nil
}

// --- helpers ---

func levelAllowed(allowed []Level, want Level) bool {
	for _, l := range allowed {
		if l == want {
			return true
		}
	}
	return false
}

func appendLevel(existing []Level, l Level) []Level {
	if levelAllowed(existing, l) {
		return existing
	}
	return append(existing, l)
}

func highestAllowed(allowed []Level) Level {
	rank := map[Level]int{LevelL1: 1, LevelL2: 2, LevelL3: 3, LevelL4: 4, LevelL5: 5}
	best := LevelL1
	for _, l := range allowed {
		if rank[l] > rank[best] {
			best = l
		}
	}
	return best
}

func (s *Service) emitTriggered(in IncidentInput, d *HealingDecision, env *Envelope, suppressed bool) {
	_ = s.Sink.Emit(newEvent(in.TenantID, in.WorkspaceID, "healing.triggered.v1",
		"incident/"+in.IncidentID, map[string]any{
			"incident_id":   in.IncidentID,
			"action_id":     d.ActionID,
			"envelope_id":   env.ID,
			"level":         d.Level,
			"suppressed":    suppressed,
			"synthetic":     in.Synthetic,
		}))
}

func (s *Service) emitLevelDecided(in IncidentInput, d *HealingDecision, reason string) {
	_ = s.Sink.Emit(newEvent(in.TenantID, in.WorkspaceID, "healing.level_decided.v1",
		"incident/"+in.IncidentID, map[string]any{
			"incident_id": in.IncidentID,
			"requested":   d.RequestedLevel,
			"applied":     d.Level,
			"reason":      reason,
			"synthetic":   in.Synthetic,
		}))
}

func (s *Service) emitExecuted(in IncidentInput, d *HealingDecision, outcome string) {
	_ = s.Sink.Emit(newEvent(in.TenantID, in.WorkspaceID, "healing.executed.v1",
		"incident/"+in.IncidentID, map[string]any{
			"incident_id":   in.IncidentID,
			"action_id":     d.ActionID,
			"workflow_run":  d.WorkflowRunID,
			"outcome":       outcome,
			"approval_id":   d.ApprovalID,
			"synthetic":     in.Synthetic,
		}))
}

func (s *Service) emitRolledBack(in IncidentInput, d *HealingDecision) {
	_ = s.Sink.Emit(newEvent(in.TenantID, in.WorkspaceID, "healing.rolled_back.v1",
		"incident/"+in.IncidentID, map[string]any{
			"incident_id":  in.IncidentID,
			"action_id":    d.ActionID,
			"workflow_run": d.WorkflowRunID,
			"reason":       d.Reason,
			"synthetic":    in.Synthetic,
		}))
}

func (s *Service) emitEscalated(in IncidentInput, d *HealingDecision) {
	_ = s.Sink.Emit(newEvent(in.TenantID, in.WorkspaceID, "healing.escalated.v1",
		"incident/"+in.IncidentID, map[string]any{
			"incident_id": in.IncidentID,
			"action_id":   d.ActionID,
			"reason":      d.Reason,
			"synthetic":   in.Synthetic,
		}))
}

// --- defaults ---

type noopExecutor struct{}

func (n *noopExecutor) Execute(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "wfr-" + uuid.NewString(), nil
}
func (n *noopExecutor) Verify(_ context.Context, _ string) (bool, error)              { return true, nil }
func (n *noopExecutor) Rollback(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "wfr-rb-" + uuid.NewString(), nil
}

type autoApprover struct{ decision bool }

func (a *autoApprover) Create(_ context.Context, _ map[string]any) (string, error) {
	return "approval-" + uuid.NewString(), nil
}
func (a *autoApprover) Wait(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return a.decision, nil
}
