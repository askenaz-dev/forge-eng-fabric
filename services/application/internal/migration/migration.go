// Package migration implements the spec→app backfill + orphan-deletion
// classifier used by the spec-app-migration CLI. The package is independent
// of Postgres so it can be unit-tested against in-memory fixtures; the CLI
// is the production caller that loads real rows.
package migration

import (
	"context"
	"errors"
	"time"
)

// Classification is the per-spec verdict produced by the dry-run.
type Classification string

const (
	ClassRetainWithTargetApp Classification = "retain_with_target_app"
	ClassRetainUnassigned    Classification = "retain_unassigned"
	ClassOrphan              Classification = "orphan"
)

// SpecRow is the offline fixture format. Production loads these from
// openspec_index joined against onboarding_request, asset_deployment,
// runtime and workspace pinned-set tables.
type SpecRow struct {
	SpecID                string    `json:"spec_id"`
	WorkspaceID           string    `json:"workspace_id"`
	LifecycleStatus       string    `json:"lifecycle_status"`
	HasActiveDeployment   bool      `json:"has_active_deployment"`
	HasOnboardingInFlight bool      `json:"has_onboarding_in_flight"`
	HasRuntimeRef         bool      `json:"has_runtime_ref"`
	InPinnedSet           bool      `json:"in_pinned_set"`
	LastActivity          time.Time `json:"last_activity"`
	TargetAppCandidate    string    `json:"target_app_candidate,omitempty"`
}

// ClassificationResult is the verdict for one spec. Mirrors the columns of
// the dry-run CSV emitted by the CLI.
type ClassificationResult struct {
	SpecID         string         `json:"spec_id"`
	Classification Classification `json:"classification"`
	TargetAppID    string         `json:"target_app_id,omitempty"`
	LastActivity   time.Time      `json:"last_activity"`
	Evidence       map[string]any `json:"evidence"`
}

// Classify applies the per-spec orphan rule documented in design Decision 4:
//
//	orphan iff
//	  no active deployment AND
//	  no live onboarding AND
//	  no runtime ref AND
//	  not in pinned set/dashboard/Alfred conv within 90 days AND
//	  lifecycle in {proposed, draft}
//
// Specs with a clear App candidate retain into `retain_with_target_app`;
// the rest of the retainable specs go to `retain_unassigned`.
func Classify(rows []SpecRow, now time.Time) []ClassificationResult {
	threshold := now.Add(-90 * 24 * time.Hour)
	out := make([]ClassificationResult, 0, len(rows))
	for _, row := range rows {
		evidence := map[string]any{
			"has_active_deployment":    row.HasActiveDeployment,
			"has_onboarding_in_flight": row.HasOnboardingInFlight,
			"has_runtime_ref":          row.HasRuntimeRef,
			"in_pinned_set":            row.InPinnedSet,
			"lifecycle_status":         row.LifecycleStatus,
			"recent_activity":          row.LastActivity.After(threshold),
		}
		retainable := row.HasActiveDeployment || row.HasOnboardingInFlight || row.HasRuntimeRef ||
			row.InPinnedSet || row.LifecycleStatus == "approved" || row.LifecycleStatus == "committed"
		if !retainable && row.LastActivity.After(threshold) {
			// Recent activity within 90 days disqualifies it from the orphan
			// bucket even when no other evidence is present.
			retainable = true
		}
		switch {
		case !retainable && (row.LifecycleStatus == "proposed" || row.LifecycleStatus == "draft"):
			out = append(out, ClassificationResult{
				SpecID:         row.SpecID,
				Classification: ClassOrphan,
				LastActivity:   row.LastActivity,
				Evidence:       evidence,
			})
		case row.TargetAppCandidate != "":
			out = append(out, ClassificationResult{
				SpecID:         row.SpecID,
				Classification: ClassRetainWithTargetApp,
				TargetAppID:    row.TargetAppCandidate,
				LastActivity:   row.LastActivity,
				Evidence:       evidence,
			})
		default:
			out = append(out, ClassificationResult{
				SpecID:         row.SpecID,
				Classification: ClassRetainUnassigned,
				LastActivity:   row.LastActivity,
				Evidence:       evidence,
			})
		}
	}
	return out
}

// ExecutionPlan is the per-spec action list that `execute` applies to live
// data: backfill or hard-delete, in deterministic order.
type ExecutionPlan struct {
	Backfill []ClassificationResult
	Purge    []ClassificationResult
	Actor    string
}

func PlanExecution(report []ClassificationResult, actor string) (*ExecutionPlan, error) {
	if actor == "" {
		return nil, errors.New("actor is required")
	}
	plan := &ExecutionPlan{Actor: actor}
	for _, row := range report {
		switch row.Classification {
		case ClassOrphan:
			plan.Purge = append(plan.Purge, row)
		case ClassRetainWithTargetApp, ClassRetainUnassigned:
			plan.Backfill = append(plan.Backfill, row)
		}
	}
	return plan, nil
}

// EmittedEvent is the CloudEvents stub recorded by the CLI for every change
// it would publish in production.
type EmittedEvent struct {
	Type    string         `json:"type"`
	Subject string         `json:"subject"`
	Data    map[string]any `json:"data"`
}

// ExecutionResult summarises what Apply touched.
type ExecutionResult struct {
	Backfilled int
	Purged     int
}

// Apply walks the plan and emits the corresponding events. The function is
// production-quality only when wired to a real database adapter; here it is
// the offline rehearsal path that exercises the event shapes against the
// captured fixture.
func Apply(_ context.Context, plan *ExecutionPlan, emit func(eventType, subject string, data map[string]any)) ExecutionResult {
	if plan == nil {
		return ExecutionResult{}
	}
	for _, row := range plan.Backfill {
		emit("spec.reparented.v1", "spec/"+row.SpecID, map[string]any{
			"spec_id":     row.SpecID,
			"from_app_id": nil,
			"to_app_id":   row.TargetAppID,
			"principal":   plan.Actor,
			"reason":      "migration",
		})
	}
	for _, row := range plan.Purge {
		emit("spec.purged.v1", "spec/"+row.SpecID, map[string]any{
			"spec_id":   row.SpecID,
			"principal": plan.Actor,
			"reason":    "orphan",
			"evidence":  row.Evidence,
		})
	}
	return ExecutionResult{Backfilled: len(plan.Backfill), Purged: len(plan.Purge)}
}
