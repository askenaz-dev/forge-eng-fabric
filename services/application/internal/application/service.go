package application

import (
	"context"
	"fmt"
	"time"
)

// Service is the orchestrator for App lifecycle. It composes Store, Authorizer,
// EventSink, AuditSink, LiveArtefactChecker and WorkspaceLookup.
type Service struct {
	Store        *Store
	Authz        Authorizer
	Events       EventSink
	Audit        AuditSink
	LiveCheck    LiveArtefactChecker
	WorkspaceDir WorkspaceLookup
	// DesignSystem resolves a `design_system_ref` against the AI Asset
	// Registry. Optional — when nil, the service writes the ref through
	// without validation (backwards-compatible with services started before
	// design-system-catalog rolled out).
	DesignSystem DesignSystemResolver
	// Now is used for deterministic timestamps in tests.
	Now func() time.Time
}

func NewService(store *Store, authz Authorizer, events EventSink, audit AuditSink, live LiveArtefactChecker, dir WorkspaceLookup) *Service {
	return &Service{
		Store:        store,
		Authz:        authz,
		Events:       events,
		Audit:        audit,
		LiveCheck:    live,
		WorkspaceDir: dir,
		Now:          func() time.Time { return time.Now().UTC() },
	}
}

// Caller identifies the acting principal and correlation id for a request.
type Caller struct {
	Principal     string
	CorrelationID string
}

