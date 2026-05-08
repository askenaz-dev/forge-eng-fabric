package engine

import (
	"context"
	"errors"
	"testing"
	"time"
)

func setupAction(s *Store) *Action {
	a := &Action{
		ID:          "restart-pod",
		Risk:        "low",
		Reversible:  true,
		BlastRadius: "pod",
		AllowedLevelsByEnv: map[string][]Level{
			"prod":  {LevelL1, LevelL2, LevelL3},
			"stage": {LevelL1, LevelL2, LevelL3, LevelL4},
		},
		WorkflowRef: "registry:workflow/sre-platform/wf-restart-pod@1.0.0",
		EvalSuiteID: "ds-restart-pod-eval@1.0.0",
	}
	s.SetAction(a)
	return a
}

func setupEnvelope(s *Store, env, criticality string, defaultLevel Level, allowed []Level) *Envelope {
	e := &Envelope{
		Capability:    "sdlc-devops",
		Environment:   env,
		Criticality:   criticality,
		DefaultLevel:  defaultLevel,
		AllowedLevels: allowed,
	}
	s.SetEnvelope(e)
	return e
}

func TestL3PathRequiresApproval(t *testing.T) {
	store := NewStore()
	setupAction(store)
	setupEnvelope(store, "stage", "high", LevelL3, []Level{LevelL1, LevelL2, LevelL3})
	svc := NewService(store, &MemorySink{})
	// Approver auto-rejects, so the run should report approval denied.
	d, err := svc.Trigger(context.Background(), IncidentInput{
		IncidentID: "inc-1", Service: "svc-a", Environment: "stage", Capability: "sdlc-devops",
		Criticality: "high", SuggestedActions: []string{"restart-pod"},
	})
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if d.Level != LevelL3 || d.Outcome != OutcomeFailed {
		t.Fatalf("expected L3 failed (approval denied), got %+v", d)
	}
}

func TestL3WithApprovalExecutes(t *testing.T) {
	store := NewStore()
	setupAction(store)
	setupEnvelope(store, "stage", "high", LevelL3, []Level{LevelL1, LevelL2, LevelL3})
	svc := NewService(store, &MemorySink{})
	svc.Approvals = &autoApprover{decision: true}
	d, err := svc.Trigger(context.Background(), IncidentInput{
		IncidentID: "inc-1", Service: "svc-a", Environment: "stage", Capability: "sdlc-devops",
		Criticality: "high", SuggestedActions: []string{"restart-pod"},
	})
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if d.Outcome != OutcomeExecuted {
		t.Fatalf("expected executed, got %s reason=%s", d.Outcome, d.Reason)
	}
	if d.WorkflowRunID == "" {
		t.Fatal("expected workflow run id")
	}
}

func TestEnvelopeCapDegradesLevel(t *testing.T) {
	store := NewStore()
	setupAction(store)
	// Default L4 but envelope only allows up to L3.
	setupEnvelope(store, "prod", "high", LevelL4, []Level{LevelL1, LevelL2, LevelL3})
	svc := NewService(store, &MemorySink{})
	svc.Approvals = &autoApprover{decision: true}
	d, _ := svc.Trigger(context.Background(), IncidentInput{
		IncidentID: "inc-2", Service: "svc-a", Environment: "prod", Capability: "sdlc-devops",
		Criticality: "high", SuggestedActions: []string{"restart-pod"},
	})
	if d.Level != LevelL3 || d.RequestedLevel != LevelL4 {
		t.Fatalf("expected degrade to L3, got %s requested=%s", d.Level, d.RequestedLevel)
	}
	sink := svc.Sink.(*MemorySink)
	foundDecision := false
	for _, e := range sink.Events {
		if e.Type == "healing.level_decided.v1" {
			if reason, _ := e.Data["reason"].(string); reason == "envelope_cap" {
				foundDecision = true
			}
		}
	}
	if !foundDecision {
		t.Fatal("expected level_decided event with envelope_cap reason")
	}
}

func TestKillSwitchDegradesToL1(t *testing.T) {
	store := NewStore()
	setupAction(store)
	setupEnvelope(store, "prod", "critical", LevelL3, AllLevels)
	store.SetKillSwitch("", true)
	svc := NewService(store, &MemorySink{})
	d, _ := svc.Trigger(context.Background(), IncidentInput{
		IncidentID: "inc-3", Service: "svc-a", Environment: "prod", Capability: "sdlc-devops",
		Criticality: "critical", SuggestedActions: []string{"restart-pod"},
	})
	if d.Level != LevelL1 || d.Outcome != OutcomeSuppressed {
		t.Fatalf("expected suppressed L1, got %+v", d)
	}
}

