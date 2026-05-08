// Package observability aggregates per-asset metrics from Langfuse, Temporal
// and the platform bus. The aggregations power the Observability tab in the
// Portal Asset detail.
package observability

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// AssetType identifies the asset class.
type AssetType string

const (
	AssetTypeSkill    AssetType = "skill"
	AssetTypePrompt   AssetType = "prompt"
	AssetTypeWorkflow AssetType = "workflow"
	AssetTypeMCP      AssetType = "mcp_tool"
)

// Invocation is one instrumented invocation contributing to per-asset
// aggregates. Invocations are typically forwarded by the runtime, by
// Langfuse callbacks (LLM cost) and by skill SDKs.
type Invocation struct {
	AssetID         string    `json:"asset_id"`
	AssetType       AssetType `json:"asset_type"`
	AssetVersion    string    `json:"asset_version"`
	TenantID        string    `json:"tenant_id"`
	WorkspaceID     string    `json:"workspace_id"`
	StartedAt       time.Time `json:"started_at"`
	DurationMS      float64   `json:"duration_ms"`
	Success         bool      `json:"success"`
	LLMCostUSD      float64   `json:"llm_cost_usd"`
	ComputeCostUSD  float64   `json:"compute_cost_usd"`
	EvalScore       *float64  `json:"eval_score,omitempty"`
	StepFailures    []string  `json:"step_failures,omitempty"`
	BusinessMetric  *float64  `json:"business_metric,omitempty"`
	BusinessKey     string    `json:"business_metric_key,omitempty"`
	CorrelationID   string    `json:"correlation_id,omitempty"`
}

// MetricRange selects the time window for queries.
type MetricRange string

const (
	Range1h  MetricRange = "1h"
	Range24h MetricRange = "24h"
	Range7d  MetricRange = "7d"
	Range30d MetricRange = "30d"
)

// Granularity bucket size.
type Granularity string

const (
	GranMinute Granularity = "minute"
	GranHour   Granularity = "hour"
	GranDay    Granularity = "day"
)

// MetricPoint is a single time bucket.
type MetricPoint struct {
	Bucket          time.Time `json:"bucket"`
	Invocations     int       `json:"invocations"`
	Successes       int       `json:"successes"`
	Failures        int       `json:"failures"`
	SuccessRate     float64   `json:"success_rate"`
	LatencyP50      float64   `json:"latency_p50_ms"`
	LatencyP95      float64   `json:"latency_p95_ms"`
	LatencyP99      float64   `json:"latency_p99_ms"`
	CostLLMUSD      float64   `json:"cost_llm_usd"`
	CostComputeUSD  float64   `json:"cost_compute_usd"`
	CostTotalUSD    float64   `json:"cost_total_usd"`
	EvalScoreAvg    *float64  `json:"eval_score_avg,omitempty"`
	BusinessMetric  *float64  `json:"business_metric_avg,omitempty"`
	BusinessKey     string    `json:"business_metric_key,omitempty"`
}

// MetricSeries is a sorted series of MetricPoints plus rollup totals.
type MetricSeries struct {
	AssetID     string        `json:"asset_id"`
	AssetType   AssetType     `json:"asset_type"`
	Range       MetricRange   `json:"range"`
	Granularity Granularity   `json:"granularity"`
	Points      []MetricPoint `json:"points"`
	Totals      MetricPoint   `json:"totals"`
	TopFailing  []string      `json:"top_failing_steps,omitempty"`
	DriftAlert  bool          `json:"drift_alert"`
}

// Store retains invocations in memory keyed by asset.
type Store struct {
	mu          sync.RWMutex
	byAsset     map[string][]Invocation
	driftWindow int
	driftDelta  float64
}

// NewStore creates an empty store with sensible defaults.
func NewStore() *Store {
	return &Store{
		byAsset:     map[string][]Invocation{},
		driftWindow: 3,
		driftDelta:  0.05,
	}
}

