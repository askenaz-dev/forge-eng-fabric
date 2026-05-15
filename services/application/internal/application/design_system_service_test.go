package application

import (
	"context"
	"strings"
	"testing"
)

func newDesignSystemTestFixture(t *testing.T) (*DesignSystemService, *App, *MockPROpener, *MemoryDesignSystemPRStore, *MemorySink) {
	t.Helper()
	store := NewStore()
	authz := NewAllowAllAuthorizer()
	events := NewMemorySink()
	audit := NewMemoryAuditSink()
	resolver := StaticDesignSystemResolver{
		"ds-forge-default": {AssetID: "design_system:platform:desing-system-1", Version: "1.0.0", LifecycleState: "approved", Visibility: "tenant_global", BuiltInTemplate: true},
		"design_system:platform:desing-system-1": {AssetID: "design_system:platform:desing-system-1", Version: "1.0.0", LifecycleState: "approved", Visibility: "tenant_global", BuiltInTemplate: true},
		"design_system:platform:desing-system-3": {AssetID: "design_system:platform:desing-system-3", Version: "2.0.0", LifecycleState: "approved", Visibility: "tenant_global", BuiltInTemplate: true},
		"design_system:platform:desing-system-proposed": {AssetID: "design_system:platform:desing-system-proposed", Version: "0.1.0", LifecycleState: "proposed", Visibility: "tenant"},
	}
	svc := NewService(store, authz, events, audit, NoLiveArtefacts{}, StaticWorkspaceLookup{"ws-1": "tenant-1"})
	svc.DesignSystem = resolver
	app, err := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug: "demo-app", Name: "Demo App", Owners: []string{"alice"},
		DesignSystemRef: "ds-forge-default",
		RepoLinks:       []string{"https://github.example/forge/demo-app-portal-bundle"},
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	pr := &MockPROpener{}
	prStore := NewMemoryDesignSystemPRStore()
	ds := NewDesignSystemService(DesignSystemServiceDeps{Service: svc, Resolver: resolver, PR: pr, PRStore: prStore})
	return ds, app, pr, prStore, events
}

func TestApp_Create_DefaultsToForgeDefault(t *testing.T) {
	store := NewStore()
	authz := NewAllowAllAuthorizer()
	events := NewMemorySink()
	resolver := StaticDesignSystemResolver{
		"ds-forge-default": {AssetID: "design_system:platform:desing-system-1", Version: "1.0.0", LifecycleState: "approved"},
	}
	svc := NewService(store, authz, events, NewMemoryAuditSink(), NoLiveArtefacts{}, StaticWorkspaceLookup{"ws-1": "tenant-1"})
	svc.DesignSystem = resolver
	app, err := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug: "no-ds", Name: "No DS", Owners: []string{"alice"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if app.DesignSystemRef != DesignSystemDefaultRef {
		t.Fatalf("expected default ref %s, got %q", DesignSystemDefaultRef, app.DesignSystemRef)
	}
}

func TestApp_Create_RejectsNonApprovedRef(t *testing.T) {
	store := NewStore()
	resolver := StaticDesignSystemResolver{
		"ds-forge-default":                              {AssetID: "x", LifecycleState: "approved"},
		"design_system:platform:desing-system-proposed": {AssetID: "y", LifecycleState: "proposed"},
	}
	svc := NewService(store, NewAllowAllAuthorizer(), NewMemorySink(), NewMemoryAuditSink(), NoLiveArtefacts{}, StaticWorkspaceLookup{"ws-1": "tenant-1"})
	svc.DesignSystem = resolver
	_, err := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug: "bad-ds", Name: "Bad DS", Owners: []string{"alice"},
		DesignSystemRef: "design_system:platform:desing-system-proposed",
	})
	if err != ErrDesignSystemNotApproved {
		t.Fatalf("expected ErrDesignSystemNotApproved, got %v", err)
	}
}

