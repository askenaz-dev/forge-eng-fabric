package engine

import (
	"context"
	"testing"
)

// ---- helpers ----

func newServiceForL1L2() (*Service, *MemorySink) {
	store := NewStore()
	sink := &MemorySink{}
	svc := NewService(store, sink)
	return svc, sink
}

func hasEvent(sink *MemorySink, eventType string) bool {
	for _, e := range sink.Events {
		if e.Type == eventType {
			return true
		}
	}
	return false
}

// ---- tests ----

// TestDetectEmitsHealingDetectedV1 verifies that Detect emits healing.detected.v1.
func TestDetectEmitsHealingDetectedV1(t *testing.T) {
	svc, sink := newServiceForL1L2()

	det, err := svc.Detect(context.Background(), DetectionInput{
		AppID:        "app-001",
		TenantID:     "tenant-1",
		WorkspaceID:  "ws-1",
		CorrelationID: "corr-1",
		SignalSource: SignalPrometheus,
		CandidateHypotheses: []Hypothesis{
			{ID: "h1", Description: "high error rate", Confidence: 0.9},
		},
		CandidateActions:    []string{"restart-pod"},
		BlastRadiusEstimate: "pod",
	})

	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if det == nil {
		t.Fatal("Detect returned nil detection")
	}
	if det.ID == "" {
		t.Error("detection ID must not be empty")
	}
	if det.AppID != "app-001" {
		t.Errorf("expected AppID=app-001, got %q", det.AppID)
	}

	if !hasEvent(sink, "healing.detected.v1") {
		t.Errorf("expected healing.detected.v1 event; got events: %v", eventTypes(sink))
	}

	// Verify it was stored.
	stored := svc.Store.GetDetection(det.ID)
	if stored == nil {
		t.Error("detection was not persisted to the store")
	}
}

// TestProposeFixEmitsHealingFixProposedV1 verifies that ProposeFix emits
// healing.fix_proposed.v1 when the fix passes safety checks.
func TestProposeFixEmitsHealingFixProposedV1(t *testing.T) {
	svc, sink := newServiceForL1L2()

	sug, err := svc.ProposeFix(context.Background(), ProposeFixInput{
		AppID:        "app-002",
		TenantID:     "tenant-1",
		WorkspaceID:  "ws-1",
		CorrelationID: "corr-2",
		TopHypothesis: Hypothesis{
			ID:          "h1",
			Description: "misconfigured replica count",
			Confidence:  0.85,
			AffectedFiles: []string{"deploy/values.yaml"},
		},
		DiffType:        "config",
		SizeBudgetLines: 200,
	})

	if err != nil {
		t.Fatalf("ProposeFix returned error: %v", err)
	}
	if sug == nil {
		t.Fatal("ProposeFix returned nil suggestion")
	}
	if !sug.SafetyPassed {
		t.Errorf("expected safety to pass, reason: %s", sug.RejectionReason)
	}

	if !hasEvent(sink, "healing.fix_proposed.v1") {
		t.Errorf("expected healing.fix_proposed.v1 event; got: %v", eventTypes(sink))
	}

	// Stored.
	stored := svc.Store.GetSuggestion(sug.ID)
	if stored == nil {
		t.Error("suggestion was not persisted to the store")
	}
}

// TestSafetyEvalFailsSizeBudget verifies SafetyEval rejects fixes that exceed
// the line budget.
func TestSafetyEvalFailsSizeBudget(t *testing.T) {
	svc, _ := newServiceForL1L2()

	fix := ProposedFix{
		ID:       "fix-1",
		AppID:    "app-x",
		DiffType: "code",
		FileDiffs: []FileDiff{
			{Path: "main.go", Before: "", After: "", LinesChanged: 100},
			{Path: "util.go", Before: "", After: "", LinesChanged: 150},
		},
	}

	result := svc.SafetyEval(fix, nil, 200)

	if result.Passed {
		t.Error("expected SafetyEval to fail on size budget exceeded")
	}
	if result.Reason == "" {
		t.Error("expected a non-empty failure reason")
	}
}

// TestSafetyEvalFailsProtectedPath verifies SafetyEval rejects fixes that
// touch protected file paths.
func TestSafetyEvalFailsProtectedPath(t *testing.T) {
	svc, _ := newServiceForL1L2()

	fix := ProposedFix{
		ID:       "fix-2",
		AppID:    "app-x",
		DiffType: "config",
		FileDiffs: []FileDiff{
			{Path: "secrets/prod.env", Before: "", After: "CHANGED=1", LinesChanged: 1},
		},
	}

	result := svc.SafetyEval(fix, []string{"secrets/"}, 200)

	if result.Passed {
		t.Error("expected SafetyEval to fail on protected path")
	}
}

// TestSafetyEvalFailsSecretReference verifies SafetyEval rejects fixes whose
// diff content contains secret-like patterns.
func TestSafetyEvalFailsSecretReference(t *testing.T) {
	svc, _ := newServiceForL1L2()

	fix := ProposedFix{
		ID:       "fix-3",
		AppID:    "app-x",
		DiffType: "config",
		FileDiffs: []FileDiff{
			{
				Path:         "config/app.yaml",
				Before:       "",
				After:        "db_password=supersecret123",
				LinesChanged: 1,
			},
		},
	}

	result := svc.SafetyEval(fix, nil, 200)

	if result.Passed {
		t.Error("expected SafetyEval to fail on secret reference in diff")
	}
}

