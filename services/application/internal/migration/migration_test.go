package migration

import (
	"context"
	"testing"
	"time"
)

func TestClassify_OrphanWhenNoEvidence(t *testing.T) {
	now := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	rows := []SpecRow{
		{
			SpecID:          "spec-1",
			WorkspaceID:     "ws-1",
			LifecycleStatus: "proposed",
			LastActivity:    now.Add(-200 * 24 * time.Hour),
		},
	}
	report := Classify(rows, now)
	if got := report[0].Classification; got != ClassOrphan {
		t.Fatalf("expected orphan, got %s", got)
	}
}

func TestClassify_RetainWithTargetAppWhenCandidate(t *testing.T) {
	now := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	rows := []SpecRow{
		{
			SpecID:              "spec-2",
			LifecycleStatus:     "proposed",
			HasActiveDeployment: true,
			TargetAppCandidate:  "app-a",
		},
	}
	report := Classify(rows, now)
	if got := report[0].Classification; got != ClassRetainWithTargetApp {
		t.Fatalf("expected retain_with_target_app, got %s", got)
	}
	if report[0].TargetAppID != "app-a" {
		t.Fatalf("expected target_app_id=app-a, got %q", report[0].TargetAppID)
	}
}

func TestClassify_RetainUnassignedWhenRecentlyActive(t *testing.T) {
	now := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	rows := []SpecRow{
		{
			SpecID:          "spec-3",
			LifecycleStatus: "proposed",
			LastActivity:    now.Add(-30 * 24 * time.Hour),
		},
	}
	report := Classify(rows, now)
	if got := report[0].Classification; got != ClassRetainUnassigned {
		t.Fatalf("expected retain_unassigned, got %s", got)
	}
}

func TestClassify_ApprovedNeverOrphan(t *testing.T) {
	now := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	rows := []SpecRow{
		{
			SpecID:          "spec-4",
			LifecycleStatus: "approved",
			LastActivity:    now.Add(-1000 * 24 * time.Hour),
		},
	}
	report := Classify(rows, now)
	if got := report[0].Classification; got == ClassOrphan {
		t.Fatalf("approved specs MUST NOT be orphans, got %s", got)
	}
}

func TestApply_EmitsExpectedEvents(t *testing.T) {
	report := []ClassificationResult{
		{SpecID: "spec-keep", Classification: ClassRetainWithTargetApp, TargetAppID: "app-a"},
		{SpecID: "spec-bin", Classification: ClassOrphan},
	}
	plan, err := PlanExecution(report, "system:test")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	var events []EmittedEvent
	result := Apply(context.Background(), plan, func(eventType, subject string, data map[string]any) {
		events = append(events, EmittedEvent{Type: eventType, Subject: subject, Data: data})
	})
	if result.Backfilled != 1 || result.Purged != 1 {
		t.Fatalf("expected 1 backfilled + 1 purged, got %+v", result)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "spec.reparented.v1" || events[1].Type != "spec.purged.v1" {
		t.Fatalf("unexpected event types: %+v", events)
	}
}
