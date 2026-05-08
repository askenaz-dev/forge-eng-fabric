package runtime

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Metrics holds Prometheus-style counters.
type Metrics struct {
	mu                  sync.Mutex
	executionsStarted   map[string]int
	executionsCompleted map[string]int
	stepFailures        map[string]int
	hitlTimeouts        int
	retries             int
}

// NewMetrics creates an empty metrics container.
func NewMetrics() *Metrics {
	return &Metrics{
		executionsStarted:   map[string]int{},
		executionsCompleted: map[string]int{},
		stepFailures:        map[string]int{},
	}
}

// IncStarted increments the started counter for a given status.
func (m *Metrics) IncStarted(status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executionsStarted[status]++
}

// IncCompleted increments the completed counter for a given status.
func (m *Metrics) IncCompleted(status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executionsCompleted[status]++
}

// IncStepFailure increments per-step failure counter.
func (m *Metrics) IncStepFailure(stepType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stepFailures[stepType]++
}

// IncHITLTimeout increments the HITL timeout counter.
func (m *Metrics) IncHITLTimeout() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hitlTimeouts++
}

// IncRetry increments the retry counter.
func (m *Metrics) IncRetry() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retries++
}

// Render returns a Prometheus exposition format string.
func (m *Metrics) Render() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var b strings.Builder
	b.WriteString("# HELP workflow_executions_started_total Total workflow executions started\n")
	b.WriteString("# TYPE workflow_executions_started_total counter\n")
	for _, k := range sortedKeys(m.executionsStarted) {
		fmt.Fprintf(&b, "workflow_executions_started_total{status=%q} %d\n", k, m.executionsStarted[k])
	}
	b.WriteString("# HELP workflow_executions_completed_total Total workflow executions completed\n")
	b.WriteString("# TYPE workflow_executions_completed_total counter\n")
	for _, k := range sortedKeys(m.executionsCompleted) {
		fmt.Fprintf(&b, "workflow_executions_completed_total{status=%q} %d\n", k, m.executionsCompleted[k])
	}
	b.WriteString("# HELP workflow_step_failure_total Total step failures by type\n")
	b.WriteString("# TYPE workflow_step_failure_total counter\n")
	for _, k := range sortedKeys(m.stepFailures) {
		fmt.Fprintf(&b, "workflow_step_failure_total{type=%q} %d\n", k, m.stepFailures[k])
	}
	fmt.Fprintf(&b, "# HELP human_in_loop_timeout_total HITL timeouts\n# TYPE human_in_loop_timeout_total counter\nhuman_in_loop_timeout_total %d\n", m.hitlTimeouts)
	fmt.Fprintf(&b, "# HELP workflow_retried_total Step retries\n# TYPE workflow_retried_total counter\nworkflow_retried_total %d\n", m.retries)
	return b.String()
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
