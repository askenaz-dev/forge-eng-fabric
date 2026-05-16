package application

import (
	"context"
	"errors"
	"testing"
)

func testSetup(t *testing.T) (*Service, *AllowAllAuthorizer, *MemorySink, *MemoryAuditSink) {
	t.Helper()
	authz := NewAllowAllAuthorizer()
	events := NewMemorySink()
	audit := NewMemoryAuditSink()
	dir := StaticWorkspaceLookup{"ws-1": "tenant-acme", "ws-2": "tenant-acme"}
	svc := NewService(NewStore(), authz, events, audit, NoLiveArtefacts{}, dir)
	return svc, authz, events, audit
}

func TestCreate_HappyPath(t *testing.T) {
	svc, authz, events, audit := testSetup(t)
	caller := Caller{Principal: "alice", CorrelationID: "corr-1"}
	app, err := svc.Create(context.Background(), caller, "ws-1", CreateInput{
		Slug:   "hr-portal",
		Name:   "HR Portal",
		Owners: []string{"alice"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if app.WorkspaceID != "ws-1" || app.TenantID != "tenant-acme" {
		t.Fatalf("wrong workspace/tenant on returned app: %+v", app)
	}
	if app.Lifecycle != LifecycleActive {
		t.Fatalf("expected lifecycle active, got %s", app.Lifecycle)
	}
	// alfred-design-system-picker (D3): when the caller omits
	// `design_system_chosen_explicitly`, the service emits the standard
	// `app.created.v1` AND the sibling `app.design_system.user_skipped.v1`
	// so observability can measure catalog discoverability.
	got := events.Events()
	if len(got) != 2 {
		t.Fatalf("expected app.created.v1 + app.design_system.user_skipped.v1, got %+v", got)
	}
	if got[0].Type != EventAppCreated || got[1].Type != EventDesignSystemUserSkipped {
		t.Fatalf("wrong event order/types: %+v", got)
	}
	if got[0].CorrelationID != got[1].CorrelationID {
		t.Fatalf("create and skip events MUST share correlation_id; got %s vs %s",
			got[0].CorrelationID, got[1].CorrelationID)
	}
	if got := audit.Records(); len(got) != 1 || got[0].Action != "created" {
		t.Fatalf("expected one created audit, got %+v", got)
	}
	// Tuples: one workspace -> app#parent + one owner tuple per owner.
	tuples := authz.Tuples()
	hasParent := false
	hasOwner := false
	for _, tuple := range tuples {
		if tuple.User == "workspace:ws-1" && tuple.Relation == "parent" {
			hasParent = true
		}
		if tuple.User == "user:alice" && tuple.Relation == string(PermOwner) {
			hasOwner = true
		}
	}
	if !hasParent || !hasOwner {
		t.Fatalf("missing tuples; got %+v", tuples)
	}
}

func TestCreate_SlugConflict(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	caller := Caller{Principal: "alice"}
	if _, err := svc.Create(context.Background(), caller, "ws-1", CreateInput{Slug: "hr-portal", Name: "HR", Owners: []string{"alice"}}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(context.Background(), caller, "ws-1", CreateInput{Slug: "hr-portal", Name: "HR Two", Owners: []string{"alice"}})
	if !errors.Is(err, ErrSlugConflict) {
		t.Fatalf("expected ErrSlugConflict, got %v", err)
	}
	// Same slug succeeds in a different workspace.
	if _, err := svc.Create(context.Background(), caller, "ws-2", CreateInput{Slug: "hr-portal", Name: "HR", Owners: []string{"alice"}}); err != nil {
		t.Fatalf("create in ws-2: %v", err)
	}
}

func TestCreate_ReservedSlugRejected(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	_, err := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug: "_unassigned", Name: "n", Owners: []string{"alice"},
	})
	if !errors.Is(err, ErrSlugReserved) {
		t.Fatalf("expected ErrSlugReserved, got %v", err)
	}
}

func TestCreate_InvalidSlugRejected(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	_, err := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug: "HR Portal!", Name: "n", Owners: []string{"alice"},
	})
	if !errors.Is(err, ErrSlugInvalid) {
		t.Fatalf("expected ErrSlugInvalid, got %v", err)
	}
}

func TestCreate_MissingOwners(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	_, err := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug: "hr-portal", Name: "n",
	})
	if !errors.Is(err, ErrMissingOwners) {
		t.Fatalf("expected ErrMissingOwners, got %v", err)
	}
}

