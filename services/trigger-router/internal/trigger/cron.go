package trigger

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
	cronlib "github.com/robfig/cron/v3"
)

// CronScheduler holds an internal robfig/cron instance and (re-)schedules
// entries whenever the registry changes.
//
// Production wiring uses Temporal cron-workflow primitives for the same
// behavior with HA semantics; this in-process implementation is the dev
// fallback. The two implementations are interchangeable behind the
// Scheduler interface.
type CronScheduler struct {
	Registry   *Registry
	Dispatcher *Dispatcher

	mu       sync.Mutex
	cron     *cronlib.Cron
	entries  map[Key]cronlib.EntryID
}

// NewCronScheduler builds a scheduler ready to receive Refresh calls.
func NewCronScheduler(reg *Registry, dispatch *Dispatcher) *CronScheduler {
	return &CronScheduler{
		Registry:   reg,
		Dispatcher: dispatch,
		cron:       cronlib.New(cronlib.WithSeconds()),
		entries:    map[Key]cronlib.EntryID{},
	}
}

// Start begins the underlying cron loop. Idempotent.
func (s *CronScheduler) Start() {
	s.cron.Start()
}

// Stop halts the cron loop, waiting for any in-flight invocation.
func (s *CronScheduler) Stop() context.Context {
	return s.cron.Stop()
}

// Refresh re-syncs scheduled entries with the registry. Called after
// each IngestWorkflow / RemoveWorkflowVersion. Safe to call frequently
// — entry diffs are computed under the lock.
func (s *CronScheduler) Refresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	desired := map[Key]Subscription{}
	for _, sub := range s.Registry.ByType(ast.TriggerCron) {
		desired[Key{sub.WorkflowID, sub.TriggerID, sub.Version}] = sub
	}
	// Removals.
	for k, eid := range s.entries {
		if _, ok := desired[k]; !ok {
			s.cron.Remove(eid)
			delete(s.entries, k)
		}
	}
	// Additions.
	for k, sub := range desired {
		if _, ok := s.entries[k]; ok {
			continue
		}
		expr, _ := sub.Config["expression"].(string)
		if expr == "" {
			log.Printf("cron: subscription %v missing config.expression", k)
			continue
		}
		// Honor timezone if present by prefixing CRON_TZ=…
		if tz, _ := sub.Config["timezone"].(string); tz != "" {
			expr = fmt.Sprintf("CRON_TZ=%s %s", tz, expr)
		}
		sub := sub
		id, err := s.cron.AddFunc(expr, func() {
			ctx := context.Background()
			_, ferr := s.Dispatcher.Fire(ctx, sub, map[string]any{
				"fired_at": time.Now().UTC(),
				"source":   "cron",
			})
			if ferr != nil {
				log.Printf("cron dispatch failed for %v: %v", k, ferr)
			}
		})
		if err != nil {
			log.Printf("cron schedule failed for %v: %v", k, err)
			continue
		}
		s.entries[k] = id
	}
}

// ScheduledCount returns how many cron entries are currently live.
// Used by tests + the admin endpoint.
func (s *CronScheduler) ScheduledCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}