func TestDesignSystemSwap_OpensPRAndEmitsEvent(t *testing.T) {
	ds, app, pr, prStore, events := newDesignSystemTestFixture(t)
	_, swapPR, err := ds.Swap(context.Background(), Caller{Principal: "alice"}, app.ID, SwapInput{
		TargetRef: "design_system:platform:desing-system-3",
		Reason:    "Corporate refresh",
	})
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	if len(pr.Opens) != 1 {
		t.Fatalf("expected 1 PR opened, got %d", len(pr.Opens))
	}
	if pr.Opens[0].TargetRef != "design_system:platform:desing-system-3" {
		t.Fatalf("wrong target_ref on PR: %+v", pr.Opens[0])
	}
	if swapPR.Status != "open" || swapPR.PRURL == "" {
		t.Fatalf("expected open swap PR with URL, got %+v", swapPR)
	}
	open, _ := prStore.ListOpen(context.Background(), app.ID)
	if len(open) != 1 {
		t.Fatalf("expected 1 open PR row, got %d", len(open))
	}
	found := false
	for _, ev := range events.Events() {
		if ev.Type == EventDesignSystemSwapRequested {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected app.design_system.swap_requested.v1 emitted, got %+v", events.Events())
	}
}

func TestDesignSystemSwap_RejectsNonApprovedTarget(t *testing.T) {
	ds, app, _, _, _ := newDesignSystemTestFixture(t)
	_, _, err := ds.Swap(context.Background(), Caller{Principal: "alice"}, app.ID, SwapInput{
		TargetRef: "design_system:platform:desing-system-proposed",
		Reason:    "bad",
	})
	if err == nil || !strings.Contains(err.Error(), "design_system_not_approved") {
		t.Fatalf("expected design_system_not_approved, got %v", err)
	}
}

func TestDesignSystemSwap_AutoClosesPriorOpenPR(t *testing.T) {
	ds, app, pr, prStore, _ := newDesignSystemTestFixture(t)
	_, _, _ = ds.Swap(context.Background(), Caller{Principal: "alice"}, app.ID, SwapInput{TargetRef: "design_system:platform:desing-system-3"})
	_, _, _ = ds.Swap(context.Background(), Caller{Principal: "alice"}, app.ID, SwapInput{TargetRef: "design_system:platform:desing-system-1"})
	if len(pr.Opens) != 2 {
		t.Fatalf("expected 2 PR opens, got %d", len(pr.Opens))
	}
	if len(pr.Closes) != 1 {
		t.Fatalf("expected 1 PR close (supersede), got %d (%+v)", len(pr.Closes), pr.Closes)
	}
	if !strings.Contains(pr.Closes[0].Reason, "Superseded") {
		t.Fatalf("expected supersede reason, got %+v", pr.Closes[0])
	}
	open, _ := prStore.ListOpen(context.Background(), app.ID)
	if len(open) != 1 {
		t.Fatalf("expected exactly 1 open PR after supersede, got %d", len(open))
	}
}

func TestDesignSystemSwap_RequiresOwner(t *testing.T) {
	store := NewStore()
	resolver := StaticDesignSystemResolver{
		"ds-forge-default":                              {AssetID: "x", LifecycleState: "approved"},
		"design_system:platform:desing-system-1":        {AssetID: "design_system:platform:desing-system-1", LifecycleState: "approved"},
	}
	// A denying authorizer simulates a caller without app#owner.
	denyAuthz := denyAuthorizer{}
	svc := NewService(store, NewAllowAllAuthorizer(), NewMemorySink(), NewMemoryAuditSink(), NoLiveArtefacts{}, StaticWorkspaceLookup{"ws-1": "tenant-1"})
	svc.DesignSystem = resolver
	app, _ := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug: "demo", Name: "demo", Owners: []string{"alice"},
		RepoLinks: []string{"https://example/repo"},
	})
	// Now flip to a denying authorizer for the swap call.
	svc.Authz = denyAuthz
	ds := NewDesignSystemService(DesignSystemServiceDeps{Service: svc, Resolver: resolver})
	_, _, err := ds.Swap(context.Background(), Caller{Principal: "bob"}, app.ID, SwapInput{TargetRef: "design_system:platform:desing-system-1"})
	if err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestDesignSystemSwap_MergedWebhookFlipsRef(t *testing.T) {
	ds, app, pr, _, events := newDesignSystemTestFixture(t)
	_, _, err := ds.Swap(context.Background(), Caller{Principal: "alice"}, app.ID, SwapInput{TargetRef: "design_system:platform:desing-system-3"})
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	prURL := pr.Opens[0]
	_ = prURL
	// The MockPROpener returned a URL of the form https://github.example/<slug>/pull/<n>.
	openPRs, _ := ds.PRStore.ListOpen(context.Background(), app.ID)
	if len(openPRs) != 1 {
		t.Fatalf("expected 1 open PR, got %d", len(openPRs))
	}
	merged, err := ds.HandleSwapPRMerged(context.Background(), Caller{Principal: "system:portal-bundle-webhook"}, openPRs[0].PRURL)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if merged.DesignSystemRef != "design_system:platform:desing-system-3" {
		t.Fatalf("expected ref flipped to desing-system-3, got %q", merged.DesignSystemRef)
	}
	found := false
	for _, ev := range events.Events() {
		if ev.Type == EventDesignSystemChanged {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected app.design_system.changed.v1 emitted")
	}
}

func TestDesignSystemOverrides_RejectsUnknownComponent(t *testing.T) {
	ds, app, _, _, _ := newDesignSystemTestFixture(t)
	_, err := ds.PatchOverrides(context.Background(), Caller{Principal: "alice"}, app.ID, OverridesInput{
		Overrides: map[string]string{"weird-thing": "design_system:platform:desing-system-3"},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown_component") {
		t.Fatalf("expected unknown_component, got %v", err)
	}
}

func TestDesignSystemOverrides_RejectsNonApprovedTarget(t *testing.T) {
	ds, app, _, _, _ := newDesignSystemTestFixture(t)
	_, err := ds.PatchOverrides(context.Background(), Caller{Principal: "alice"}, app.ID, OverridesInput{
		Overrides: map[string]string{"card": "design_system:platform:desing-system-proposed"},
	})
	if err == nil || !strings.Contains(err.Error(), "design_system_not_approved") {
		t.Fatalf("expected design_system_not_approved, got %v", err)
	}
}

func TestDesignSystemOverrides_PersistsAndEmits(t *testing.T) {
	ds, app, _, _, events := newDesignSystemTestFixture(t)
	updated, err := ds.PatchOverrides(context.Background(), Caller{Principal: "alice"}, app.ID, OverridesInput{
		Overrides: map[string]string{"card": "design_system:platform:desing-system-3"},
	})
	if err != nil {
		t.Fatalf("patch overrides: %v", err)
	}
	if updated.DesignSystemOverrides["card"] != "design_system:platform:desing-system-3" {
		t.Fatalf("override not persisted: %+v", updated.DesignSystemOverrides)
	}
	found := false
	for _, ev := range events.Events() {
		if ev.Type == EventDesignSystemOverrideChanged {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected app.design_system.override_changed.v1 emitted")
	}
}

// denyAuthorizer rejects every Check call. Used in owner-required tests.
type denyAuthorizer struct{}

func (denyAuthorizer) Check(_ context.Context, _, _, _ string) (bool, error)        { return false, nil }
func (denyAuthorizer) WriteTuple(_ context.Context, _, _, _ string) error           { return nil }
func (denyAuthorizer) DeleteTuple(_ context.Context, _, _, _ string) error          { return nil }
