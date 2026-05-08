package marketplace

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
	"github.com/google/uuid"
)

// Service provides marketplace operations.
type Service struct {
	mu       sync.RWMutex
	listings map[string]*Listing
	installs map[string]*Install
	now      func() time.Time
	sink     Sink
}

// NewService creates an empty service.
func NewService(sink Sink) *Service {
	if sink == nil {
		sink = &MemorySink{}
	}
	return &Service{
		listings: map[string]*Listing{},
		installs: map[string]*Install{},
		now:      func() time.Time { return time.Now().UTC() },
		sink:     sink,
	}
}

// Publish creates a marketplace listing or moves a workflow version up the
// visibility ladder. Promotion to `tenant` requires Tenant admin approval;
// promotion to `forge-certified` requires eval pass + security review.
func (s *Service) Publish(_ context.Context, req PublishRequest) (*Listing, error) {
	if req.WorkflowID == "" || req.Version == "" {
		return nil, errors.New("workflow_id_and_version_required")
	}
	if req.Visibility == "" {
		req.Visibility = ast.VisibilityWorkspace
	}
	state := ApprovalNotRequired
	switch req.Visibility {
	case ast.VisibilityTenant:
		state = ApprovalPending
	case ast.VisibilityForgeCertified:
		// eval+security gate required upfront
		if req.EvalRunID == "" || strings.ToLower(req.EvalOutcome) != "passed" {
			return nil, fmt.Errorf("%w: eval_pass_missing", ErrCertificationPrereq)
		}
		if req.SecurityRev == "" {
			return nil, fmt.Errorf("%w: security_review_missing", ErrCertificationPrereq)
		}
		state = ApprovalPending
	}
	now := s.now()
	listing := &Listing{
		ID:             uuid.NewString(),
		TenantID:       req.TenantID,
		WorkspaceID:    req.WorkspaceID,
		WorkflowID:     req.WorkflowID,
		Version:        req.Version,
		Name:           req.Name,
		Description:    req.Description,
		Tags:           append([]string(nil), req.Tags...),
		Criticality:    req.Criticality,
		Visibility:     req.Visibility,
		ApprovalState:  state,
		EvalRunID:      req.EvalRunID,
		EvalOutcome:    req.EvalOutcome,
		SecurityRev:    req.SecurityRev,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listings[listing.ID] = cloneListing(listing)
	_ = s.sink.Emit(newEvent(req.TenantID, req.WorkspaceID, "workflow.published.v1", listing.ID, map[string]any{
		"listing_id":    listing.ID,
		"workflow_id":   listing.WorkflowID,
		"version":       listing.Version,
		"visibility":    listing.Visibility,
		"approval_state": listing.ApprovalState,
	}))
	if listing.ApprovalState == ApprovalPending {
		_ = s.sink.Emit(newEvent(req.TenantID, req.WorkspaceID, "marketplace.approval.requested.v1", listing.ID, map[string]any{
			"listing_id":    listing.ID,
			"target_visibility": listing.Visibility,
			"approver_role": approverRoleFor(listing.Visibility),
		}))
	}
	return cloneListing(listing), nil
}

// Approve transitions a pending listing to approved/rejected.
func (s *Service) Approve(_ context.Context, req ApproveRequest) (*Listing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[req.ListingID]
	if !ok {
		return nil, ErrListingNotFound
	}
	if listing.ApprovalState != ApprovalPending {
		return nil, ErrApprovalNotPending
	}
	if req.Approve {
		listing.ApprovalState = ApprovalApproved
	} else {
		listing.ApprovalState = ApprovalRejected
	}
	listing.UpdatedAt = s.now()
	s.listings[listing.ID] = cloneListing(listing)
	_ = s.sink.Emit(newEvent(listing.TenantID, listing.WorkspaceID, "marketplace.approval.decided.v1", listing.ID, map[string]any{
		"listing_id": listing.ID,
		"approver":   req.Approver,
		"approved":   req.Approve,
		"reason":     req.Reason,
	}))
	return cloneListing(listing), nil
}

// Search returns listings matching the filters, scoped by Tenant.
func (s *Service) Search(_ context.Context, f SearchFilters) []*Listing {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*Listing{}
	for _, l := range s.listings {
		if f.TenantID != "" && l.TenantID != f.TenantID {
			continue
		}
		if !visibilityVisible(l, f) {
			continue
		}
		if f.Visibility != "" && l.Visibility != f.Visibility {
			continue
		}
		if f.Criticality != "" && l.Criticality != f.Criticality {
			continue
		}
		if !matchAllTags(l.Tags, f.Tags) {
			continue
		}
		if f.Text != "" && !textMatches(l, f.Text) {
			continue
		}
		out = append(out, cloneListing(l))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

// GetListing returns one listing scoped by tenant.
func (s *Service) GetListing(tenantID, listingID string) (*Listing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.listings[listingID]
	if !ok {
		return nil, ErrListingNotFound
	}
	if tenantID != "" && l.TenantID != tenantID {
		return nil, ErrCrossTenantInvisible
	}
	return cloneListing(l), nil
}

// Install creates an Install pinned to listing.WorkflowID@listing.Version.
// Visibility checks: workspace listings can be installed only into the same
// workspace; tenant/forge-certified must be approved.
func (s *Service) Install(_ context.Context, req InstallRequest) (*Install, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.listings[req.ListingID]
	if !ok {
		return nil, ErrListingNotFound
	}
	if req.TenantID != "" && l.TenantID != req.TenantID {
		return nil, ErrCrossTenantInvisible
	}
	switch l.Visibility {
	case ast.VisibilityPrivate:
		return nil, ErrInstallNotPermitted
	case ast.VisibilityWorkspace:
		if req.TargetWorkspaceID != l.WorkspaceID {
			return nil, ErrInstallNotPermitted
		}
	case ast.VisibilityTenant, ast.VisibilityForgeCertified:
		if l.ApprovalState != ApprovalApproved {
			return nil, ErrInstallNotPermitted
		}
	}
	install := &Install{
		ID:                uuid.NewString(),
		TenantID:          l.TenantID,
		TargetWorkspaceID: req.TargetWorkspaceID,
		SourceWorkspaceID: l.WorkspaceID,
		WorkflowID:        l.WorkflowID,
		Version:           l.Version,
		ListingID:         l.ID,
		Status:            "active",
		InstalledAt:       s.now(),
		InstalledBy:       req.Actor,
	}
	s.installs[install.ID] = cloneInstall(install)
	_ = s.sink.Emit(newEvent(l.TenantID, req.TargetWorkspaceID, "workflow.installed_to_workspace.v1", install.ID, map[string]any{
		"install_id":         install.ID,
		"target_workspace":   install.TargetWorkspaceID,
		"source_workspace":   install.SourceWorkspaceID,
		"workflow_id":        install.WorkflowID,
		"version":            install.Version,
		"listing_id":         install.ListingID,
	}))
	return cloneInstall(install), nil
}

// ListInstalls returns installs for a Workspace.
func (s *Service) ListInstalls(tenantID, workspaceID string) []*Install {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*Install{}
	for _, i := range s.installs {
		if tenantID != "" && i.TenantID != tenantID {
			continue
		}
		if workspaceID != "" && i.TargetWorkspaceID != workspaceID {
			continue
		}
		out = append(out, cloneInstall(i))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstalledAt.After(out[j].InstalledAt) })
	return out
}

func visibilityVisible(l *Listing, f SearchFilters) bool {
	switch l.Visibility {
	case ast.VisibilityPrivate:
		return f.WorkspaceID != "" && l.WorkspaceID == f.WorkspaceID
	case ast.VisibilityWorkspace:
		return f.WorkspaceID == "" || l.WorkspaceID == f.WorkspaceID
	case ast.VisibilityTenant, ast.VisibilityForgeCertified:
		if !f.IncludePending && l.ApprovalState != ApprovalApproved {
			return false
		}
		return true
	}
	return false
}

func matchAllTags(have, want []string) bool {
	if len(want) == 0 {
		return true
	}
	hs := map[string]struct{}{}
	for _, t := range have {
		hs[strings.ToLower(t)] = struct{}{}
	}
	for _, w := range want {
		if _, ok := hs[strings.ToLower(w)]; !ok {
			return false
		}
	}
	return true
}

func textMatches(l *Listing, text string) bool {
	t := strings.ToLower(text)
	hay := strings.ToLower(l.Name + " " + l.Description + " " + l.WorkflowID + " " + strings.Join(l.Tags, " "))
	return strings.Contains(hay, t)
}

func approverRoleFor(v ast.Visibility) string {
	switch v {
	case ast.VisibilityTenant:
		return "tenant-admin"
	case ast.VisibilityForgeCertified:
		return "forge-certifier"
	}
	return ""
}

func cloneListing(in *Listing) *Listing {
	if in == nil {
		return nil
	}
	out := *in
	out.Tags = append([]string(nil), in.Tags...)
	return &out
}

func cloneInstall(in *Install) *Install {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
