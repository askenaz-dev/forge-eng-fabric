package sdlc

import (
	"context"
	"strings"
	"testing"
)

func TestCreateInitiativeEntersProduct(t *testing.T) {
	sink := &MemorySink{}
	svc := NewService(NewStore(), sink)
	initiative, err := svc.CreateInitiative(context.Background(), CreateInitiativeRequest{
		WorkspaceID:  "ws-1",
		OpenSpecRoot: "spec-7",
		Criticality:  "high",
		Actor:        "alfred",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if initiative.CurrentPhase != PhaseProduct {
		t.Fatalf("current phase = %s", initiative.CurrentPhase)
	}
	if got := len(sink.ByType(EventPhaseEntered)); got != 1 {
		t.Fatalf("entered events = %d", got)
	}
}

func TestCompletePhaseProgressesWhenGatesPass(t *testing.T) {
	sink := &MemorySink{}
	svc := NewService(NewStore(), sink)
	initiative, _ := svc.CreateInitiative(context.Background(), CreateInitiativeRequest{WorkspaceID: "ws-1", OpenSpecRoot: "spec-7"})
	updated, err := svc.CompletePhase(context.Background(), initiative.ID, PhaseProduct, CompletePhaseRequest{
		Evidence: map[string]any{"acceptance_criteria_present": true, "story_size_estimated": true},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if updated.CurrentPhase != PhaseArchitecture {
		t.Fatalf("current phase = %s", updated.CurrentPhase)
	}
	if got := len(sink.ByType(EventPhaseProgressed)); got != 1 {
		t.Fatalf("progressed events = %d", got)
	}

	duplicate, err := svc.CompletePhase(context.Background(), initiative.ID, PhaseProduct, CompletePhaseRequest{})
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if duplicate.CurrentPhase != PhaseArchitecture {
		t.Fatalf("duplicate changed phase to %s", duplicate.CurrentPhase)
	}
	if got := len(sink.ByType(EventPhaseProgressed)); got != 1 {
		t.Fatalf("duplicate emitted progress event, got %d", got)
	}
}

func TestGateFailureBlocksProgression(t *testing.T) {
	sink := &MemorySink{}
	svc := NewService(NewStore(), sink)
	initiative, _ := svc.CreateInitiative(context.Background(), CreateInitiativeRequest{WorkspaceID: "ws-1", OpenSpecRoot: "spec-7"})
	blocked, err := svc.CompletePhase(context.Background(), initiative.ID, PhaseProduct, CompletePhaseRequest{
		Evidence: map[string]any{"acceptance_criteria_present": false, "story_size_estimated": true},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if blocked.CurrentPhase != PhaseProduct {
		t.Fatalf("current phase = %s", blocked.CurrentPhase)
	}
	state := phaseState(blocked, PhaseProduct)
	if state == nil || state.Status != StatusBlocked || len(state.Blockers) != 1 {
		t.Fatalf("unexpected state: %+v", state)
	}
	if got := len(sink.ByType(EventPhaseBlocked)); got != 1 {
		t.Fatalf("blocked events = %d", got)
	}
}

func TestOverrideProgressesDespiteGateFailure(t *testing.T) {
	sink := &MemorySink{}
	svc := NewService(NewStore(), sink)
	initiative, _ := svc.CreateInitiative(context.Background(), CreateInitiativeRequest{WorkspaceID: "ws-1", OpenSpecRoot: "spec-7"})
	updated, err := svc.CompletePhase(context.Background(), initiative.ID, PhaseProduct, CompletePhaseRequest{
		Evidence: map[string]any{"acceptance_criteria_present": false, "story_size_estimated": true},
		Override: &OverrideInput{Approved: true, ApprovedBy: "bob", ApproverRole: "release-manager", Reason: "hotfix", TTLSeconds: 3600},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if updated.CurrentPhase != PhaseArchitecture {
		t.Fatalf("current phase = %s", updated.CurrentPhase)
	}
	if got := len(sink.ByType(EventOverrideConsumed)); got != 1 {
		t.Fatalf("override consumed events = %d", got)
	}
}

func TestEventWorkerCompletesPhase(t *testing.T) {
	svc := NewService(NewStore(), &MemorySink{})
	initiative, _ := svc.CreateInitiative(context.Background(), CreateInitiativeRequest{WorkspaceID: "ws-1", OpenSpecRoot: "spec-7"})
	updated, err := svc.HandleEvent(context.Background(), BusEvent{
		Type: "test.run.completed.v1",
		Data: map[string]any{
			"initiative_id": initiative.ID,
			"phase":         "product",
			"evidence": map[string]any{
				"acceptance_criteria_present": true,
				"story_size_estimated":        true,
			},
		},
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if updated.CurrentPhase != PhaseArchitecture {
		t.Fatalf("current phase = %s", updated.CurrentPhase)
	}
}

func TestMetricsExposePhaseAndGateRates(t *testing.T) {
	svc := NewService(NewStore(), &MemorySink{})
	initiative, _ := svc.CreateInitiative(context.Background(), CreateInitiativeRequest{WorkspaceID: "ws-1", OpenSpecRoot: "spec-7"})
	_, _ = svc.CompletePhase(context.Background(), initiative.ID, PhaseProduct, CompletePhaseRequest{Evidence: map[string]any{"acceptance_criteria_present": true, "story_size_estimated": true}})
	metrics := svc.Metrics()
	for _, name := range []string{"sdlc_phase_duration_seconds", "gate_pass_rate", "phase_block_rate"} {
		if !strings.Contains(metrics, name) {
			t.Fatalf("metrics missing %s: %s", name, metrics)
		}
	}
}
