package main

import (
	"testing"

	"github.com/google/uuid"
)

func TestSemVerPattern(t *testing.T) {
	tests := map[string]bool{
		"0.1.0":    true,
		"1.0.0":    true,
		"10.20.30": true,
		"1.0":      false,
		"1.0.0-rc": false,
		"01.0.0":   false,
		"v1.0.0":   false,
	}
	for version, want := range tests {
		if got := semverPattern.MatchString(version); got != want {
			t.Fatalf("semverPattern.MatchString(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestPhase2AssetTypes(t *testing.T) {
	for _, assetType := range []string{"mcp", "skill", "agent", "workflow", "prompt_template", "application", "repo_template"} {
		if _, ok := validTypes[assetType]; !ok {
			t.Fatalf("expected %q to be a valid asset type", assetType)
		}
	}
	for _, assetType := range []string{"prompt", "eval_dataset", "healing_action"} {
		if _, ok := validTypes[assetType]; ok {
			t.Fatalf("did not expect legacy asset type %q to be valid", assetType)
		}
	}
}

func TestPipelineHookReadyRequiresGreenGatesAndSignedImage(t *testing.T) {
	ready := pipelineGreenHookRequest{
		ImageSigned:         true,
		SignatureVerified:   true,
		AttestationVerified: true,
		SBOMPublished:       true,
		GateResults:         []hookGate{{Stage: "lint", Outcome: "pass"}, {Stage: "sast", Outcome: "warn"}},
	}
	if ok, reason := pipelineHookReady(ready); !ok {
		t.Fatalf("expected ready hook, got %s", reason)
	}
	ready.GateResults = append(ready.GateResults, hookGate{Stage: "sca", Outcome: "fail"})
	if ok, _ := pipelineHookReady(ready); ok {
		t.Fatal("failing gate should block proposed to in_review hook")
	}
}

func TestLifecycleTransitionsAndEvalThresholds(t *testing.T) {
	if !canTransition("proposed", "in_review") {
		t.Fatal("proposed should transition to in_review")
	}
	if canTransition("proposed", "approved") {
		t.Fatal("proposed should not transition directly to approved")
	}
	failing := failingEvalScores("T1", map[string]any{"quality": 0.9, "safety": 0.9, "cost": 0.9, "latency": 0.7})
	if _, ok := failing["latency"]; !ok || len(failing) != 1 {
		t.Fatalf("expected only latency to fail, got %#v", failing)
	}
}

func TestInvocationAllowedRequiresApprovedForProd(t *testing.T) {
	if ok, _ := invocationAllowed("in_review", "prod"); ok {
		t.Fatal("in_review asset should not be invocable in prod")
	}
	if ok, _ := invocationAllowed("approved", "prod"); !ok {
		t.Fatal("approved asset should be invocable in prod")
	}
	if ok, _ := invocationAllowed("in_review", "dev"); !ok {
		t.Fatal("dev flow may invoke non-approved assets")
	}
}

func TestAssetInvocationCheckedEventIncludesAuditContext(t *testing.T) {
	workspaceID := uuid.New()
	tenantID := uuid.New()
	event := buildAssetInvocationCheckedEvent(
		"asset-1",
		"0.1.0",
		workspaceID,
		tenantID,
		"in_review",
		"T1",
		"prod",
		false,
		"production-relevant flows require approved assets",
		"com.forge.asset.invocation.checked.v1",
		"corr-registry",
		"user-1",
	)

	if event["type"] != "com.forge.asset.invocation.checked.v1" {
		t.Fatalf("unexpected event type: %#v", event["type"])
	}
	if event["forgecorrelationid"] != "corr-registry" {
		t.Fatalf("missing correlation id: %#v", event["forgecorrelationid"])
	}
	data := event["data"].(map[string]any)
	if data["allowed"] != false || data["environment"] != "prod" {
		t.Fatalf("unexpected invocation audit data: %#v", data)
	}
}
