package runtime

import (
	"errors"
	"sync"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// ErrDropConcurrency is returned by StartWorkflow when a trigger with
// concurrency=drop fires while a prior execution for the same
// (workflow_id, trigger_id) pair is still in flight. The HTTP layer
// translates this into a 409 response so trigger-router can emit
// workflow.trigger.dropped.v1 with the correct correlation id.
var ErrDropConcurrency = errors.New("drop_concurrency")

// concurrencyKey identifies a unique (workflow_id, trigger_id) lane.
// Version is intentionally omitted — concurrency policy applies across
// versions of the same trigger.
type concurrencyKey struct {
	WorkflowID string
	TriggerID  string
}

// concurrencyTracker enforces per-(workflow, trigger) concurrency
// policy. It is goroutine-safe and used by StartWorkflow before
// dispatching the execution goroutine. Each Acquire/Release pair is
// owned by exactly one in-flight execution.
type concurrencyTracker struct {
	mu      sync.Mutex
	cond    *sync.Cond
	holders map[concurrencyKey]int
}

func newConcurrencyTracker() *concurrencyTracker {
	t := &concurrencyTracker{holders: map[concurrencyKey]int{}}
	t.cond = sync.NewCond(&t.mu)
	return t
}

// Acquire applies the requested policy to a (workflow, trigger) lane.
// Returns ErrDropConcurrency when policy=drop and the lane is busy.
// Blocks until the lane is free when policy=queue. Always succeeds for
// policy=overlap or an empty policy.
//
// Caller MUST call Release exactly once after the execution completes.
func (t *concurrencyTracker) Acquire(key concurrencyKey, policy ast.TriggerConcurrency) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch policy {
	case "", ast.TriggerConcurrencyOverlap:
		t.holders[key]++
		return nil
	case ast.TriggerConcurrencyDrop:
		if t.holders[key] > 0 {
			return ErrDropConcurrency
		}
		t.holders[key]++
		return nil
	case ast.TriggerConcurrencyQueue:
		for t.holders[key] > 0 {
			t.cond.Wait()
		}
		t.holders[key]++
		return nil
	default:
		// Unknown policy — treat as overlap. The lint layer rejects
		// unknown values at publish time.
		t.holders[key]++
		return nil
	}
}

// Release marks the lane as freed by one in-flight execution and wakes
// any goroutines waiting in queue policy.
func (t *concurrencyTracker) Release(key concurrencyKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.holders[key] > 0 {
		t.holders[key]--
	}
	if t.holders[key] == 0 {
		delete(t.holders, key)
	}
	t.cond.Broadcast()
}

// InFlight returns the current count of in-flight executions for the lane.
// Exposed for tests + observability.
func (t *concurrencyTracker) InFlight(key concurrencyKey) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.holders[key]
}

// lookupTriggerPolicy returns the concurrency policy declared on the
// workflow's trigger with the matching id, or the default ("queue")
// when not found. The runtime resolves policy at start time from the
// workflow's spec; the trigger-router doesn't carry it in the event.
func lookupTriggerPolicy(wf *ast.Workflow, triggerID string) ast.TriggerConcurrency {
	if wf == nil {
		return ast.TriggerConcurrencyQueue
	}
	for _, t := range wf.Spec.Triggers {
		if t.ID == triggerID {
			return t.ConcurrencyOrDefault()
		}
	}
	return ast.TriggerConcurrencyQueue
}
