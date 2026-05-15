package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// design-system-catalog event types emitted by the App service when an owner
// initiates a swap, an override changes or a swap PR merges and flips the
// App's `design_system_ref`. Subscribers correlate these against the App's
// audit trail.
const (
	EventDesignSystemSwapRequested    EventType = "app.design_system.swap_requested.v1"
	EventDesignSystemChanged          EventType = "app.design_system.changed.v1"
	EventDesignSystemOverrideChanged  EventType = "app.design_system.override_changed.v1"
)

// DesignSystemServiceDeps composes the side seams the design-system endpoints
// need. They are kept off the base Service struct to keep existing
// constructors lean; the wire-up code creates one DesignSystemService per
// process and registers its handlers alongside the rest.
type DesignSystemServiceDeps struct {
	Service  *Service
	Resolver DesignSystemResolver
	PR       PROpener
	PRStore  DesignSystemPRStore
}

// DesignSystemService implements the swap, overrides and webhook flows. The
// generic App service stays untouched (it already supports `design_system_ref`
// in the Patch input); this struct owns the additional endpoints.
type DesignSystemService struct {
	DesignSystemServiceDeps
}

// NewDesignSystemService wires the design-system endpoints. Callers that omit
// PR / PRStore get the in-memory mocks (suitable for dev and unit tests).
func NewDesignSystemService(deps DesignSystemServiceDeps) *DesignSystemService {
	if deps.PR == nil {
		deps.PR = &MockPROpener{}
	}
	if deps.PRStore == nil {
		deps.PRStore = NewMemoryDesignSystemPRStore()
	}
	return &DesignSystemService{DesignSystemServiceDeps: deps}
}

// ValidateDesignSystemRef resolves `ref` against the Registry and confirms the
// asset is approved and visible to the App's tenant. Returns the resolved
// asset_id (without alias indirection) so audit records carry the canonical
// pointer rather than the alias.
func (d *DesignSystemService) ValidateDesignSystemRef(ctx context.Context, tenantID, ref string) (DesignSystemRecord, error) {
	rec, err := d.Resolver.Resolve(ctx, ref, tenantID)
	if err != nil {
		return DesignSystemRecord{}, err
	}
	if rec.LifecycleState != "approved" {
		return rec, fmt.Errorf("%w: %s is in %s", ErrDesignSystemNotApproved, rec.AssetID, rec.LifecycleState)
	}
	return rec, nil
}

// Swap opens a PR against the App's portal-bundle repository to change the
// `design_system_ref`. Implements POST /v1/apps/{id}/design-system:swap.
func (d *DesignSystemService) Swap(ctx context.Context, caller Caller, appID string, in SwapInput) (*App, *DesignSystemPR, error) {
	if strings.TrimSpace(in.TargetRef) == "" {
		return nil, nil, fmt.Errorf("missing target_ref")
	}
	app, err := d.Service.Store.Get(appID)
	if err != nil {
		return nil, nil, err
	}
	if app.SystemManaged || app.Slug == UnassignedSlug {
		return nil, nil, ErrSystemManaged
	}
	allowed, err := d.Service.Authz.Check(ctx, principalToFGAUser(caller.Principal), string(PermOwner), "app:"+appID)
	if err != nil {
		return nil, nil, err
	}
	if !allowed {
		return nil, nil, ErrForbidden
	}
	rec, err := d.ValidateDesignSystemRef(ctx, app.TenantID, in.TargetRef)
	if err != nil {
		return nil, nil, err
	}
	repoURL, repoErr := portalBundleRepoOf(app)
	if repoErr != nil {
		return nil, nil, repoErr
	}
	prResult, err := d.PR.OpenSwapPR(ctx, SwapPRInput{
		AppID:         app.ID,
		AppSlug:       app.Slug,
		RepoURL:       repoURL,
		FromRef:       app.DesignSystemRef,
		TargetRef:     in.TargetRef,
		Reason:        in.Reason,
		OpenedBy:      caller.Principal,
		CorrelationID: caller.CorrelationID,
	})
	if err != nil {
		return nil, nil, err
	}
	now := d.Service.Now()
	row := DesignSystemPR{
		AppID:         app.ID,
		WorkspaceID:   app.WorkspaceID,
		TenantID:      app.TenantID,
		TargetRef:     in.TargetRef,
		Reason:        in.Reason,
		PRURL:         prResult.URL,
		Status:        "open",
		OpenedBy:      caller.Principal,
		OpenedAt:      now,
		CorrelationID: caller.CorrelationID,
	}
	// Mark any prior open PR as superseded, then record the new one.
	open, _ := d.PRStore.ListOpen(ctx, app.ID)
	for _, prior := range open {
		_ = d.PR.CloseSupersededPR(ctx, prior.PRURL, "Superseded by "+prResult.URL)
	}
	_ = d.PRStore.MarkSuperseded(ctx, app.ID, prResult.URL)
	if err := d.PRStore.OpenPR(ctx, row); err != nil {
		return nil, nil, err
	}
	_ = d.Service.Events.Publish(ctx, Event{
		Type:          EventDesignSystemSwapRequested,
		AppID:         app.ID,
		WorkspaceID:   app.WorkspaceID,
		TenantID:      app.TenantID,
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        app,
		OccurredAt:    now,
	})
	_ = d.Service.Audit.Record(ctx, AuditRecord{
		AppID:         app.ID,
		WorkspaceID:   app.WorkspaceID,
		TenantID:      app.TenantID,
		Action:        "updated",
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Reason:        "design_system_swap_requested",
		Before:        app,
		Evidence: map[string]any{
			"swap_pr_url": prResult.URL,
			"target_ref":  in.TargetRef,
			"resolved_to": rec.AssetID,
		},
		CreatedAt: now,
	})
	return app, &row, nil
}

