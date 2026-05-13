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
	// Source identifies where the invocation came from: "runtime" (default
	// in-platform), "workflow", or "gateway" (developer skill gateway).
	// Used by per-source rollups so gateway traffic is queryable on its
	// own and drift detection can identify a contributing source.
	Source          string    `json:"source,omitempty"`
}

// Install is one record of an external client installing an asset via the
// gateway. Keyed by (developer_sub, asset_id, client); a repeat install
// updates last_seen_at and the version pointer instead of inserting a row.
type Install struct {
	AssetID       string    `json:"asset_id"`
	AssetVersion  string    `json:"asset_version"`
	TenantID      string    `json:"tenant_id"`
	DeveloperSub  string    `json:"developer_sub"`
	Client        string    `json:"client"`
	PackageDigest string    `json:"package_digest,omitempty"`
	InstalledAt   time.Time `json:"installed_at"`
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

// SourceRollup is a per-source breakdown of a window's invocations.
type SourceRollup struct {
	Source       string  `json:"source"`
	Invocations  int     `json:"invocations"`
	Successes    int     `json:"successes"`
	Failures     int     `json:"failures"`
	SuccessRate  float64 `json:"success_rate"`
	CostTotalUSD float64 `json:"cost_total_usd"`
	EvalScoreAvg *float64 `json:"eval_score_avg,omitempty"`
}

// InstallSummary is the per-asset rollup of gateway-originated installs.
type InstallSummary struct {
	Total      int            `json:"total"`
	Active     int            `json:"active"`
	ByVersion  map[string]int `json:"by_version,omitempty"`
	ByClient   map[string]int `json:"by_client,omitempty"`
}

// MetricSeries is a sorted series of MetricPoints plus rollup totals.
type MetricSeries struct {
	AssetID     string         `json:"asset_id"`
	AssetType   AssetType      `json:"asset_type"`
	Range       MetricRange    `json:"range"`
	Granularity Granularity    `json:"granularity"`
	Points      []MetricPoint  `json:"points"`
	Totals      MetricPoint    `json:"totals"`
	TopFailing  []string       `json:"top_failing_steps,omitempty"`
	DriftAlert  bool           `json:"drift_alert"`
	DriftSource string         `json:"drift_source,omitempty"`
	BySource    []SourceRollup `json:"by_source,omitempty"`
	Installs    *InstallSummary `json:"installs,omitempty"`
}

// Store retains invocations in memory keyed by asset.
type Store struct {
	mu          sync.RWMutex
	byAsset     map[string][]Invocation
	installs    map[string]map[string]Install // assetID -> dedup key -> install
	driftWindow int
	driftDelta  float64
}

// NewStore creates an empty store with sensible defaults.
func NewStore() *Store {
	return &Store{
		byAsset:     map[string][]Invocation{},
		installs:    map[string]map[string]Install{},
		driftWindow: 3,
		driftDelta:  0.05,
	}
}

// All returns a flat copy of every retained invocation across all assets.
// Cheap because the store is in-memory; callers MUST treat the result as
// read-only.
func (s *Store) All() []Invocation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Invocation, 0)
	for _, items := range s.byAsset {
		out = append(out, items...)
	}
	return out
}