// TestDowngradeEmitsHealingFixDowngradedV1 verifies that ProposeFix emits
// healing.fix_downgraded.v1 when a fix fails safety evaluation.
func TestDowngradeEmitsHealingFixDowngradedV1(t *testing.T) {
	svc, sink := newServiceForL1L2()

	// Build input where the fix will exceed the size budget.
	sug, err := svc.ProposeFix(context.Background(), ProposeFixInput{
		AppID:        "app-003",
		TenantID:     "tenant-1",
		WorkspaceID:  "ws-1",
		CorrelationID: "corr-3",
		TopHypothesis: Hypothesis{
			ID:          "h1",
			Description: "large generated code change",
			Confidence:  0.7,
			// Many affected files so synthesiseFix produces enough diffs to
			// push LinesChanged over budget.
			AffectedFiles: []string{
				"a.go", "b.go", "c.go", "d.go", "e.go",
				"f.go", "g.go", "h.go", "i.go", "j.go",
				"k.go", "l.go", "m.go", "n.go", "o.go",
				"p.go", "q.go", "r.go", "s.go", "t.go",
				"u.go", "v.go", "w.go", "x.go", "y.go",
				"z1.go", "z2.go", "z3.go", "z4.go", "z5.go",
				"z6.go", "z7.go", "z8.go", "z9.go", "z10.go",
				"z11.go", "z12.go", "z13.go", "z14.go", "z15.go",
				"z16.go", "z17.go", "z18.go", "z19.go", "z20.go",
				"z21.go", "z22.go", "z23.go", "z24.go", "z25.go",
				"z26.go", "z27.go", "z28.go", "z29.go", "z30.go",
				"z31.go", "z32.go", "z33.go", "z34.go", "z35.go",
				"z36.go", "z37.go", "z38.go", "z39.go", "z40.go",
				"z41.go", "z42.go", "z43.go", "z44.go", "z45.go",
				"z46.go", "z47.go", "z48.go", "z49.go", "z50.go",
				"z51.go", "z52.go", "z53.go", "z54.go", "z55.go",
				"z56.go", "z57.go", "z58.go", "z59.go", "z60.go",
				"z61.go", "z62.go", "z63.go", "z64.go", "z65.go",
				"z66.go", "z67.go", "z68.go", "z69.go", "z70.go",
				"z71.go", "z72.go", "z73.go", "z74.go", "z75.go",
				"z76.go", "z77.go", "z78.go", "z79.go", "z80.go",
				"z81.go", "z82.go", "z83.go", "z84.go", "z85.go",
				"z86.go", "z87.go", "z88.go", "z89.go", "z90.go",
				"z91.go", "z92.go", "z93.go", "z94.go", "z95.go",
				"z96.go", "z97.go", "z98.go", "z99.go", "z100.go",
			},
		},
		DiffType:        "code",
		SizeBudgetLines: 5, // tiny budget so the large fix gets rejected
	})

	if err != nil {
		t.Fatalf("ProposeFix returned error: %v", err)
	}
	if sug.SafetyPassed {
		t.Error("expected safety to fail")
	}

	if !hasEvent(sink, "healing.fix_downgraded.v1") {
		t.Errorf("expected healing.fix_downgraded.v1 event; got: %v", eventTypes(sink))
	}
	if hasEvent(sink, "healing.fix_proposed.v1") {
		t.Error("healing.fix_proposed.v1 must NOT be emitted when downgraded")
	}
}

// TestListDetectionsAndSuggestions verifies the Store list helpers filter by appID.
func TestListDetectionsAndSuggestions(t *testing.T) {
	svc, _ := newServiceForL1L2()

	// Create two detections for app-A and one for app-B.
	for i := 0; i < 2; i++ {
		_, _ = svc.Detect(context.Background(), DetectionInput{
			AppID:        "app-A",
			TenantID:     "t1",
			SignalSource: SignalCIFailed,
		})
	}
	_, _ = svc.Detect(context.Background(), DetectionInput{
		AppID:        "app-B",
		TenantID:     "t1",
		SignalSource: SignalDeployFailed,
	})

	aList := svc.Store.ListDetections("app-A")
	if len(aList) != 2 {
		t.Errorf("expected 2 detections for app-A, got %d", len(aList))
	}
	bList := svc.Store.ListDetections("app-B")
	if len(bList) != 1 {
		t.Errorf("expected 1 detection for app-B, got %d", len(bList))
	}

	// Suggestions.
	_, _ = svc.ProposeFix(context.Background(), ProposeFixInput{
		AppID:         "app-A",
		TenantID:      "t1",
		TopHypothesis: Hypothesis{ID: "h1", Description: "test"},
		SizeBudgetLines: 200,
	})
	sugList := svc.Store.ListSuggestions("app-A")
	if len(sugList) != 1 {
		t.Errorf("expected 1 suggestion for app-A, got %d", len(sugList))
	}
}

// eventTypes is a diagnostic helper that collects event type strings.
func eventTypes(sink *MemorySink) []string {
	out := make([]string, len(sink.Events))
	for i, e := range sink.Events {
		out[i] = e.Type
	}
	return out
}
