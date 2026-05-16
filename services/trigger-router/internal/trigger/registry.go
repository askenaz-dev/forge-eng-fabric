// Package trigger implements the in-memory registry of active triggers
// per (workflow_id, version) and the dispatch path that fires executions
// against workflow-runtime.
//
// The registry is fed by subscribing to workflow.published.v1 events from
// workflow-registry; each published workflow's spec.triggers entries are
// indexed for routing. Lookups are O(1) on (workflow_id, trigger_id).
package trigger

import (
	"sync"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// Subscription holds the metadata trigger-router needs to dispatch to
// workflow-runtime when a trigger fires. The registry stores one entry
// per (tenant, workspace, workflow, version, trigger_id).
type Subscription struct {
	TenantID    string
	WorkspaceID string
	WorkflowID  string
	Version     string
	TriggerID   string
	Type        ast.TriggerType
	Config      map[string]any
	Concurrency ast.TriggerConcurrency
	RegisteredAt time.Time
}

// Key identifies a single subscription. The registry is keyed by
// (workflow_id, trigger_id, version) — the same trigger_id can be
// registered against multiple versions (cutover overlap).
type Key struct {
	WorkflowID string
	TriggerID  string
	Version    string
}

// Registry indexes active triggers. Goroutine-safe.
type Registry struct {
	mu   sync.RWMutex
	subs map[Key]Subscription
	byType map[ast.TriggerType]map[Key]struct{} // secondary index for cron/email scanners
}

// NewRegistry constructs an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		subs:   map[Key]Subscription{},
		byType: map[ast.TriggerType]map[Key]struct{}{},
	}
}

// IngestWorkflow indexes every trigger declared on the workflow. Called
// from the workflow.published.v1 event handler. Idempotent.
func (r *Registry) IngestWorkflow(tenantID, workspaceID string, wf *ast.Workflow) []Subscription {
	if wf == nil {
		return nil
	}
	added := []Subscription{}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range wf.Spec.Triggers {
		k := Key{WorkflowID: wf.Metadata.ID, TriggerID: t.ID, Version: wf.Metadata.Version}
		sub := Subscription{
			TenantID:     tenantID,
			WorkspaceID:  workspaceID,
			WorkflowID:   wf.Metadata.ID,
			Version:      wf.Metadata.Version,
			TriggerID:    t.ID,
			Type:         t.Type,
			Config:       t.Config,
			Concurrency:  t.ConcurrencyOrDefault(),
			RegisteredAt: time.Now(),
		}
		r.subs[k] = sub
		if r.byType[t.Type] == nil {
			r.byType[t.Type] = map[Key]struct{}{}
		}
		r.byType[t.Type][k] = struct{}{}
		added = append(added, sub)
	}
	return added
}

// RemoveWorkflowVersion removes every subscription for a given workflow
// version. Called when a version is unpinned or deleted.
func (r *Registry) RemoveWorkflowVersion(workflowID, version string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, sub := range r.subs {
		if k.WorkflowID == workflowID && k.Version == version {
			delete(r.subs, k)
			if set, ok := r.byType[sub.Type]; ok {
				delete(set, k)
			}
		}
	}
}

// Lookup returns the subscription for (workflow_id, trigger_id, version).
// When version is empty the lookup returns any version, preferring the
// most recently registered (best-effort, see callers).
func (r *Registry) Lookup(workflowID, triggerID, version string) (Subscription, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if version != "" {
		s, ok := r.subs[Key{workflowID, triggerID, version}]
		return s, ok
	}
	var latest Subscription
	var ok bool
	for k, s := range r.subs {
		if k.WorkflowID == workflowID && k.TriggerID == triggerID {
			if !ok || s.RegisteredAt.After(latest.RegisteredAt) {
				latest, ok = s, true
			}
		}
	}
	return latest, ok
}

// ByType returns every subscription of a given trigger type. Used by the
// cron, event-bus, and email scanners.
func (r *Registry) ByType(t ast.TriggerType) []Subscription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []Subscription{}
	for k := range r.byType[t] {
		if s, ok := r.subs[k]; ok {
			out = append(out, s)
		}
	}
	return out
}

// All returns every subscription. For tests and admin endpoints.
func (r *Registry) All() []Subscription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Subscription, 0, len(r.subs))
	for _, s := range r.subs {
		out = append(out, s)
	}
	return out
}