// Ingest records an invocation. Sets a default Source when omitted.
func (s *Store) Ingest(inv Invocation) {
	if inv.AssetID == "" {
		return
	}
	if inv.StartedAt.IsZero() {
		inv.StartedAt = time.Now().UTC()
	}
	if inv.Source == "" {
		inv.Source = "runtime"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byAsset[inv.AssetID] = append(s.byAsset[inv.AssetID], inv)
}

// RecordInstall idempotently records (or refreshes) a gateway install.
// Repeat installs of the same (developer, asset, client) update the version
// pointer and InstalledAt without inflating the active count.
func (s *Store) RecordInstall(in Install) {
	if in.AssetID == "" || in.DeveloperSub == "" || in.Client == "" {
		return
	}
	if in.InstalledAt.IsZero() {
		in.InstalledAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.installs[in.AssetID] == nil {
		s.installs[in.AssetID] = map[string]Install{}
	}
	key := in.DeveloperSub + "|" + in.Client
	s.installs[in.AssetID][key] = in
}

// Errors.
var (
	ErrUnknownRange = errors.New("unknown_range")
)

// Aggregate produces a MetricSeries for an asset and range. The optional
// sourceFilter narrows the rollup to a single source (e.g. "gateway"). When
// empty, the union of all sources is used and BySource is populated.
func (s *Store) Aggregate(assetID string, rng MetricRange, gran Granularity, now time.Time, sourceFilter ...string) (*MetricSeries, error) {
	window, err := windowFor(rng)
	if err != nil {
		return nil, err
	}
	source := ""
	if len(sourceFilter) > 0 {
		source = sourceFilter[0]
	}
	bucket := bucketFor(gran)
	cutoff := now.Add(-window)
	s.mu.RLock()
	invs := append([]Invocation(nil), s.byAsset[assetID]...)
	installs := append([]Install(nil), valuesFor(s.installs[assetID])...)
	s.mu.RUnlock()
	filtered := []Invocation{}
	for _, inv := range invs {
		if !inv.StartedAt.After(cutoff) {
			continue
		}
		if source != "" && inv.Source != source {
			continue
		}
		filtered = append(filtered, inv)
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
	driftAlert, driftSource := s.detectDriftWithSource(filtered)
	out := &MetricSeries{
		AssetID:     assetID,
		AssetType:   inferType(filtered),
		Range:       rng,
		Granularity: gran,
		Points:      points,
		Totals:      totals,
		TopFailing:  topFailing,
		DriftAlert:  driftAlert,
		DriftSource: driftSource,
	}
	if source == "" {
		out.BySource = sourceRollups(filtered)
		out.Installs = installSummary(installs, now)
	}
	return out, nil
}

func valuesFor(m map[string]Install) []Install {
	if len(m) == 0 {
		return nil
	}
	out := make([]Install, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func sourceRollups(invs []Invocation) []SourceRollup {
	bySource := map[string][]Invocation{}
	for _, inv := range invs {
		bySource[inv.Source] = append(bySource[inv.Source], inv)
	}
	out := make([]SourceRollup, 0, len(bySource))
	for src, group := range bySource {
		s := summarize(time.Time{}, group)
		var evalAvg *float64
		if s.EvalScoreAvg != nil {
			evalAvg = s.EvalScoreAvg
		}
		out = append(out, SourceRollup{
			Source:       src,
			Invocations:  s.Invocations,
			Successes:    s.Successes,
			Failures:     s.Failures,
			SuccessRate:  s.SuccessRate,
			CostTotalUSD: s.CostTotalUSD,
			EvalScoreAvg: evalAvg,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

func installSummary(installs []Install, now time.Time) *InstallSummary {
	if len(installs) == 0 {
		return nil
	}
	out := &InstallSummary{
		ByVersion: map[string]int{},
		ByClient:  map[string]int{},
	}
	cutoff := now.Add(-30 * 24 * time.Hour)
	for _, in := range installs {
		out.Total++
		if in.InstalledAt.After(cutoff) {
			out.Active++
		}
		out.ByVersion[in.AssetVersion]++
		out.ByClient[in.Client]++
	}
	return out
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
	ok, _ := s.detectDriftWithSource(invs)
	return ok
}

// detectDriftWithSource returns (alert, source) — the source is the per-source
// series whose tail drove the alert, or "" when the union triggered.
func (s *Store) detectDriftWithSource(invs []Invocation) (bool, string) {
	scores := []float64{}
	for _, inv := range invs {
		if inv.EvalScore != nil {
			scores = append(scores, *inv.EvalScore)
		}
	}
	if len(scores) >= s.driftWindow*2 {
		baseline := mean(scores[:len(scores)-s.driftWindow])
		recent := mean(scores[len(scores)-s.driftWindow:])
		if baseline-recent > s.driftDelta {
			return true, ""
		}
	}
	// Per-source detection: a regression in only one source counts.
	bySource := map[string][]float64{}
	for _, inv := range invs {
		if inv.EvalScore == nil {
			continue
		}
		bySource[inv.Source] = append(bySource[inv.Source], *inv.EvalScore)
	}
	for src, vals := range bySource {
		if len(vals) < s.driftWindow*2 {
			continue
		}
		baseline := mean(vals[:len(vals)-s.driftWindow])
		recent := mean(vals[len(vals)-s.driftWindow:])
		if baseline-recent > s.driftDelta {
			return true, src
		}
	}
	return false, ""
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
