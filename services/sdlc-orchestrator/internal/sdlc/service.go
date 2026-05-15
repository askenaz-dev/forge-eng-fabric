package sdlc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInitiativeNotFound = errors.New("initiative_not_found")
	ErrInvalidPhase       = errors.New("invalid_phase")
	ErrPhaseMismatch      = errors.New("phase_mismatch")
	ErrMissingOpenSpec    = errors.New("openspec_root_required")
	ErrMissingWorkspace   = errors.New("workspace_id_required")
	ErrInvalidOverride    = errors.New("invalid_phase_progression_bypass")
)

type Service struct {
	Store         *Store
	Sink          Sink
	GateEvaluator GateEvaluator
	Now           func() time.Time
}

func NewService(store *Store, sink Sink) *Service {
	if sink == nil {
		sink = &MemorySink{}
	}
	now := func() time.Time { return time.Now().UTC() }
	return &Service{Store: store, Sink: sink, GateEvaluator: EvidenceGateEvaluator{Now: now}, Now: now}
}

func (s *Service) CreateInitiative(_ context.Context, req CreateInitiativeRequest) (*Initiative, error) {
	if req.WorkspaceID == "" {
		return nil, ErrMissingWorkspace
	}
	if req.OpenSpecRoot == "" {
		return nil, ErrMissingOpenSpec
	}
	criticality := strings.ToLower(req.Criticality)
	if criticality == "" {
		criticality = "medium"
	}

	// Build effective targets: start from app-level defaults, apply spec override.
	appTargets := req.Targets
	if appTargets == nil {
		appTargets = DefaultTargets()
	}
	var targets map[Phase]TargetPolicy
	if req.TargetsOverride != nil {
		var err error
		targets, err = MergeTargets(appTargets, req.TargetsOverride)
		if err != nil {
			return nil, err
		}
	} else {
		targets = appTargets
	}

	// Promote opt-in phases that are explicitly included.
	for _, p := range req.Include {
		if targets[p] == TargetOptIn {
			targets[p] = TargetRequired
		}
	}

	now := s.Now()
	initiative := &Initiative{
		WorkspaceID:  req.WorkspaceID,
		OpenSpecRoot: req.OpenSpecRoot,
		JiraEpicKey:  req.JiraEpicKey,
		Criticality:  criticality,
		CurrentPhase: PhaseProduct,
		Targets:      targets,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	initiative.PhaseStates = make([]PhaseState, 0, len(OrderedPhases))
	ec := fromCreate(req)
	for _, phase := range OrderedPhases {
		policy := targets[phase]
		status := StatusNotStarted
		var enteredAt *time.Time
		switch {
		case phase == PhaseProduct:
			status = StatusInProgress
			entered := now
			enteredAt = &entered
		case policy == TargetSkipped:
			status = StatusSkipped
		case policy == TargetOptIn:
			// Opt-in phases not included remain skipped in plan.
			status = StatusSkipped
		}
		initiative.PhaseStates = append(initiative.PhaseStates, PhaseState{
			Phase:     phase,
			Status:    status,
			EnteredAt: enteredAt,
		})
	}
	initiative = s.Store.Insert(initiative)
	for i := range initiative.PhaseStates {
		initiative.PhaseStates[i].InitiativeID = initiative.ID
	}
	initiative = s.Store.Update(initiative)
	_ = s.Sink.Emit(newEvent(initiative, ec, EventPhaseEntered, map[string]any{
		"initiative_id": initiative.ID,
		"phase":         PhaseProduct,
		"openspec_root": initiative.OpenSpecRoot,
	}))
	// Emit skipped events for all phases that won't run.
	for _, phase := range OrderedPhases {
		if phase == PhaseProduct {
			continue
		}
		policy := targets[phase]
		if policy == TargetSkipped || policy == TargetOptIn {
			_ = s.Sink.Emit(newEvent(initiative, ec, EventPhaseSkipped, map[string]any{
				"initiative_id": initiative.ID,
				"phase":         phase,
				"target":        string(policy),
			}))
		}
	}
	return initiative, nil
}

func (s *Service) GetInitiative(id string) (*Initiative, error) {
	initiative, ok := s.Store.Get(id)
	if !ok {
		return nil, ErrInitiativeNotFound
	}
	return initiative, nil
}

func (s *Service) ListInitiatives(workspaceID string) []*Initiative {
	return s.Store.List(workspaceID)
}

func (s *Service) CompletePhase(ctx context.Context, id string, phase Phase, req CompletePhaseRequest) (*Initiative, error) {
	if PhaseIndex(phase) < 0 {
		return nil, ErrInvalidPhase
	}
	initiative, ok := s.Store.Get(id)
	if !ok {
		return nil, ErrInitiativeNotFound
	}

	if PhaseIndex(phase) < PhaseIndex(initiative.CurrentPhase) {
		// Duplicate or stale completion call: return current state without side effects.
		return initiative, nil
	}
	if initiative.CurrentPhase != phase {
		return nil, fmt.Errorf("%w: current_phase=%s requested=%s", ErrPhaseMismatch, initiative.CurrentPhase, phase)
	}

	state := phaseState(initiative, phase)
	if state == nil {
		return nil, ErrInvalidPhase
	}
	state.Status = StatusGatePending
	results, err := s.GateEvaluator.Evaluate(ctx, initiative, phase, req.Evidence)
	if err != nil {
		return nil, err
	}
	state.Gates = append(state.Gates, results...)
	for _, result := range results {
		_ = s.Sink.Emit(newEvent(initiative, fromComplete(req), EventGateEvaluated, map[string]any{
			"initiative_id": initiative.ID,
			"phase":         phase,
			"gate":          result.Gate,
			"outcome":       result.Outcome,
			"reason":        result.Reason,
		}))
	}

	failed := failedGates(results)
	overridden, overrideErr := validateOverride(req.Override)
	ec := fromComplete(req)

	// Determine the effective target policy for this phase.
	policy := TargetRequired
	if initiative.Targets != nil {
		if p, ok := initiative.Targets[phase]; ok {
			policy = p
		}
	}

	if len(failed) > 0 && !overridden {
		if overrideErr != nil {
			return nil, overrideErr
		}
		if policy == TargetOptional {
			// Optional: warn and continue rather than block.
			_ = s.Sink.Emit(newEvent(initiative, ec, EventPhaseWarning, map[string]any{
				"initiative_id": initiative.ID,
				"phase":         phase,
				"failed_gates":  gateNames(failed),
				"target":        string(policy),
			}))
		} else {
			now := s.Now()
			state.Status = StatusBlocked
			for _, result := range failed {
				state.Blockers = append(state.Blockers, Blocker{
					ID:           newID(),
					InitiativeID: initiative.ID,
					Phase:        phase,
					Gate:         result.Gate,
					Reason:       result.Reason,
					CreatedAt:    now,
				})
			}
			initiative = s.Store.Update(initiative)
			_ = s.Sink.Emit(newEvent(initiative, ec, EventPhaseBlocked, map[string]any{
				"initiative_id": initiative.ID,
				"phase":         phase,
				"failed_gates":  gateNames(failed),
				"target":        string(policy),
			}))
			return initiative, nil
		}
	}

	now := s.Now()
	completed := now
	state.CompletedAt = &completed
	state.Status = StatusPassed
	if overridden {
		state.Status = StatusOverridden
		_ = s.Sink.Emit(newEvent(initiative, fromComplete(req), EventOverrideConsumed, map[string]any{
			"initiative_id": initiative.ID,
			"phase":         phase,
			"override":      "phase-progression-bypass",
			"override_id":   req.Override.ID,
			"reason":        req.Override.Reason,
		}))
	}

	next, ok := NextPhase(phase)
	if !ok {
		return nil, ErrInvalidPhase
	}
	from := initiative.CurrentPhase

	// Advance past any phases that are skipped in the targets plan.
	for next != PhaseDone {
		if initiative.Targets == nil {
			break
		}
		p, exists := initiative.Targets[next]
		if !exists || (p != TargetSkipped && p != TargetOptIn) {
			break
		}
		// Emit skipped event for this auto-skipped phase.
		_ = s.Sink.Emit(newEvent(initiative, ec, EventPhaseSkipped, map[string]any{
			"initiative_id": initiative.ID,
			"phase":         next,
			"target":        string(p),
		}))
		next, ok = NextPhase(next)
		if !ok {
			break
		}
	}

	initiative.CurrentPhase = next
	if next != PhaseDone {
		nextState := phaseState(initiative, next)
		if nextState != nil {
			nextState.Status = StatusInProgress
			entered := now
			nextState.EnteredAt = &entered
		}
	}
	initiative = s.Store.Update(initiative)
	_ = s.Sink.Emit(newEvent(initiative, ec, EventPhaseProgressed, map[string]any{
		"initiative_id": initiative.ID,
		"from":          from,
		"to":            next,
		"override":      overridden,
	}))
	if next != PhaseDone {
		_ = s.Sink.Emit(newEvent(initiative, ec, EventPhaseEntered, map[string]any{
			"initiative_id": initiative.ID,
			"phase":         next,
		}))
	}
	return initiative, nil
}

func (s *Service) Regress(id string, to Phase, req CompletePhaseRequest) (*Initiative, error) {
	if PhaseIndex(to) < 0 {
		return nil, ErrInvalidPhase
	}
	initiative, ok := s.Store.Get(id)
	if !ok {
		return nil, ErrInitiativeNotFound
	}
	from := initiative.CurrentPhase
	initiative.CurrentPhase = to
	if state := phaseState(initiative, to); state != nil {
		state.Status = StatusInProgress
		entered := s.Now()
		state.EnteredAt = &entered
	}
	initiative = s.Store.Update(initiative)
	_ = s.Sink.Emit(newEvent(initiative, fromComplete(req), EventPhaseRegressed, map[string]any{
		"initiative_id": initiative.ID,
		"from":          from,
		"to":            to,
	}))
	return initiative, nil
}

func (s *Service) HandleEvent(ctx context.Context, event BusEvent) (*Initiative, error) {
	initiativeID := stringFrom(event.Data["initiative_id"])
	if initiativeID == "" {
		return nil, errors.New("initiative_id_required")
	}
	phase := Phase(stringFrom(event.Data["phase"]))
	if phase == "" {
		initiative, err := s.GetInitiative(initiativeID)
		if err != nil {
			return nil, err
		}
		phase = initiative.CurrentPhase
	}
	if event.Type == "sdlc.phase.regress_requested.v1" {
		to := Phase(stringFrom(event.Data["to_phase"]))
		return s.Regress(initiativeID, to, CompletePhaseRequest{Actor: event.Actor, TenantID: event.TenantID, CorrelationID: event.CorrelationID})
	}
	return s.CompletePhase(ctx, initiativeID, phase, CompletePhaseRequest{
		Actor:         event.Actor,
		TenantID:      event.TenantID,
		CorrelationID: event.CorrelationID,
		Evidence:      mapFrom(event.Data["evidence"]),
	})
}

func failedGates(results []GateResult) []GateResult {
	out := []GateResult{}
	for _, result := range results {
		if result.Outcome == GateFailed {
			out = append(out, result)
		}
	}
	return out
}

func gateNames(results []GateResult) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, result.Gate)
	}
	return out
}

func validateOverride(in *OverrideInput) (bool, error) {
	if in == nil || !in.Approved {
		return false, nil
	}
	if in.ApproverRole != "release-manager" || in.Reason == "" || in.TTLSeconds <= 0 || in.TTLSeconds > 86400 {
		return false, ErrInvalidOverride
	}
	return true, nil
}

func fromCreate(req CreateInitiativeRequest) eventContext {
	return eventContext{tenantID: req.TenantID, actor: valueOr(req.Actor, "alfred"), correlationID: req.CorrelationID}
}

func fromComplete(req CompletePhaseRequest) eventContext {
	return eventContext{tenantID: req.TenantID, actor: valueOr(req.Actor, "alfred"), correlationID: req.CorrelationID}
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func stringFrom(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func mapFrom(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