func TestCreate_SystemPrincipalRefusedOnPublicCreate(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	_, err := svc.Create(context.Background(), Caller{Principal: SystemActor}, "ws-1", CreateInput{
		Slug: "x", Name: "n", Owners: []string{"alice"},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for system principal, got %v", err)
	}
}

func TestCreateUnassigned_OnlyForSystem(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	_, err := svc.CreateUnassigned(context.Background(), Caller{Principal: "alice"}, "ws-1", "tenant-acme")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	app, err := svc.CreateUnassigned(context.Background(), Caller{Principal: SystemActor}, "ws-1", "tenant-acme")
	if err != nil {
		t.Fatalf("system create: %v", err)
	}
	if !app.SystemManaged || app.Slug != UnassignedSlug {
		t.Fatalf("expected system-managed _unassigned, got %+v", app)
	}
}

func TestPatch_ReadOnlyOnUnassigned(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	app, _ := svc.CreateUnassigned(context.Background(), Caller{Principal: SystemActor}, "ws-1", "tenant-acme")
	newName := "renamed"
	_, err := svc.Patch(context.Background(), Caller{Principal: "alice"}, app.ID, PatchInput{Name: &newName})
	if !errors.Is(err, ErrSystemManaged) {
		t.Fatalf("expected ErrSystemManaged, got %v", err)
	}
}

func TestArchive_RefusedOnUnassigned(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	app, _ := svc.CreateUnassigned(context.Background(), Caller{Principal: SystemActor}, "ws-1", "tenant-acme")
	_, err := svc.Archive(context.Background(), Caller{Principal: "alice"}, app.ID)
	if !errors.Is(err, ErrSystemManaged) {
		t.Fatalf("expected ErrSystemManaged on archive of _unassigned, got %v", err)
	}
}

func TestDelete_RefusedWithLiveArtefacts(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	svc.LiveCheck = StaticLiveArtefacts{Report: LiveArtefactReport{Specs: []string{"spec-7"}}}
	caller := Caller{Principal: "alice"}
	app, _ := svc.Create(context.Background(), caller, "ws-1", CreateInput{Slug: "hr-portal", Name: "n", Owners: []string{"alice"}})
	result, err := svc.Delete(context.Background(), caller, app.ID)
	if !errors.Is(err, ErrAppHasLiveArtefacts) {
		t.Fatalf("expected ErrAppHasLiveArtefacts, got %v", err)
	}
	if result.Blocked == nil || len(result.Blocked.Specs) != 1 {
		t.Fatalf("expected blocked spec list, got %+v", result)
	}
}

func TestDelete_HappyPath(t *testing.T) {
	svc, _, events, _ := testSetup(t)
	caller := Caller{Principal: "alice"}
	app, _ := svc.Create(context.Background(), caller, "ws-1", CreateInput{Slug: "hr-portal", Name: "n", Owners: []string{"alice"}})
	if _, err := svc.Delete(context.Background(), caller, app.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(context.Background(), caller, app.ID); !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("expected ErrAppNotFound after delete, got %v", err)
	}
	saw := false
	for _, ev := range events.Events() {
		if ev.Type == EventAppDeleted {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("expected app.deleted.v1 event")
	}
}

func TestArchive_RestoreFlow(t *testing.T) {
	svc, _, events, _ := testSetup(t)
	caller := Caller{Principal: "alice"}
	app, _ := svc.Create(context.Background(), caller, "ws-1", CreateInput{Slug: "hr-portal", Name: "n", Owners: []string{"alice"}})
	if _, err := svc.Archive(context.Background(), caller, app.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	got, err := svc.Get(context.Background(), caller, app.ID)
	if err != nil {
		t.Fatalf("get archived: %v", err)
	}
	if got.Lifecycle != LifecycleArchived {
		t.Fatalf("expected archived, got %s", got.Lifecycle)
	}
	if _, err := svc.Restore(context.Background(), caller, app.ID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	got, _ = svc.Get(context.Background(), caller, app.ID)
	if got.Lifecycle != LifecycleActive {
		t.Fatalf("expected active after restore, got %s", got.Lifecycle)
	}
	var seen []EventType
	for _, ev := range events.Events() {
		seen = append(seen, ev.Type)
	}
	if !hasEvent(seen, EventAppArchived) || !hasEvent(seen, EventAppRestored) {
		t.Fatalf("expected archive+restore events, got %+v", seen)
	}
}

func TestPatch_EmitsDiff(t *testing.T) {
	svc, _, events, _ := testSetup(t)
	caller := Caller{Principal: "alice"}
	app, _ := svc.Create(context.Background(), caller, "ws-1", CreateInput{Slug: "hr-portal", Name: "n", Owners: []string{"alice"}})
	newName := "renamed"
	if _, err := svc.Patch(context.Background(), caller, app.ID, PatchInput{Name: &newName}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	var updateEvent *Event
	for i := range events.Events() {
		ev := events.Events()[i]
		if ev.Type == EventAppUpdated {
			updateEvent = &ev
		}
	}
	if updateEvent == nil {
		t.Fatalf("expected app.updated.v1 event")
	}
	if updateEvent.Before == nil || updateEvent.After == nil {
		t.Fatalf("expected before+after on update event")
	}
	if updateEvent.Before.Name != "n" || updateEvent.After.Name != "renamed" {
		t.Fatalf("expected diff names, got before=%q after=%q", updateEvent.Before.Name, updateEvent.After.Name)
	}
}

func TestValidateAppWorkspaceCoherence(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	app, _ := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{Slug: "hr-portal", Name: "n", Owners: []string{"alice"}})
	if err := svc.ValidateAppWorkspaceCoherence(context.Background(), app.ID, "ws-1"); err != nil {
		t.Fatalf("coherent: %v", err)
	}
	if err := svc.ValidateAppWorkspaceCoherence(context.Background(), app.ID, "ws-2"); !errors.Is(err, ErrAppWorkspaceMismatch) {
		t.Fatalf("expected mismatch, got %v", err)
	}
}

func hasEvent(events []EventType, want EventType) bool {
	for _, ev := range events {
		if ev == want {
			return true
		}
	}
	return false
}

// --- alfred-design-system-picker (D3, D4) atomic create tests ---

// approvedResolver returns a static StaticDesignSystemResolver pre-populated
// with `ds-forge-default`, `desing-system-1`, `desing-system-3` as approved
// tenant_global entries plus one tenant-private entry for tenant-other.
func approvedResolver() StaticDesignSystemResolver {
	return StaticDesignSystemResolver{
		DesignSystemDefaultRef: {AssetID: "desing-system-1", Version: "1.0.0", LifecycleState: "approved", Visibility: "tenant_global", BuiltInTemplate: true},
		"desing-system-1":      {AssetID: "desing-system-1", Version: "1.0.0", LifecycleState: "approved", Visibility: "tenant_global", BuiltInTemplate: true},
		"desing-system-3":      {AssetID: "desing-system-3", Version: "2.0.0", LifecycleState: "approved", Visibility: "tenant_global", BuiltInTemplate: true},
		"ds-proposed":          {AssetID: "ds-proposed", Version: "0.1.0", LifecycleState: "proposed", Visibility: "tenant_global"},
		"ds-other-tenant":      {AssetID: "ds-other-tenant", Version: "1.0.0", LifecycleState: "approved", Visibility: "tenant", TenantID: "tenant-other"},
	}
}

func TestCreate_AtomicWithExplicitDesignSystem(t *testing.T) {
	svc, _, events, audit := testSetup(t)
	svc.DesignSystem = approvedResolver()
	caller := Caller{Principal: "alice", CorrelationID: "corr-explicit"}
	app, err := svc.Create(context.Background(), caller, "ws-1", CreateInput{
		Slug:                         "hr-portal",
		Name:                         "HR Portal",
		Owners:                       []string{"alice"},
		DesignSystemRef:              "desing-system-3@2.0.0",
		DesignSystemChosenExplicitly: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if app.DesignSystemRef != "desing-system-3@2.0.0" {
		t.Fatalf("expected explicit ref persisted, got %s", app.DesignSystemRef)
	}
	got := events.Events()
	if len(got) != 1 {
		t.Fatalf("expected single app.created.v1 (no skip), got %d events: %+v", len(got), got)
	}
	if got[0].Type != EventAppCreated {
		t.Fatalf("expected app.created.v1, got %s", got[0].Type)
	}
	if got[0].Extra["design_system_chosen_explicitly"] != true {
		t.Fatalf("create event MUST carry chosen_explicitly=true, got %+v", got[0].Extra)
	}
	if got[0].Extra["design_system_ref"] != "desing-system-3@2.0.0" {
		t.Fatalf("create event MUST carry resolved ref, got %+v", got[0].Extra)
	}
	if rec := audit.Records(); len(rec) != 1 || rec[0].Evidence["design_system_chosen_explicitly"] != true {
		t.Fatalf("audit MUST carry chosen_explicitly=true evidence, got %+v", rec)
	}
}

func TestCreate_AtomicOmittingDesignSystemEmitsSkip(t *testing.T) {
	svc, _, events, _ := testSetup(t)
	svc.DesignSystem = approvedResolver()
	caller := Caller{Principal: "alice", CorrelationID: "corr-skip"}
	app, err := svc.Create(context.Background(), caller, "ws-1", CreateInput{
		Slug:   "hr-portal",
		Name:   "HR Portal",
		Owners: []string{"alice"},
		// no DesignSystemRef, no DesignSystemChosenExplicitly
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if app.DesignSystemRef != DesignSystemDefaultRef {
		t.Fatalf("expected alias-resolved default, got %s", app.DesignSystemRef)
	}
	got := events.Events()
	if len(got) != 2 {
		t.Fatalf("expected app.created.v1 + app.design_system.user_skipped.v1, got %d: %+v", len(got), got)
	}
	if got[1].Type != EventDesignSystemUserSkipped {
		t.Fatalf("expected second event to be user_skipped, got %s", got[1].Type)
	}
	if got[1].CorrelationID != "corr-skip" {
		t.Fatalf("skip event MUST share correlation_id, got %s", got[1].CorrelationID)
	}
	if got[1].Extra["resolved_ref"] != DesignSystemDefaultRef {
		t.Fatalf("skip event MUST carry resolved_ref, got %+v", got[1].Extra)
	}
}

func TestCreate_ChosenExplicitlyFalseWithExplicitRefStillEmitsSkip(t *testing.T) {
	// D3 edge case: explicit false wins. Ref is persisted, skip event fires.
	svc, _, events, _ := testSetup(t)
	svc.DesignSystem = approvedResolver()
	app, err := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug:                         "hr-portal",
		Name:                         "HR Portal",
		Owners:                       []string{"alice"},
		DesignSystemRef:              "desing-system-3@2.0.0",
		DesignSystemChosenExplicitly: false,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if app.DesignSystemRef != "desing-system-3@2.0.0" {
		t.Fatalf("explicit ref MUST persist even when chosen_explicitly=false, got %s", app.DesignSystemRef)
	}
	got := events.Events()
	if len(got) != 2 || got[1].Type != EventDesignSystemUserSkipped {
		t.Fatalf("expected skip event despite explicit ref, got %+v", got)
	}
	if got[1].Extra["resolved_ref"] != "desing-system-3@2.0.0" {
		t.Fatalf("skip event resolved_ref MUST be the explicit ref, got %+v", got[1].Extra)
	}
}

func TestCreate_RejectsProposedDesignSystem(t *testing.T) {
	svc, _, events, _ := testSetup(t)
	svc.DesignSystem = approvedResolver()
	_, err := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug: "hr-portal", Name: "n", Owners: []string{"alice"},
		DesignSystemRef:              "ds-proposed",
		DesignSystemChosenExplicitly: true,
	})
	if !errors.Is(err, ErrDesignSystemNotApproved) {
		t.Fatalf("expected ErrDesignSystemNotApproved, got %v", err)
	}
	if len(events.Events()) != 0 {
		t.Fatalf("expected no events on rejected create, got %+v", events.Events())
	}
}

func TestCreate_RejectsInvisibleDesignSystem(t *testing.T) {
	svc, _, events, _ := testSetup(t)
	svc.DesignSystem = approvedResolver()
	_, err := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug: "hr-portal", Name: "n", Owners: []string{"alice"},
		DesignSystemRef:              "ds-other-tenant",
		DesignSystemChosenExplicitly: true,
	})
	if !errors.Is(err, ErrDesignSystemNotVisible) {
		t.Fatalf("expected ErrDesignSystemNotVisible, got %v", err)
	}
	if len(events.Events()) != 0 {
		t.Fatalf("expected no events on rejected create, got %+v", events.Events())
	}
}

func TestCreate_AuditEvidenceMatchesEvent(t *testing.T) {
	svc, _, events, audit := testSetup(t)
	svc.DesignSystem = approvedResolver()
	caller := Caller{Principal: "alice", CorrelationID: "corr-match"}
	_, err := svc.Create(context.Background(), caller, "ws-1", CreateInput{
		Slug: "hr-portal", Name: "n", Owners: []string{"alice"},
		DesignSystemRef:              "desing-system-3@2.0.0",
		DesignSystemChosenExplicitly: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ev := events.Events()[0]
	rec := audit.Records()[0]
	if rec.Evidence["design_system_chosen_explicitly"] != ev.Extra["design_system_chosen_explicitly"] {
		t.Fatalf("audit evidence MUST match create event extra, audit=%v event=%v",
			rec.Evidence, ev.Extra)
	}
}
