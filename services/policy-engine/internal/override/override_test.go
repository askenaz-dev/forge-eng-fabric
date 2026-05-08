package override

import (
	"testing"
	"time"
)

const sampleYAML = `
templates:
  - id: bypass-gate
    action: pipeline.gate.bypass
    target: pull_request
    required_role: security-approver
    max_ttl_seconds: 3600
    requires_reason: true
    events_on_grant: [policy.override.granted.v1]
    events_on_consume: [policy.override.consumed.v1]
    events_on_expire: [policy.override.expired.v1]
  - id: relax-branch-protection
    action: github.branch_protection.relax
    target: branch
    required_role: security-approver
    max_ttl_seconds: 86400
    requires_reason: true
    events_on_grant: [policy.override.granted.v1]
    events_on_expire: [policy.override.expired.v1]
`

func newManager(t *testing.T) (*Manager, *MemorySink) {
	t.Helper()
	tpls, err := LoadTemplates([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}
	sink := &MemorySink{}
	return NewManager(tpls, sink), sink
}

func TestGrantHappyPath(t *testing.T) {
	m, sink := newManager(t)
	ov, err := m.Grant(GrantInput{
		TemplateID:   "bypass-gate",
		WorkspaceID:  "ws-1",
		Subject:      "pr/org/repo#42",
		RequestedBy:  "alice",
		ApprovedBy:   "bob",
		ApproverRole: "security-approver",
		Reason:       "production incident remediation",
		TTLSeconds:   1800,
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	if ov.State != StateActive {
		t.Fatalf("expected active, got %s", ov.State)
	}
	if !m.IsActive("bypass-gate", "pr/org/repo#42") {
		t.Fatal("expected IsActive=true")
	}
	if len(sink.Events) != 1 || sink.Events[0].Type != "policy.override.granted.v1" {
		t.Fatalf("missing granted event: %+v", sink.Events)
	}
}

func TestGrantRejectsInsufficientRole(t *testing.T) {
	m, _ := newManager(t)
	_, err := m.Grant(GrantInput{
		TemplateID:   "bypass-gate",
		WorkspaceID:  "ws-1",
		Subject:      "pr/x#1",
		ApprovedBy:   "carol",
		ApproverRole: "developer",
		Reason:       "x",
		TTLSeconds:   60,
	})
	if err != ErrInsufficientRole {
		t.Fatalf("expected ErrInsufficientRole, got %v", err)
	}
}

func TestGrantRejectsTTLExceedingMax(t *testing.T) {
	m, _ := newManager(t)
	_, err := m.Grant(GrantInput{
		TemplateID:   "bypass-gate",
		WorkspaceID:  "ws-1",
		Subject:      "pr/x#1",
		ApprovedBy:   "bob",
		ApproverRole: "security-approver",
		Reason:       "x",
		TTLSeconds:   90000,
	})
	if err != ErrTTLExceedsMax {
		t.Fatalf("expected ErrTTLExceedsMax, got %v", err)
	}
}

func TestConsumeAndIsActive(t *testing.T) {
	m, sink := newManager(t)
	ov, _ := m.Grant(GrantInput{
		TemplateID: "bypass-gate", WorkspaceID: "ws-1", Subject: "pr/y#1",
		ApprovedBy: "bob", ApproverRole: "security-approver", Reason: "x", TTLSeconds: 600,
	})
	if err := m.Consume(ov.ID, "alice"); err != nil {
		t.Fatalf("consume: %v", err)
	}
	if m.IsActive("bypass-gate", "pr/y#1") {
		t.Fatal("expected IsActive=false after consume")
	}
	consumed := false
	for _, e := range sink.Events {
		if e.Type == "policy.override.consumed.v1" {
			consumed = true
		}
	}
	if !consumed {
		t.Fatal("missing consumed event")
	}
	if err := m.Consume(ov.ID, "alice"); err != ErrAlreadyTerminated {
		t.Fatalf("expected ErrAlreadyTerminated, got %v", err)
	}
}

func TestReconcileExpiredEmitsEventAndRevertsActive(t *testing.T) {
	m, sink := newManager(t)
	clock := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	m.SetClock(func() time.Time { return clock })

	ov, _ := m.Grant(GrantInput{
		TemplateID: "relax-branch-protection", WorkspaceID: "ws-1",
		Subject: "branch/org/repo/main", ApprovedBy: "bob",
		ApproverRole: "security-approver", Reason: "remediation", TTLSeconds: 3600,
	})

	// Advance past TTL
	clock = clock.Add(2 * time.Hour)
	ids := m.ReconcileExpired()
	if len(ids) != 1 || ids[0] != ov.ID {
		t.Fatalf("expected 1 expiration of %s, got %v", ov.ID, ids)
	}

	expired := false
	for _, e := range sink.Events {
		if e.Type == "policy.override.expired.v1" {
			expired = true
		}
	}
	if !expired {
		t.Fatal("missing expired event")
	}
	if m.IsActive("relax-branch-protection", "branch/org/repo/main") {
		t.Fatal("override should not be active after expiration")
	}
}
