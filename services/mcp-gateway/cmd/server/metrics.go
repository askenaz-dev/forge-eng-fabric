package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Lightweight Prometheus exposition format implementation. We track the
// four metrics the spec calls for — `requests_total`, `latency`,
// `errors`, `budget_blocks` — without adding the github.com/prometheus
// client_golang dependency tree. The exposed /metrics endpoint serves
// the canonical text format that any Prometheus scraper understands.

// counterVec is a simple label-keyed counter. We pre-declare the label
// set so the metric is well-formed even before any data points exist.
type counterVec struct {
	name   string
	help   string
	labels []string
	mu     sync.Mutex
	data   map[string]*uint64
}

func newCounterVec(name, help string, labels []string) *counterVec {
	return &counterVec{name: name, help: help, labels: labels, data: map[string]*uint64{}}
}

func (c *counterVec) Inc(labelValues ...string) {
	if len(labelValues) != len(c.labels) {
		return
	}
	k := strings.Join(labelValues, "\x1f")
	c.mu.Lock()
	v, ok := c.data[k]
	if !ok {
		var zero uint64
		v = &zero
		c.data[k] = v
	}
	c.mu.Unlock()
	atomic.AddUint64(v, 1)
}

func (c *counterVec) writeTo(sb *strings.Builder) {
	sb.WriteString("# HELP ")
	sb.WriteString(c.name)
	sb.WriteString(" ")
	sb.WriteString(c.help)
	sb.WriteString("\n# TYPE ")
	sb.WriteString(c.name)
	sb.WriteString(" counter\n")
	c.mu.Lock()
	keys := make([]string, 0, len(c.data))
	for k := range c.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := atomic.LoadUint64(c.data[k])
		sb.WriteString(c.name)
		if len(c.labels) > 0 {
			parts := strings.Split(k, "\x1f")
			sb.WriteString("{")
			for i, name := range c.labels {
				if i > 0 {
					sb.WriteString(",")
				}
				fmt.Fprintf(sb, `%s="%s"`, name, escapeLabel(parts[i]))
			}
			sb.WriteString("}")
		}
		fmt.Fprintf(sb, " %d\n", v)
	}
	c.mu.Unlock()
}

// histogramVec is a fixed-bucket histogram. Buckets are tuned for typical
// MCP call latencies (1ms..2s) so the exposed _bucket / _count / _sum
// trio is meaningful to Grafana out of the box.
type histogramVec struct {
	name    string
	help    string
	labels  []string
	buckets []float64 // upper bounds, sorted ascending
	mu      sync.Mutex
	series  map[string]*histSeries
}

type histSeries struct {
	counts []uint64
	sum    float64
	count  uint64
}

func newHistogramVec(name, help string, labels []string, buckets []float64) *histogramVec {
	return &histogramVec{name: name, help: help, labels: labels, buckets: buckets, series: map[string]*histSeries{}}
}

func (h *histogramVec) Observe(value float64, labelValues ...string) {
	if len(labelValues) != len(h.labels) {
		return
	}
	k := strings.Join(labelValues, "\x1f")
	h.mu.Lock()
	s, ok := h.series[k]
	if !ok {
		s = &histSeries{counts: make([]uint64, len(h.buckets))}
		h.series[k] = s
	}
	for i, b := range h.buckets {
		if value <= b {
			s.counts[i]++
		}
	}
	s.sum += value
	s.count++
	h.mu.Unlock()
}

func (h *histogramVec) writeTo(sb *strings.Builder) {
	sb.WriteString("# HELP ")
	sb.WriteString(h.name)
	sb.WriteString(" ")
	sb.WriteString(h.help)
	sb.WriteString("\n# TYPE ")
	sb.WriteString(h.name)
	sb.WriteString(" histogram\n")
	h.mu.Lock()
	keys := make([]string, 0, len(h.series))
	for k := range h.series {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := h.series[k]
		parts := strings.Split(k, "\x1f")
		labelPairs := func(extra string) string {
			items := make([]string, 0, len(h.labels)+1)
			for i, name := range h.labels {
				items = append(items, fmt.Sprintf(`%s="%s"`, name, escapeLabel(parts[i])))
			}
			if extra != "" {
				items = append(items, extra)
			}
			return "{" + strings.Join(items, ",") + "}"
		}
		for i, b := range h.buckets {
			fmt.Fprintf(sb, "%s_bucket%s %d\n", h.name, labelPairs(fmt.Sprintf(`le="%g"`, b)), s.counts[i])
		}
		fmt.Fprintf(sb, "%s_bucket%s %d\n", h.name, labelPairs(`le="+Inf"`), s.count)
		fmt.Fprintf(sb, "%s_count%s %d\n", h.name, labelPairs(""), s.count)
		fmt.Fprintf(sb, "%s_sum%s %g\n", h.name, labelPairs(""), s.sum)
	}
	h.mu.Unlock()
}

func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	return v
}

// Metrics aggregates the gateway's exported series. Constructed once in
// main() and passed to every handler.
type Metrics struct {
	RequestsTotal *counterVec
	Errors        *counterVec
	BudgetBlocks  *counterVec
	Latency       *histogramVec
}

func newMetrics() *Metrics {
	return &Metrics{
		RequestsTotal: newCounterVec("mcp_gateway_requests_total",
			"Total MCP invocations routed by the gateway",
			[]string{"tenant", "workspace", "asset", "source", "outcome"}),
		Errors: newCounterVec("mcp_gateway_errors_total",
			"Total errors observed while routing MCP invocations",
			[]string{"tenant", "asset", "reason"}),
		BudgetBlocks: newCounterVec("mcp_gateway_budget_blocks_total",
			"Total MCP invocations refused by Tenant-budget check",
			[]string{"tenant", "asset"}),
		Latency: newHistogramVec("mcp_gateway_latency_seconds",
			"End-to-end latency in seconds for MCP invocations",
			[]string{"tenant", "asset", "source"},
			[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}),
	}
}

func (m *Metrics) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var sb strings.Builder
		m.RequestsTotal.writeTo(&sb)
		m.Errors.writeTo(&sb)
		m.BudgetBlocks.writeTo(&sb)
		m.Latency.writeTo(&sb)
		w.Header().Set("content-type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(sb.String()))
	}
}

// observe is the canonical post-call telemetry hook. Every invoke path
// calls this exactly once with the per-call labels.
func (m *Metrics) observe(tenant, workspace, asset, source, outcome string, latency time.Duration) {
	m.RequestsTotal.Inc(tenant, workspace, asset, source, outcome)
	m.Latency.Observe(latency.Seconds(), tenant, asset, source)
}