// Ingest records an invocation.
func (s *Store) Ingest(inv Invocation) {
	if inv.AssetID == "" {
		return
	}
	if inv.StartedAt.IsZero() {
		inv.StartedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byAsset[inv.AssetID] = append(s.byAsset[inv.AssetID], inv)
}

// Errors.
var (
	ErrUnknownRange = errors.New("unknown_range")
)

// Aggregate produces a MetricSeries for an asset and range.
func (s *Store) Aggregate(assetID string, rng MetricRange, gran Granularity, now time.Time) (*MetricSeries, error) {
	window, err := windowFor(rng)
	if err != nil {
		return nil, err
	}
	bucket := bucketFor(gran)
	cutoff := now.Add(-window)
	s.mu.RLock()
	invs := append([]Invocation(nil), s.byAsset[assetID]...)
	s.mu.RUnlock()
	filtered := []Invocation{}
	for _, inv := range invs {
		if inv.StartedAt.After(cutoff) {
			filtered = append(filtered, inv)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].StartedAt.Before(filtered[j].StartedAt) })
	buckets := map[time.Time][]Invocation{}
	for _, inv := range filtered {
		key := inv.StartedAt.Truncate(bucket)
		buckets[key] = append(buckets[key], inv)
	}
	keys := make([]time.Time, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
	points := make([]MetricPoint, 0, len(keys))
	for _, k := range keys {
		points = append(points, summarize(k, buckets[k]))
	}
	totals := summarize(cutoff, filtered)
	totals.Bucket = cutoff
	topFailing := topFailingSteps(filtered)
	driftAlert := s.detectDrift(filtered)
	out := &MetricSeries{
		AssetID:     assetID,
		AssetType:   inferType(filtered),
		Range:       rng,
		Granularity: gran,
		Points:      points,
		Totals:      totals,
		TopFailing:  topFailing,
		DriftAlert:  driftAlert,
	}
	return out, nil
}

func summarize(bucket time.Time, invs []Invocation) MetricPoint {
	if len(invs) == 0 {
		return MetricPoint{Bucket: bucket}
	}
	point := MetricPoint{Bucket: bucket, Invocations: len(invs)}
	durations := make([]float64, 0, len(invs))
	evalScores := []float64{}
	business := []float64{}
	businessKey := ""
	for _, inv := range invs {
		if inv.Success {
			point.Successes++
		} else {
			point.Failures++
		}
		durations = append(durations, inv.DurationMS)
		point.CostLLMUSD += inv.LLMCostUSD
		point.CostComputeUSD += inv.ComputeCostUSD
		if inv.EvalScore != nil {
			evalScores = append(evalScores, *inv.EvalScore)
		}
		if inv.BusinessMetric != nil {
			business = append(business, *inv.BusinessMetric)
			businessKey = inv.BusinessKey
		}
	}
	if point.Invocations > 0 {
		point.SuccessRate = float64(point.Successes) / float64(point.Invocations)
	}
	point.CostTotalUSD = point.CostLLMUSD + point.CostComputeUSD
	point.LatencyP50 = percentile(durations, 0.50)
	point.LatencyP95 = percentile(durations, 0.95)
	point.LatencyP99 = percentile(durations, 0.99)
	if len(evalScores) > 0 {
		avg := mean(evalScores)
		point.EvalScoreAvg = &avg
	}
	if len(business) > 0 {
		avg := mean(business)
		point.BusinessMetric = &avg
		point.BusinessKey = businessKey
	}
	return point
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 1 {
		return cp[len(cp)-1]
	}
	idx := int(float64(len(cp)-1) * p)
	return cp[idx]
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func topFailingSteps(invs []Invocation) []string {
	counts := map[string]int{}
	for _, inv := range invs {
		if inv.Success {
			continue
		}
		for _, step := range inv.StepFailures {
			counts[step]++
		}
	}
	type kv struct {
		Step string
		N    int
	}
	pairs := []kv{}
	for k, v := range counts {
		pairs = append(pairs, kv{Step: k, N: v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].N > pairs[j].N })
	out := []string{}
	for i, p := range pairs {
		if i >= 5 {
			break
		}
		out = append(out, p.Step)
	}
	return out
}

func (s *Store) detectDrift(invs []Invocation) bool {
	scores := []float64{}
	for _, inv := range invs {
		if inv.EvalScore != nil {
			scores = append(scores, *inv.EvalScore)
		}
	}
	if len(scores) < s.driftWindow*2 {
		return false
	}
	baseline := mean(scores[:len(scores)-s.driftWindow])
	recent := mean(scores[len(scores)-s.driftWindow:])
	return baseline-recent > s.driftDelta
}

func inferType(invs []Invocation) AssetType {
	for _, inv := range invs {
		if inv.AssetType != "" {
			return inv.AssetType
		}
	}
	return ""
}

func windowFor(r MetricRange) (time.Duration, error) {
	switch r {
	case Range1h:
		return time.Hour, nil
	case Range24h:
		return 24 * time.Hour, nil
	case Range7d:
		return 7 * 24 * time.Hour, nil
	case Range30d:
		return 30 * 24 * time.Hour, nil
	}
	return 0, ErrUnknownRange
}

func bucketFor(g Granularity) time.Duration {
	switch g {
	case GranMinute:
		return time.Minute
	case GranHour:
		return time.Hour
	case GranDay:
		return 24 * time.Hour
	}
	return time.Hour
}