// PatchOverrides implements PATCH /v1/apps/{id}/design-system/overrides.
func (d *DesignSystemService) PatchOverrides(ctx context.Context, caller Caller, appID string, in OverridesInput) (*App, error) {
	existing, err := d.Service.Store.Get(appID)
	if err != nil {
		return nil, err
	}
	if existing.SystemManaged || existing.Slug == UnassignedSlug {
		return nil, ErrSystemManaged
	}
	allowed, err := d.Service.Authz.Check(ctx, principalToFGAUser(caller.Principal), string(PermOwner), "app:"+appID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	if err := validateOverrides(in.Overrides); err != nil {
		return nil, err
	}
	for _, ref := range in.Overrides {
		if _, err := d.ValidateDesignSystemRef(ctx, existing.TenantID, ref); err != nil {
			return nil, err
		}
	}
	before := *existing
	updated, err := d.Service.Store.Update(appID, func(app *App) error {
		merged := map[string]string{}
		for k, v := range app.DesignSystemOverrides {
			merged[k] = v
		}
		for k, v := range in.Overrides {
			if v == "" {
				delete(merged, k)
			} else {
				merged[k] = v
			}
		}
		app.DesignSystemOverrides = merged
		app.UpdatedBy = caller.Principal
		return nil
	})
	if err != nil {
		return nil, err
	}
	now := d.Service.Now()
	_ = d.Service.Events.Publish(ctx, Event{
		Type:          EventDesignSystemOverrideChanged,
		AppID:         updated.ID,
		WorkspaceID:   updated.WorkspaceID,
		TenantID:      updated.TenantID,
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        &before,
		After:         updated,
		OccurredAt:    now,
	})
	_ = d.Service.Audit.Record(ctx, AuditRecord{
		AppID:         updated.ID,
		WorkspaceID:   updated.WorkspaceID,
		TenantID:      updated.TenantID,
		Action:        "updated",
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Reason:        "design_system_overrides_patched",
		Before:        &before,
		After:         updated,
		CreatedAt:     now,
	})
	return updated, nil
}

// HandleSwapPRMerged is invoked by the GitHub webhook listener on a merged
// swap PR. It flips the App's `design_system_ref` to the recorded target,
// marks the PR as merged and emits `app.design_system.changed.v1`. The
// caller (the webhook adapter) is responsible for verifying the webhook
// signature and the PR's merge state before calling this method.
func (d *DesignSystemService) HandleSwapPRMerged(ctx context.Context, caller Caller, prURL string) (*App, error) {
	pr, err := d.PRStore.GetByPRURL(ctx, prURL)
	if err != nil {
		return nil, err
	}
	if pr.Status != "open" {
		// Idempotent: ignore re-deliveries of the same merge event.
		return nil, nil
	}
	existing, err := d.Service.Store.Get(pr.AppID)
	if err != nil {
		return nil, err
	}
	before := *existing
	updated, err := d.Service.Store.Update(pr.AppID, func(app *App) error {
		app.DesignSystemRef = pr.TargetRef
		app.UpdatedBy = caller.Principal
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = d.PRStore.MarkMerged(ctx, prURL)
	now := d.Service.Now()
	_ = d.Service.Events.Publish(ctx, Event{
		Type:          EventDesignSystemChanged,
		AppID:         updated.ID,
		WorkspaceID:   updated.WorkspaceID,
		TenantID:      updated.TenantID,
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        &before,
		After:         updated,
		OccurredAt:    now,
	})
	_ = d.Service.Audit.Record(ctx, AuditRecord{
		AppID:         updated.ID,
		WorkspaceID:   updated.WorkspaceID,
		TenantID:      updated.TenantID,
		Action:        "updated",
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Reason:        "design_system_swap_merged",
		Before:        &before,
		After:         updated,
		Evidence:      map[string]any{"swap_pr_url": prURL, "from": before.DesignSystemRef, "to": pr.TargetRef},
		CreatedAt:     now,
	})
	return updated, nil
}

// portalBundleRepoOf picks the App's portal-bundle repo from the recorded
// repo_links. Convention: the first https-URL link is the portal-bundle repo.
// The wider RepoLinks structure (kind/tag) is deferred to a follow-up.
func portalBundleRepoOf(app *App) (string, error) {
	for _, link := range app.RepoLinks {
		if strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "git@") {
			return link, nil
		}
	}
	return "", ErrAppRepoMissing
}

// resolveTimes shaves Go pointer-friction in tests.
var _ = time.Time{}
var _ = errors.New
