package policy

import "testing"

func TestRequireEvalPassDeniesMissingEval(t *testing.T) {
	engine, err := LoadWorkflowTemplates()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	resp, err := engine.Evaluate(EvaluateRequest{
		Action: "workflow:publish",
		Target: map[string]any{
			"eval_outcome": "missing",
		},
	})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if resp.Decision != Deny || resp.PolicyID != "require-eval-pass" {
		t.Fatalf("expected deny by require-eval-pass, got %+v", resp)
	}
}

func TestTenantShareRequiresApproval(t *testing.T) {
	engine, err := LoadWorkflowTemplates()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	resp, err := engine.Evaluate(EvaluateRequest{
		Action: "workflow:promote",
		Target: map[string]any{
			"visibility":   "tenant",
			"eval_outcome": "passed",
		},
	})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if resp.Decision != RequiresApproval {
		t.Fatalf("expected requires_approval, got %+v", resp)
	}
	hasTenantAdmin := false
	for _, a := range resp.RequiredApprovers {
		if a == "tenant-admin" {
			hasTenantAdmin = true
		}
	}
	if !hasTenantAdmin {
		t.Fatalf("expected tenant-admin in required approvers, got %+v", resp.RequiredApprovers)
	}
}

func TestForgeCertificationDeniedWithoutSecurityReview(t *testing.T) {
	engine, err := LoadWorkflowTemplates()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	resp, err := engine.Evaluate(EvaluateRequest{
		Action: "marketplace:certify",
		Target: map[string]any{
			"visibility":         "forge-certified",
			"eval_outcome":       "passed",
			"security_review_id": "",
		},
	})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if resp.Decision != Deny {
		t.Fatalf("expected deny, got %+v", resp)
	}
}
