// Package marketplace implements the tenant-scoped workflow marketplace:
// listings, search, install records and visibility approval flows.
package marketplace

import (
	"errors"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// ApprovalState mirrors the Approvals Inbox states for marketplace promotions.
type ApprovalState string

const (
	ApprovalNotRequired ApprovalState = "not_required"
	ApprovalPending     ApprovalState = "pending"
	ApprovalApproved    ApprovalState = "approved"
	ApprovalRejected    ApprovalState = "rejected"
)

// Listing is a marketplace catalog entry for a workflow version.
type Listing struct {
	ID            string         `json:"id"`
	TenantID      string         `json:"tenant_id"`
	WorkspaceID   string         `json:"workspace_id"`
	WorkflowID    string         `json:"workflow_id"`
	Version       string         `json:"version"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Tags          []string       `json:"tags,omitempty"`
	Criticality   ast.Criticality `json:"criticality,omitempty"`
	Visibility    ast.Visibility `json:"visibility"`
	ApprovalState ApprovalState  `json:"approval_state"`
	EvalRunID     string         `json:"eval_run_id,omitempty"`
	EvalOutcome   string         `json:"eval_outcome,omitempty"`
	SecurityRev   string         `json:"security_review_id,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// Install is a record of a workflow installed into a Workspace.
type Install struct {
	ID                 string    `json:"id"`
	TenantID           string    `json:"tenant_id"`
	TargetWorkspaceID  string    `json:"target_workspace_id"`
	SourceWorkspaceID  string    `json:"source_workspace_id"`
	WorkflowID         string    `json:"workflow_id"`
	Version            string    `json:"version"`
	ListingID          string    `json:"listing_id"`
	Status             string    `json:"status"`
	InstalledAt        time.Time `json:"installed_at"`
	InstalledBy        string    `json:"installed_by,omitempty"`
}

// PublishRequest publishes (or promotes) a workflow version into the catalog.
type PublishRequest struct {
	WorkflowID    string         `json:"workflow_id"`
	Version       string         `json:"version"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Tags          []string       `json:"tags,omitempty"`
	Criticality   ast.Criticality `json:"criticality,omitempty"`
	TenantID      string         `json:"tenant_id"`
	WorkspaceID   string         `json:"workspace_id"`
	Visibility    ast.Visibility `json:"visibility"`
	EvalRunID     string         `json:"eval_run_id,omitempty"`
	EvalOutcome   string         `json:"eval_outcome,omitempty"`
	SecurityRev   string         `json:"security_review_id,omitempty"`
	Actor         string         `json:"actor,omitempty"`
}

// InstallRequest installs a listing into a target Workspace.
type InstallRequest struct {
	TenantID          string `json:"tenant_id"`
	ListingID         string `json:"listing_id"`
	TargetWorkspaceID string `json:"target_workspace_id"`
	Actor             string `json:"actor,omitempty"`
}

// ApproveRequest approves or rejects a pending listing.
type ApproveRequest struct {
	ListingID string `json:"listing_id"`
	Approver  string `json:"approver"`
	Approve   bool   `json:"approve"`
	Reason    string `json:"reason,omitempty"`
}

// SearchFilters narrows the catalog query.
type SearchFilters struct {
	TenantID      string
	WorkspaceID   string
	Visibility    ast.Visibility
	Tags          []string
	Criticality   ast.Criticality
	Text          string
	IncludePending bool
}

// Errors.
var (
	ErrListingNotFound          = errors.New("listing_not_found")
	ErrInstallNotPermitted      = errors.New("install_not_permitted")
	ErrCertificationPrereq      = errors.New("certification_prerequisites_missing")
	ErrApprovalNotPending       = errors.New("approval_not_pending")
	ErrCrossTenantInvisible     = errors.New("cross_tenant_listing_invisible")
)
