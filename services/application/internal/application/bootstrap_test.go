package application

import (
	"context"
	"errors"
	"testing"
)

func TestBootstrap_IdempotentOnReentry(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	hook := NewBootstrapHook(svc)
	first, err := hook.OnWorkspaceCreated(context.Background(), "ws-1", "tenant-acme")
	if err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	second, err := hook.OnWorkspaceCreated(context.Background(), "ws-1", "tenant-acme")
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected idempotent bootstrap, got two different App ids: %s vs %s", first.ID, second.ID)
	}
	if !first.SystemManaged || first.Slug != UnassignedSlug {
		t.Fatalf("expected system-managed _unassigned App, got %+v", first)
	}
}

func TestBootstrap_RejectsEmpty(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	hook := NewBootstrapHook(svc)
	if _, err := hook.OnWorkspaceCreated(context.Background(), "", "tenant-acme"); err == nil {
		t.Fatalf("expected error for empty workspace")
	}
	if _, err := hook.OnWorkspaceCreated(context.Background(), "ws-1", ""); err == nil {
		t.Fatalf("expected error for empty tenant")
	}
}

func TestUnassignedApp_PublicCreateRefused(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	_, err := svc.Create(context.Background(), Caller{Principal: "alice"}, "ws-1", CreateInput{
		Slug: UnassignedSlug, Name: "Unassigned", Owners: []string{"alice"},
	})
	if !errors.Is(err, ErrSlugReserved) {
		t.Fatalf("expected ErrSlugReserved on _unassigned via public create, got %v", err)
	}
}

func TestUnassignedApp_DeleteRefused(t *testing.T) {
	svc, _, _, _ := testSetup(t)
	hook := NewBootstrapHook(svc)
	app, _ := hook.OnWorkspaceCreated(context.Background(), "ws-1", "tenant-acme")
	_, err := svc.Delete(context.Background(), Caller{Principal: "alice"}, app.ID)
	if !errors.Is(err, ErrSystemManaged) {
		t.Fatalf("expected ErrSystemManaged on delete _unassigned, got %v", err)
	}
}
