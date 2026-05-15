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
	if got := events.Events(); len(got) != 1 || got[0].Type != EventAppCreated {
		t.Fatalf("expected one app.created.v1, got %+v", got)
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
