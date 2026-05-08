package observability

import (
	"testing"
	"time"
)

func TestAggregateSummarizesSuccessAndCost(t *testing.T) {
	store := NewStore()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		store.Ingest(Invocation{
			AssetID:        "wf-1",
			AssetType:      AssetTypeWorkflow,
			TenantID:       "t1",
			StartedAt:      now.Add(-time.Duration(i) * time.Minute),
			DurationMS:     float64(100 + i*10),
			Success:        i != 0,
			LLMCostUSD:     0.02,
			ComputeCostUSD: 0.005,
		})
	}
	series, err := store.Aggregate("wf-1", Range1h, GranMinute, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if series.Totals.Invocations != 4 {
		t.Fatalf("invocations: %d", series.Totals.Invocations)
	}
	if series.Totals.Successes != 3 {
		t.Fatalf("successes: %d", series.Totals.Successes)
	}
	if series.Totals.SuccessRate < 0.74 || series.Totals.SuccessRate > 0.76 {
		t.Fatalf("success rate: %f", series.Totals.SuccessRate)
	}
	if series.Totals.CostTotalUSD <= 0 {
		t.Fatalf("cost: %f", series.Totals.CostTotalUSD)
	}
}

func TestDriftDetectionFires(t *testing.T) {
	store := NewStore()
	store.driftWindow = 3
	store.driftDelta = 0.05
	now := time.Now().UTC()
	score := func(v float64) *float64 { return &v }
	highs := []float64{0.95, 0.96, 0.95, 0.94, 0.95, 0.96}
	lows := []float64{0.80, 0.78, 0.79}
	for i, v := range highs {
		store.Ingest(Invocation{AssetID: "s1", StartedAt: now.Add(-time.Duration(20-i) * time.Minute), Success: true, EvalScore: score(v)})
	}
	for i, v := range lows {
		store.Ingest(Invocation{AssetID: "s1", StartedAt: now.Add(-time.Duration(3-i) * time.Minute), Success: true, EvalScore: score(v)})
	}
	series, err := store.Aggregate("s1", Range24h, GranHour, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("agg: %v", err)
	}
	if !series.DriftAlert {
		t.Fatalf("expected drift alert")
	}
}