// Create validates the input and persists a new App. Application invariants:
// - Slug must match the slug grammar.
// - Slug must not be reserved (no `_unassigned` via this path).
// - Caller must have workspace#editor on the parent workspace.
// - At least one owner.
//
// The system-managed bootstrap path uses CreateUnassigned, which bypasses
// the reserved-slug check.
func (s *Service) Create(ctx context.Context, caller Caller, workspaceID string, in CreateInput) (*App, error) {
	if caller.Principal == SystemActor {
		return nil, fmt.Errorf("%w: system principal cannot use public create", ErrForbidden)
	}
	if IsReservedSlug(in.Slug) {
		return nil, ErrSlugReserved
	}
	if err := ValidateSlug(in.Slug); err != nil {
		return nil, err
	}
	if in.Name == "" {
		return nil, ErrMissingName
	}
	if len(in.Owners) == 0 {
		return nil, ErrMissingOwners
	}
	allowed, err := s.Authz.Check(ctx, principalToFGAUser(caller.Principal), string(PermEditor), "workspace:"+workspaceID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	tenantID, err := s.WorkspaceDir.TenantID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.insert(ctx, caller, workspaceID, tenantID, in, false)
}

// CreateUnassigned is the bootstrap path used by the workspace-bootstrap
// pipeline to materialise the `_unassigned` App for a new workspace. The
// caller MUST be the system actor; the function rejects all other principals.
func (s *Service) CreateUnassigned(ctx context.Context, caller Caller, workspaceID, tenantID string) (*App, error) {
	if caller.Principal != SystemActor {
		return nil, ErrForbidden
	}
	in := CreateInput{
		Slug:        UnassignedSlug,
		Name:        "Unassigned",
		Description: "System-managed bucket for unassigned specs",
		Owners:      []string{SystemActor},
	}
	return s.insert(ctx, caller, workspaceID, tenantID, in, true)
}

func (s *Service) insert(ctx context.Context, caller Caller, workspaceID, tenantID string, in CreateInput, systemManaged bool) (*App, error) {
	if in.DefaultEnvironments == nil {
		in.DefaultEnvironments = []string{}
	}
	if in.RepoLinks == nil {
		in.RepoLinks = []string{}
	}
	if in.RuntimeLinks == nil {
		in.RuntimeLinks = []string{}
	}
	// design-system-catalog + alfred-design-system-picker (D3, D4): every App
	// carries a design_system_ref. Default to `ds-forge-default` when omitted;
	// validate the resolved asset is approved AND visible to the App's tenant.
	// System-managed Apps (e.g. `_unassigned`) skip validation to keep the
	// workspace bootstrap path independent of the Registry's readiness.
	if in.DesignSystemRef == "" {
		in.DesignSystemRef = DesignSystemDefaultRef
	}
	if !systemManaged && s.DesignSystem != nil {
		rec, err := s.DesignSystem.Resolve(ctx, in.DesignSystemRef, tenantID)
		if err != nil {
			return nil, err
		}
		if rec.LifecycleState != "approved" {
			return nil, ErrDesignSystemNotApproved
		}
		// Visibility check: tenant_global and built-in templates are visible
		// to every tenant. Tenant- or workspace-scoped entries MUST belong to
		// the caller's tenant.
		if !rec.BuiltInTemplate && rec.Visibility != "tenant_global" && rec.TenantID != "" && rec.TenantID != tenantID {
			return nil, ErrDesignSystemNotVisible
		}
	}
	app := &App{
		ID:                    newID(),
		Slug:                  in.Slug,
		Name:                  in.Name,
		Description:           in.Description,
		WorkspaceID:           workspaceID,
		TenantID:              tenantID,
		Lifecycle:             LifecycleActive,
		DesignSystemRef:       in.DesignSystemRef,
		DesignSystemOverrides: map[string]string{},
		DefaultEnvironments:   in.DefaultEnvironments,
		RepoLinks:             in.RepoLinks,
		RuntimeLinks:          in.RuntimeLinks,
		Owners:                append([]string(nil), in.Owners...),
		Targets:               DefaultTargets(),
		SystemManaged:         systemManaged,
		CreatedAt:             s.Now(),
		CreatedBy:             caller.Principal,
	}
	app.UpdatedAt = app.CreatedAt
	stored, err := s.Store.Insert(app)
	if err != nil {
		return nil, err
	}
	// Seed OpenFGA tuples: parent edge (workspace -> app) + one tuple per owner.
	// Best-effort: a tuple-write failure does not unwind storage. The migration
	// job's reconciler closes any drift.
	_ = s.Authz.WriteTuple(ctx, "workspace:"+workspaceID, "parent", "app:"+stored.ID)
	for _, owner := range stored.Owners {
		_ = s.Authz.WriteTuple(ctx, principalToFGAUser(owner), string(PermOwner), "app:"+stored.ID)
	}
	// alfred-design-system-picker (D3): emit `app.created.v1` with the picker
	// flag in Extra so subscribers can branch. System-managed bootstrap inserts
	// (`_unassigned`) bypass the picker concept entirely — emit chosen=true
	// so the skip event does not fire for the platform's own writes.
	chosenExplicitly := in.DesignSystemChosenExplicitly
	if systemManaged {
		chosenExplicitly = true
	}
	createdAt := s.Now()
	_ = s.Events.Publish(ctx, Event{
		Type:          EventAppCreated,
		AppID:         stored.ID,
		WorkspaceID:   stored.WorkspaceID,
		TenantID:      stored.TenantID,
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		After:         stored,
		Extra: map[string]any{
			"design_system_ref":              stored.DesignSystemRef,
			"design_system_chosen_explicitly": chosenExplicitly,
		},
		OccurredAt: createdAt,
	})
	_ = s.Audit.Record(ctx, AuditRecord{
		AppID:         stored.ID,
		WorkspaceID:   stored.WorkspaceID,
		TenantID:      stored.TenantID,
		Action:        "created",
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		After:         stored,
		Evidence: map[string]any{
			"design_system_chosen_explicitly": chosenExplicitly,
		},
		CreatedAt: createdAt,
	})
	// alfred-design-system-picker (D3): when the picker recorded a skip (or
	// the caller did not opt in), emit a sibling event so observability can
	// measure catalog discoverability without parsing every `app.created.v1`.
	// Shares the same correlation_id and timestamp as `app.created.v1`.
	if !chosenExplicitly {
		_ = s.Events.Publish(ctx, Event{
			Type:          EventDesignSystemUserSkipped,
			AppID:         stored.ID,
			WorkspaceID:   stored.WorkspaceID,
			TenantID:      stored.TenantID,
			Actor:         caller.Principal,
			CorrelationID: caller.CorrelationID,
			Extra: map[string]any{
				"resolved_ref": stored.DesignSystemRef,
			},
			OccurredAt: createdAt,
		})
	}
	return stored, nil
}

// Get returns the App if the caller has app#viewer. ErrForbidden if not.
func (s *Service) Get(ctx context.Context, caller Caller, id string) (*App, error) {
	app, err := s.Store.Get(id)
	if err != nil {
		return nil, err
	}
	allowed, err := s.Authz.Check(ctx, principalToFGAUser(caller.Principal), string(PermView), "app:"+id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	return app, nil
}

// List returns Apps in the workspace visible to the caller. Visibility is
// enforced per-row via Authz.Check; the system-managed `_unassigned` App is
// always included for workspace#viewer principals (it carries
// `system_managed=true`).
func (s *Service) List(ctx context.Context, caller Caller, workspaceID string, includeArchived bool) ([]*App, error) {
	allowed, err := s.Authz.Check(ctx, principalToFGAUser(caller.Principal), string(PermViewer), "workspace:"+workspaceID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	return s.Store.List(Filter{WorkspaceID: workspaceID, IncludeArchived: includeArchived}), nil
}

// Patch updates mutable fields. Patch on a system-managed App is refused.
func (s *Service) Patch(ctx context.Context, caller Caller, id string, in PatchInput) (*App, error) {
	existing, err := s.Store.Get(id)
	if err != nil {
		return nil, err
	}
	if existing.SystemManaged || existing.Slug == UnassignedSlug {
		return nil, ErrSystemManaged
	}
	allowed, err := s.Authz.Check(ctx, principalToFGAUser(caller.Principal), string(PermEdit), "app:"+id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	before := *existing
	updated, err := s.Store.Update(id, func(app *App) error {
		if in.Name != nil {
			if *in.Name == "" {
				return ErrMissingName
			}
			app.Name = *in.Name
		}
		if in.Description != nil {
			app.Description = *in.Description
		}
		if in.Owners != nil {
			if len(*in.Owners) == 0 {
				return ErrMissingOwners
			}
			app.Owners = append([]string(nil), (*in.Owners)...)
		}
		if in.DesignSystemRef != nil {
			candidate := *in.DesignSystemRef
			if candidate == "" {
				candidate = DesignSystemDefaultRef
			}
			if !existing.SystemManaged && s.DesignSystem != nil {
				rec, rerr := s.DesignSystem.Resolve(ctx, candidate, app.TenantID)
				if rerr != nil {
					return rerr
				}
				if rec.LifecycleState != "approved" {
					return ErrDesignSystemNotApproved
				}
			}
			app.DesignSystemRef = candidate
		}
		if in.DefaultEnvironments != nil {
			app.DefaultEnvironments = append([]string(nil), (*in.DefaultEnvironments)...)
		}
		if in.RepoLinks != nil {
			app.RepoLinks = append([]string(nil), (*in.RepoLinks)...)
		}
		if in.RuntimeLinks != nil {
			app.RuntimeLinks = append([]string(nil), (*in.RuntimeLinks)...)
		}
		if in.Targets != nil {
			if err := ValidateTargets(in.Targets); err != nil {
				return err
			}
			if app.Targets == nil {
				app.Targets = DefaultTargets()
			}
			for phase, val := range in.Targets {
				app.Targets[phase] = val
			}
		}
		app.UpdatedBy = caller.Principal
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.Events.Publish(ctx, Event{
		Type:          EventAppUpdated,
		AppID:         updated.ID,
		WorkspaceID:   updated.WorkspaceID,
		TenantID:      updated.TenantID,
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        &before,
		After:         updated,
		OccurredAt:    s.Now(),
	})
	_ = s.Audit.Record(ctx, AuditRecord{
		AppID:         updated.ID,
		WorkspaceID:   updated.WorkspaceID,
		TenantID:      updated.TenantID,
		Action:        "updated",
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        &before,
		After:         updated,
		CreatedAt:     s.Now(),
	})
	return updated, nil
}

// Archive moves an App to lifecycle_state=archived.
func (s *Service) Archive(ctx context.Context, caller Caller, id string) (*App, error) {
	existing, err := s.Store.Get(id)
	if err != nil {
		return nil, err
	}
	if existing.SystemManaged || existing.Slug == UnassignedSlug {
		return nil, ErrSystemManaged
	}
	if existing.Lifecycle == LifecycleArchived {
		return existing, nil
	}
	allowed, err := s.Authz.Check(ctx, principalToFGAUser(caller.Principal), string(PermEdit), "app:"+id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	before := *existing
	updated, err := s.Store.Update(id, func(app *App) error {
		app.Lifecycle = LifecycleArchived
		now := s.Now()
		app.ArchivedAt = &now
		app.UpdatedBy = caller.Principal
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.Events.Publish(ctx, Event{
		Type:          EventAppArchived,
		AppID:         updated.ID,
		WorkspaceID:   updated.WorkspaceID,
		TenantID:      updated.TenantID,
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        &before,
		After:         updated,
		OccurredAt:    s.Now(),
	})
	_ = s.Audit.Record(ctx, AuditRecord{
		AppID:         updated.ID,
		WorkspaceID:   updated.WorkspaceID,
		TenantID:      updated.TenantID,
		Action:        "archived",
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        &before,
		After:         updated,
		CreatedAt:     s.Now(),
	})
	return updated, nil
}

// Restore moves an archived App back to active.
func (s *Service) Restore(ctx context.Context, caller Caller, id string) (*App, error) {
	existing, err := s.Store.Get(id)
	if err != nil {
		return nil, err
	}
	if existing.SystemManaged || existing.Slug == UnassignedSlug {
		return nil, ErrSystemManaged
	}
	if existing.Lifecycle != LifecycleArchived {
		return existing, nil
	}
	allowed, err := s.Authz.Check(ctx, principalToFGAUser(caller.Principal), string(PermEdit), "app:"+id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	before := *existing
	updated, err := s.Store.Update(id, func(app *App) error {
		app.Lifecycle = LifecycleActive
		app.ArchivedAt = nil
		app.UpdatedBy = caller.Principal
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.Events.Publish(ctx, Event{
		Type:          EventAppRestored,
		AppID:         updated.ID,
		WorkspaceID:   updated.WorkspaceID,
		TenantID:      updated.TenantID,
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        &before,
		After:         updated,
		OccurredAt:    s.Now(),
	})
	_ = s.Audit.Record(ctx, AuditRecord{
		AppID:         updated.ID,
		WorkspaceID:   updated.WorkspaceID,
		TenantID:      updated.TenantID,
		Action:        "restored",
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        &before,
		After:         updated,
		CreatedAt:     s.Now(),
	})
	return updated, nil
}

// DeleteResult is returned to handlers on DELETE so they can communicate the
// list of blocking artefacts when the deletion is refused.
type DeleteResult struct {
	Deleted *App
	Blocked *LiveArtefactReport
}

// Delete hard-deletes an App after verifying there are no live artefacts.
func (s *Service) Delete(ctx context.Context, caller Caller, id string) (*DeleteResult, error) {
	existing, err := s.Store.Get(id)
	if err != nil {
		return nil, err
	}
	if existing.SystemManaged || existing.Slug == UnassignedSlug {
		return nil, ErrSystemManaged
	}
	allowed, err := s.Authz.Check(ctx, principalToFGAUser(caller.Principal), string(PermAdmin), "app:"+id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	report, err := s.LiveCheck.Check(ctx, id)
	if err != nil {
		return nil, err
	}
	if !report.Empty() {
		return &DeleteResult{Blocked: &report}, ErrAppHasLiveArtefacts
	}
	deleted, err := s.Store.Delete(id)
	if err != nil {
		return nil, err
	}
	_ = s.Events.Publish(ctx, Event{
		Type:          EventAppDeleted,
		AppID:         deleted.ID,
		WorkspaceID:   deleted.WorkspaceID,
		TenantID:      deleted.TenantID,
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        existing,
		OccurredAt:    s.Now(),
	})
	_ = s.Audit.Record(ctx, AuditRecord{
		AppID:         deleted.ID,
		WorkspaceID:   deleted.WorkspaceID,
		TenantID:      deleted.TenantID,
		Action:        "deleted",
		Actor:         caller.Principal,
		CorrelationID: caller.CorrelationID,
		Before:        existing,
		CreatedAt:     s.Now(),
	})
	return &DeleteResult{Deleted: deleted}, nil
}

// ValidateAppWorkspaceCoherence checks that the App's workspace matches the
// claimed workspace. Used by every downstream service that accepts an App
// reference alongside a workspace_id payload (OpenSpec backbone, asset
// registry, etc.).
func (s *Service) ValidateAppWorkspaceCoherence(ctx context.Context, appID, workspaceID string) error {
	app, err := s.Store.Get(appID)
	if err != nil {
		return err
	}
	if app.WorkspaceID != workspaceID {
		return ErrAppWorkspaceMismatch
	}
	if app.Lifecycle == LifecycleArchived {
		return ErrAppArchived
	}
	return nil
}

// principalToFGAUser maps the X-Forge-Principal header value (e.g. "alice" or
// "user:alice@acme.com") to the OpenFGA user string. Production hosts a
// dedicated identity broker; this is a placeholder that mirrors the
// convention used by services/control-plane/internal/auth.
func principalToFGAUser(principal string) string {
	if principal == "" {
		return "user:anonymous"
	}
	if len(principal) > 5 && principal[:5] == "user:" {
		return principal
	}
	if principal == SystemActor {
		return principal
	}
	return "user:" + principal
}
