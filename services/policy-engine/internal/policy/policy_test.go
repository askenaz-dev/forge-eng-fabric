package policy

import (
	"strings"
	"testing"
)

const goldenPolicies = `
policies:
  - id: deny-restricted-data-prod
    priority: 100
    decision: deny
    condition: 'env == "prod" && data_classification == "restricted"'
    rationale: restricted data cannot be used in prod automation
  - id: prod-requires-approval
    priority: 90
    decision: requires_approval
    condition: 'action == "deploy:prod" || env == "prod"'
    rationale: prod changes need release-manager approval
    required_approvers: [release-manager]
  - id: workspace-default-allow
    priority: 1
    decision: allow
    condition: 'workspace_id != ""'
    rationale: workspace default autonomous
`

func TestEvaluateGoldenCases(t *testing.T) {
	engine, err := LoadYAML(strings.NewReader(goldenPolicies))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		req      EvaluateRequest
		decision Decision
		policyID string
	}{
		{
			name:     "allow dev action",
			req:      EvaluateRequest{Action: "openspec:write", WorkspaceID: "ws-1", Env: "dev"},
			decision: Allow,
			policyID: "workspace-default-allow",
		},
		{
			name:     "prod requires approval",
			req:      EvaluateRequest{Action: "deploy:prod", WorkspaceID: "ws-1", Env: "prod"},
			decision: RequiresApproval,
			policyID: "prod-requires-approval",
		},
		{
			name:     "deny wins over approval",
			req:      EvaluateRequest{Action: "deploy:prod", WorkspaceID: "ws-1", Env: "prod", DataClassification: "restricted"},
			decision: Deny,
			policyID: "deny-restricted-data-prod",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := engine.Evaluate(tt.req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.Decision != tt.decision || resp.PolicyID != tt.policyID {
				t.Fatalf("got %s/%s, want %s/%s", resp.Decision, resp.PolicyID, tt.decision, tt.policyID)
			}
		})
	}
}
