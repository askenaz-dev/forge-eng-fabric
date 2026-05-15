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

// Prometheus-text exposition for the A2A gateway. Metric series carry the
// `source` label with values `internal | external_proxy | inbound_external`
// so `per-asset-observability` can roll up per direction per asset.

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

func (c *counterVec) Inc(values ...string) {
	if len(values) != len(c.labels) {
		return
	}
	k := strings.Join(values, "\x1f")
	c.mu.Lock()
	v, ok := c.data[k]
	if !ok {
		var z uint64
		v = &z
		c.data[k] = v
	}
	c.mu.Unlock()
	atomic.AddUint64(v, 1)
}

func (c *counterVec) writeTo(sb *strings.Builder) {
	fmt.Fprintf(sb, "# HELP %s %s\n# TYPE %s counter\n", c.name, c.help, c.name)
	c.mu.Lock()
	keys := make([]string, 0, len(c.data))
	for k := range c.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := atomic.LoadUint64(c.data[k])
		parts := strings.Split(k, "\x1f")
		sb.WriteString(c.name)
		if len(c.labels) > 0 {
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

type histogramVec struct {
	name    string
	help    string
	labels  []string
	buckets []float64
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

func (h *histogramVec) Observe(v float64, vals ...string) {
	if len(vals) != len(h.labels) {
		return
	}
	k := strings.Join(vals, "\x1f")
	h.mu.Lock()
	s, ok := h.series[k]
	if !ok {
		s = &histSeries{counts: make([]uint64, len(h.buckets))}
		h.series[k] = s
	}
	for i, b := range h.buckets {
		if v <= b {
			s.counts[i]++
		}
	}
	s.sum += v
	s.count++
	h.mu.Unlock()
}

func (h *histogramVec) writeTo(sb *strings.Builder) {
	fmt.Fprintf(sb, "# HELP %s %s\n# TYPE %s histogram\n", h.name, h.help, h.name)
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

type Metrics struct {
	TasksStarted   *counterVec
	TasksCompleted *counterVec
	TasksFailed    *counterVec
	BudgetBlocks   *counterVec
	Latency        *histogramVec
}

func newMetrics() *Metrics {
	return &Metrics{
		TasksStarted: newCounterVec("a2a_gateway_tasks_started_total",
			"Total A2A tasks accepted by the gateway",
			[]string{"tenant", "workspace", "asset", "source", "method"}),
		TasksCompleted: newCounterVec("a2a_gateway_tasks_completed_total",
			"Total A2A tasks that returned a successful response",
			[]string{"tenant", "asset", "source"}),
		TasksFailed: newCounterVec("a2a_gateway_tasks_failed_total",
			"Total A2A tasks that failed at any stage",
			[]string{"tenant", "asset", "source", "reason"}),
		BudgetBlocks: newCounterVec("a2a_gateway_budget_blocks_total",
			"Total A2A tasks refused by Tenant-budget check",
			[]string{"tenant", "asset"}),
		Latency: newHistogramVec("a2a_gateway_latency_seconds",
			"End-to-end latency in seconds for A2A tasks",
			[]string{"tenant", "asset", "source", "method"},
			[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}),
	}
}

func (m *Metrics) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var sb strings.Builder
		m.TasksStarted.writeTo(&sb)
		m.TasksCompleted.writeTo(&sb)
		m.TasksFailed.writeTo(&sb)
		m.BudgetBlocks.writeTo(&sb)
		m.Latency.writeTo(&sb)
		w.Header().Set("content-type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(sb.String()))
	}
}

func (m *Metrics) observe(tenant, workspace, asset, source, method, outcome string, latency time.Duration) {
	m.TasksStarted.Inc(tenant, workspace, asset, source, method)
	m.Latency.Observe(latency.Seconds(), tenant, asset, source, method)
	if outcome == "ok" {
		m.TasksCompleted.Inc(tenant, asset, source)
	} else {
		m.TasksFailed.Inc(tenant, asset, source, outcome)
	}
}
