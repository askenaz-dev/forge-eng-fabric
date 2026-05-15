package application

import (
	"context"
	"errors"
	"fmt"
)

// BootstrapHook is the seam called by the workspace-bootstrap pipeline when a
// new Workspace is provisioned. It atomically creates the `_unassigned` App
// before the Workspace becomes available to its members (see
// workspace-management spec scenario "New workspace ships with unassigned App").
//
// The hook is idempotent: calling it twice for the same workspace returns the
// existing `_unassigned` App rather than failing on slug conflict, so partial
// retries from the bootstrap pipeline do not leave a workspace in a bad state.
type BootstrapHook struct {
	Service *Service
}

func NewBootstrapHook(svc *Service) *BootstrapHook { return &BootstrapHook{Service: svc} }

// OnWorkspaceCreated is the entry point invoked by the workspace-bootstrap
// pipeline. `tenantID` is the parent tenant; the caller must already have
// authenticated as the platform service principal.
func (h *BootstrapHook) OnWorkspaceCreated(ctx context.Context, workspaceID, tenantID string) (*App, error) {
	if workspaceID == "" || tenantID == "" {
		return nil, fmt.Errorf("workspace_id and tenant_id are required")
	}
	caller := Caller{Principal: SystemActor, CorrelationID: "workspace-bootstrap:" + workspaceID}
	existing, err := h.Service.Store.GetBySlug(workspaceID, UnassignedSlug)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrAppNotFound) {
		return nil, err
	}
	return h.Service.CreateUnassigned(ctx, caller, workspaceID, tenantID)
}
