package evolution

import (
	"context"
	"testing"
)

func sampleInput() PostmortemInput {
	return PostmortemInput{
		IncidentID:     "inc-1",
		TenantID:       "t-1",
		WorkspaceID:    "w-1",
		AssetID:        "application/web/svc-foo",
		Service:        "svc-foo",
		Environment:    "prod",
		Severity:       "critical",
		Summary:        "5xx storm caused by stale cache",
		RootCause:      "Stale Redis prefix returned 5xx",
		Lessons:        []string{"Detect stale cache faster", "Add fallback to DB"},
		HealingActions: []string{"refresh-cache"},
		PostmortemURL:  "confluence://forge/pm-1",
	}
}

func TestFromPostmortemEmitsEvent(t *testing.T) {
	store := NewStore()
	svc := NewService(store, &MemorySink{})
	p, err := svc.FromPostmortem(context.Background(), sampleInput())
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if p.Source != AutonomousLoopMarker {
		t.Fatalf("expected source=autonomous-loop")
	}
	if p.SkillVersion != SkillVersion {
		t.Fatalf("expected skill_version=%s", SkillVersion)
	}
	sink := svc.Sink.(*MemorySink)
	if len(sink.Events) != 1 || sink.Events[0].Type != "evolution.openspec_proposal.v1" {
		t.Fatalf("unexpected events: %+v", sink.Events)
	}
}

func TestSuggestionsIncludeAcceptanceCriteriaAndGate(t *testing.T) {
	store := NewStore()
	svc := NewService(store, &MemorySink{})
	p, _ := svc.FromPostmortem(context.Background(), sampleInput())
	kinds := map[ProposalKind]int{}
	for _, s := range p.Suggestions {
		kinds[s.Kind]++
	}
	if kinds[KindAcceptanceCriteria] < 2 {
		t.Fatalf("expected 2 AC suggestions, got %d", kinds[KindAcceptanceCriteria])
	}
	if kinds[KindNewGate] < 1 {
		t.Fatalf("expected 1 new-gate suggestion for severity=critical, got %d", kinds[KindNewGate])
	}
}

func TestReviewApprovesAndConvertsToChange(t *testing.T) {
	store := NewStore()
	svc := NewService(store, &MemorySink{})
	p, _ := svc.FromPostmortem(context.Background(), sampleInput())
	updated, err := svc.Review(context.Background(), p.ID, ReviewDecision{Approved: true, Reviewer: "alice"})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if updated.Status != StatusConverted {
		t.Fatalf("expected converted, got %s", updated.Status)
	}
	if !startsWithPrefix(updated.OpenSpecChange, "change-") {
		t.Fatalf("expected change id, got %s", updated.OpenSpecChange)
	}
}

func TestReviewRejection(t *testing.T) {
	store := NewStore()
	svc := NewService(store, &MemorySink{})
	p, _ := svc.FromPostmortem(context.Background(), sampleInput())
	updated, _ := svc.Review(context.Background(), p.ID, ReviewDecision{Approved: false, Reviewer: "bob", Comment: "duplicate"})
	if updated.Status != StatusRejected {
		t.Fatalf("expected rejected, got %s", updated.Status)
	}
}

func TestStatsTracksCounts(t *testing.T) {
	store := NewStore()
	svc := NewService(store, &MemorySink{})
	p1, _ := svc.FromPostmortem(context.Background(), sampleInput())
	in2 := sampleInput()
	in2.IncidentID = "inc-2"
	p2, _ := svc.FromPostmortem(context.Background(), in2)
	_, _ = svc.Review(context.Background(), p1.ID, ReviewDecision{Approved: true, Reviewer: "alice"})
	_, _ = svc.Review(context.Background(), p2.ID, ReviewDecision{Approved: false, Reviewer: "bob"})
	stats := store.Stats()
	if stats["total"] != 2 {
		t.Fatalf("expected total=2, got %d", stats["total"])
	}
	if stats["converted"] != 1 || stats["rejected"] != 1 {
		t.Fatalf("expected converted=1, rejected=1, got %v", stats)
	}
}

func startsWithPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