func TestL5AutoRollback(t *testing.T) {
	store := NewStore()
	store.SetAction(&Action{
		ID: "refresh-cache", Reversible: true, BlastRadius: "cache",
		AllowedLevelsByEnv: map[string][]Level{"stage": AllLevels},
		WorkflowRef:        "registry:workflow/sre-platform/wf-refresh-cache@1.0.0",
	})
	setupEnvelope(store, "stage", "low", LevelL5, AllLevels)
	svc := NewService(store, &MemorySink{})
	svc.Workflows = &flakyExecutor{verifyOK: false}
	d, _ := svc.Trigger(context.Background(), IncidentInput{
		IncidentID: "inc-4", Service: "svc-a", Environment: "stage", Capability: "sdlc-devops",
		Criticality: "low", SuggestedActions: []string{"refresh-cache"},
	})
	if d.Outcome != OutcomeRolledBack {
		t.Fatalf("expected rolled_back, got %s reason=%s", d.Outcome, d.Reason)
	}
	sink := svc.Sink.(*MemorySink)
	hasRolled, hasEscalated := false, false
	for _, e := range sink.Events {
		if e.Type == "healing.rolled_back.v1" {
			hasRolled = true
		}
		if e.Type == "healing.escalated.v1" {
			hasEscalated = true
		}
	}
	if !hasRolled || !hasEscalated {
		t.Fatalf("expected rolled_back+escalated events, got %v", sink.Events)
	}
}

func TestPromotionPrerequisitesUnmet(t *testing.T) {
	store := NewStore()
	setupAction(store)
	svc := NewService(store, &MemorySink{})
	// Only 10 successful runs — should fail.
	store.SetPromotionStats("restart-pod", "stage", PromotionMetrics{
		EvalPassRateLast50: 0.99, SuccessfulL3Runs: 10, DaysSinceLastPostmortem: 60,
	})
	err := svc.PromoteAction(PromotionRequest{
		ActionID: "restart-pod", Environment: "stage", TargetLevel: LevelL4,
		PlatformAdminOK: true, SecurityOK: true,
	})
	if !errors.Is(err, ErrPromotionPrerequisites) {
		t.Fatalf("expected prereq error, got %v", err)
	}
}

func TestPromotionGrantsLevel(t *testing.T) {
	store := NewStore()
	setupAction(store)
	svc := NewService(store, &MemorySink{})
	store.SetPromotionStats("restart-pod", "stage", PromotionMetrics{
		EvalPassRateLast50: 0.97, SuccessfulL3Runs: 22, DaysSinceLastPostmortem: 60,
	})
	if err := svc.PromoteAction(PromotionRequest{
		ActionID: "restart-pod", Environment: "stage", TargetLevel: LevelL4,
		PlatformAdminOK: true, SecurityOK: true,
	}); err != nil {
		t.Fatalf("promote: %v", err)
	}
	a := store.GetAction("restart-pod")
	if !levelAllowed(a.AllowedLevelsByEnv["stage"], LevelL4) {
		t.Fatalf("expected L4 added to stage, got %v", a.AllowedLevelsByEnv["stage"])
	}
}

func TestRateLimitBlocksFurtherInvocations(t *testing.T) {
	store := NewStore()
	setupAction(store)
	env := setupEnvelope(store, "prod", "low", LevelL2, AllLevels)
	env.MaxActionsPerHour = 1
	store.SetEnvelope(env)
	svc := NewService(store, &MemorySink{})
	in := IncidentInput{
		IncidentID: "inc-rl", Service: "svc", Environment: "prod", Capability: "sdlc-devops",
		Criticality: "low", SuggestedActions: []string{"restart-pod"},
	}
	if _, err := svc.Trigger(context.Background(), in); err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	in.IncidentID = "inc-rl-2"
	d2, _ := svc.Trigger(context.Background(), in)
	if d2.Outcome != OutcomeBlockedByLimits {
		t.Fatalf("expected rate-limit block, got %s", d2.Outcome)
	}
}

// flakyExecutor verifies fail to test L5 rollback path.
type flakyExecutor struct{ verifyOK bool }

func (f *flakyExecutor) Execute(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "wfr-1", nil
}
func (f *flakyExecutor) Verify(_ context.Context, _ string) (bool, error) { return f.verifyOK, nil }
func (f *flakyExecutor) Rollback(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "wfr-rb-1", nil
}

func TestApprovalTTL(t *testing.T) {
	// Just verify the field exists and is settable; integration tests run against approvals service.
	store := NewStore()
	svc := NewService(store, &MemorySink{})
	svc.ApprovalTTL = time.Minute
	if svc.ApprovalTTL != time.Minute {
		t.Fatal("ttl not configurable")
	}
}
