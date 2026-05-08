package marketplace

import (
	"context"
	"errors"
	"testing"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

func TestPublishWorkspaceNoApproval(t *testing.T) {
	svc := NewService(&MemorySink{})
	listing, err := svc.Publish(context.Background(), PublishRequest{
		WorkflowID: "wf-1", Version: "1.0.0", TenantID: "t1", WorkspaceID: "w1",
		Visibility: ast.VisibilityWorkspace, Name: "Refine PR",
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if listing.ApprovalState != ApprovalNotRequired {
		t.Fatalf("approval state: %s", listing.ApprovalState)
	}
}

func TestPublishTenantRequiresApproval(t *testing.T) {
	sink := &MemorySink{}
	svc := NewService(sink)
	listing, err := svc.Publish(context.Background(), PublishRequest{
		WorkflowID: "wf-1", Version: "1.0.0", TenantID: "t1", WorkspaceID: "w1",
		Visibility: ast.VisibilityTenant, Name: "Refine PR",
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if listing.ApprovalState != ApprovalPending {
		t.Fatalf("expected pending, got %s", listing.ApprovalState)
	}
	if len(sink.ByType("marketplace.approval.requested.v1")) != 1 {
		t.Fatalf("expected approval requested event")
	}
}

func TestForgeCertifiedRequiresEvalAndSecurity(t *testing.T) {
	svc := NewService(&MemorySink{})
	_, err := svc.Publish(context.Background(), PublishRequest{
		WorkflowID: "wf-1", Version: "1.0.0", TenantID: "t1", WorkspaceID: "w1",
		Visibility: ast.VisibilityForgeCertified, Name: "x",
	})
	if !errors.Is(err, ErrCertificationPrereq) {
		t.Fatalf("expected prereq error, got %v", err)
	}
	_, err = svc.Publish(context.Background(), PublishRequest{
		WorkflowID: "wf-1", Version: "1.0.0", TenantID: "t1", WorkspaceID: "w1",
		Visibility: ast.VisibilityForgeCertified, Name: "x",
		EvalRunID: "eval-1", EvalOutcome: "passed", SecurityRev: "sec-1",
	})
	if err != nil {
		t.Fatalf("publish ok: %v", err)
	}
}

func TestInstallPinsExactVersion(t *testing.T) {
	sink := &MemorySink{}
	svc := NewService(sink)
	listing, _ := svc.Publish(context.Background(), PublishRequest{
		WorkflowID: "wf-1", Version: "1.2.0", TenantID: "t1", WorkspaceID: "ws-source",
		Visibility: ast.VisibilityTenant,
	})
	if _, err := svc.Approve(context.Background(), ApproveRequest{ListingID: listing.ID, Approver: "tenant-admin", Approve: true}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	install, err := svc.Install(context.Background(), InstallRequest{
		TenantID: "t1", ListingID: listing.ID, TargetWorkspaceID: "ws-target", Actor: "alice",
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if install.Version != "1.2.0" {
		t.Fatalf("expected 1.2.0, got %s", install.Version)
	}
	if len(sink.ByType("workflow.installed_to_workspace.v1")) == 0 {
		t.Fatalf("expected installed_to_workspace event")
	}
}

func TestSearchRespectsTenantBoundary(t *testing.T) {
	svc := NewService(&MemorySink{})
	_, _ = svc.Publish(context.Background(), PublishRequest{
		WorkflowID: "wf-A", Version: "1.0.0", TenantID: "tA", WorkspaceID: "ws", Visibility: ast.VisibilityWorkspace,
	})
	_, _ = svc.Publish(context.Background(), PublishRequest{
		WorkflowID: "wf-B", Version: "1.0.0", TenantID: "tB", WorkspaceID: "ws", Visibility: ast.VisibilityWorkspace,
	})
	resA := svc.Search(context.Background(), SearchFilters{TenantID: "tA"})
	if len(resA) != 1 {
		t.Fatalf("expected 1 result for tenant A, got %d", len(resA))
	}
	if resA[0].WorkflowID != "wf-A" {
		t.Fatalf("expected wf-A, got %s", resA[0].WorkflowID)
	}
}
