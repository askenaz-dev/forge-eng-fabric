package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Decision mirrors the policy-engine vocabulary so the orchestrator can be
// bound either to the embedded policy templates (default) or to the
// policy-engine HTTP service.
type Decision string

const (
	DecisionAllow            Decision = "allow"
	DecisionRequiresApproval Decision = "requires_approval"
	DecisionDeny             Decision = "deny"
)

type PolicyDecision struct {
	PolicyID          string   `json:"policy_id"`
	Decision          Decision `json:"decision"`
	Rationale         string   `json:"rationale"`
	Reason            string   `json:"reason,omitempty"`
	RequiredApprovers []string `json:"required_approvers,omitempty"`
}

// DeploymentPolicy is a single template enforced before Apply. The default
// set is shipped by `DefaultDeploymentPolicies()` per the
// `deployment-policies` and `policies-and-approvals` specs.
type DeploymentPolicy interface {
	ID() string
	Evaluate(req *DeployRequest, ctx PolicyContext) PolicyDecision
}

type PolicyContext struct {
	HasApproval                 bool
	ApprovalRevisionID          string
	ApprovalExpiresAt           time.Time
	Now                         time.Time
	FreezeWindows               []FreezeWindow
	OverrideAllowUnsignedTTL    time.Time
	OverrideAllowUnsignedActive bool
}

type ExternalPolicyEvaluator interface {
	Evaluate(ctx context.Context, req *DeployRequest) (PolicyDecision, error)
}

type PolicyEngineClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

type policyEngineEvaluateRequest struct {
	Principal          string         `json:"principal"`
	Action             string         `json:"action"`
	WorkspaceID        string         `json:"workspace_id"`
	AssetID            *string        `json:"asset_id,omitempty"`
	Env                string         `json:"env"`
	Criticality        string         `json:"criticality"`
	DataClassification string         `json:"data_classification"`
	Target             map[string]any `json:"target"`
}

type policyEngineEvaluateResponse struct {
	Decision          Decision `json:"decision"`
	Rationale         string   `json:"rationale"`
	PolicyID          string   `json:"policy_id,omitempty"`
	RequiredApprovers []string `json:"required_approvers,omitempty"`
}

func (c PolicyEngineClient) Evaluate(ctx context.Context, req *DeployRequest) (PolicyDecision, error) {
	if c.BaseURL == "" {
		return PolicyDecision{}, fmt.Errorf("policy_engine_base_url_required")
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	body := policyEngineEvaluateRequest{
		Principal:          req.Actor,
		Action:             "deploy:" + req.Env,
		WorkspaceID:        req.WorkspaceID,
		AssetID:            &req.AssetID,
		Env:                req.Env,
		Criticality:        req.Criticality,
		DataClassification: req.DataClassification,
		Target: map[string]any{
			"runtime_id":     req.RuntimeID,
			"image":          req.Image,
			"image_digest":   req.ImageDigest,
			"strategy":       req.Strategy,
			"canary_percent": req.CanaryPercent,
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return PolicyDecision{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/v1/evaluate", bytes.NewReader(payload))
	if err != nil {
		return PolicyDecision{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	resp, err := hc.Do(httpReq)
	if err != nil {
		return PolicyDecision{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PolicyDecision{}, fmt.Errorf("policy_engine_status_%d", resp.StatusCode)
	}
	var out policyEngineEvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return PolicyDecision{}, err
	}
	policyID := out.PolicyID
	if policyID == "" {
		policyID = "policy-engine"
	}
	return PolicyDecision{PolicyID: policyID, Decision: out.Decision, Rationale: out.Rationale, RequiredApprovers: out.RequiredApprovers}, nil
}

type FreezeWindow struct {
	Env       string       `json:"env"`
	Reason    string       `json:"reason,omitempty"`
	StartDOW  time.Weekday `json:"start_dow"`
	StartHour int          `json:"start_hour"`
	EndDOW    time.Weekday `json:"end_dow"`
	EndHour   int          `json:"end_hour"`
}

// In returns true if `now` falls inside the freeze interval. Wrapping
// across week boundaries is supported.
func (f FreezeWindow) In(now time.Time) bool {
	current := dayHour(now.Weekday(), now.Hour())
	start := dayHour(f.StartDOW, f.StartHour)
	end := dayHour(f.EndDOW, f.EndHour)
	if start <= end {
		return current >= start && current < end
	}
	// wraps Sat→Mon
	return current >= start || current < end
}

func dayHour(d time.Weekday, h int) int {
	return int(d)*24 + h
}

// === Built-in policies =====================================================

type requireSignedImage struct{}

func (requireSignedImage) ID() string { return "require-signed-image" }

func (requireSignedImage) Evaluate(req *DeployRequest, ctx PolicyContext) PolicyDecision {
	if ctx.OverrideAllowUnsignedActive && ctx.Now.Before(ctx.OverrideAllowUnsignedTTL) {
		return PolicyDecision{PolicyID: "require-signed-image", Decision: DecisionAllow, Rationale: "override-allow-unsigned-image active"}
	}
	// The actual signature check is a separate stage; this policy only
	// states the rule is mandatory and never globally disabled.
	return PolicyDecision{PolicyID: "require-signed-image", Decision: DecisionAllow, Rationale: "active by default"}
}

type requireApprovalProd struct{}

func (requireApprovalProd) ID() string { return "require-approval-prod" }

func (requireApprovalProd) Evaluate(req *DeployRequest, ctx PolicyContext) PolicyDecision {
	if !strings.EqualFold(req.Env, "prod") {
		return PolicyDecision{PolicyID: "require-approval-prod", Decision: DecisionAllow, Rationale: "env not prod"}
	}
	if !ctx.HasApproval {
		return PolicyDecision{
			PolicyID:          "require-approval-prod",
			Decision:          DecisionRequiresApproval,
			Reason:            "pending_approval",
			Rationale:         "production deploys require release-manager approval tied to revision",
			RequiredApprovers: []string{"release-manager"},
		}
	}
	if ctx.ApprovalRevisionID != "" && req.ImageDigest != "" && !strings.EqualFold(ctx.ApprovalRevisionID, req.ImageDigest) {
		return PolicyDecision{
			PolicyID: "require-approval-prod", Decision: DecisionDeny,
			Reason: "approval_revision_mismatch", Rationale: "approval is bound to a different revision",
		}
	}
	if !ctx.ApprovalExpiresAt.IsZero() && ctx.Now.After(ctx.ApprovalExpiresAt) {
		return PolicyDecision{
			PolicyID: "require-approval-prod", Decision: DecisionDeny,
			Reason: "approval_expired", Rationale: "approval TTL exceeded (max 8h)",
		}
	}
	return PolicyDecision{PolicyID: "require-approval-prod", Decision: DecisionAllow, Rationale: "approval valid"}
}

type freezeWindowPolicy struct{}

func (freezeWindowPolicy) ID() string { return "freeze-window" }

func (freezeWindowPolicy) Evaluate(req *DeployRequest, ctx PolicyContext) PolicyDecision {
	for _, w := range ctx.FreezeWindows {
		if w.Env != "" && !strings.EqualFold(w.Env, req.Env) {
			continue
		}
		if w.In(ctx.Now) {
			return PolicyDecision{
				PolicyID: "freeze-window", Decision: DecisionDeny,
				Reason: "freeze_window_active", Rationale: "freeze window in effect for env=" + req.Env,
			}
		}
	}
	return PolicyDecision{PolicyID: "freeze-window", Decision: DecisionAllow, Rationale: "no freeze window active"}
}

type requireCanary struct{}

func (requireCanary) ID() string { return "require-canary" }

func (requireCanary) Evaluate(req *DeployRequest, ctx PolicyContext) PolicyDecision {
	if !strings.EqualFold(req.Env, "prod") || !isHighCriticality(req.Criticality) {
		return PolicyDecision{PolicyID: "require-canary", Decision: DecisionAllow, Rationale: "policy not applicable"}
	}
	if req.Strategy == "canary" || req.Strategy == "blue_green" {
		return PolicyDecision{PolicyID: "require-canary", Decision: DecisionAllow, Rationale: "canary or blue/green selected"}
	}
	return PolicyDecision{
		PolicyID: "require-canary", Decision: DecisionDeny,
		Reason: "strategy_not_allowed_for_criticality", Rationale: "criticality=" + req.Criticality + " and env=prod require canary or blue/green strategy",
	}
}

type requireRollbackPlan struct{}

func (requireRollbackPlan) ID() string { return "require-rollback-plan" }

func (requireRollbackPlan) Evaluate(req *DeployRequest, _ PolicyContext) PolicyDecision {
	if !isHighCriticality(req.Criticality) {
		return PolicyDecision{PolicyID: "require-rollback-plan", Decision: DecisionAllow, Rationale: "criticality below high"}
	}
	if strings.TrimSpace(req.RollbackPlan) == "" {
		return PolicyDecision{
			PolicyID: "require-rollback-plan", Decision: DecisionDeny,
			Reason: "rollback_plan_missing", Rationale: "criticality=" + req.Criticality + " requires explicit rollback plan",
		}
	}
	return PolicyDecision{PolicyID: "require-rollback-plan", Decision: DecisionAllow, Rationale: "rollback plan present"}
}

func isHighCriticality(c string) bool {
	switch strings.ToLower(c) {
	case "high", "critical":
		return true
	}
	return false
}

// DefaultDeploymentPolicies returns the templates required by the
// `policies-and-approvals` (Phase 3) spec delta.
func DefaultDeploymentPolicies() []DeploymentPolicy {
	return []DeploymentPolicy{
		requireSignedImage{},
		requireApprovalProd{},
		freezeWindowPolicy{},
		requireCanary{},
		requireRollbackPlan{},
	}
}

// EvaluatePolicies runs each policy and returns the first restrictive
// decision (deny > requires_approval > allow). All evaluations are returned
// for audit.
func EvaluatePolicies(policies []DeploymentPolicy, req *DeployRequest, ctx PolicyContext) (PolicyDecision, []PolicyDecision) {
	all := make([]PolicyDecision, 0, len(policies))
	var deny, approval *PolicyDecision
	for _, p := range policies {
		d := p.Evaluate(req, ctx)
		all = append(all, d)
		switch d.Decision {
		case DecisionDeny:
			if deny == nil {
				cp := d
				deny = &cp
			}
		case DecisionRequiresApproval:
			if approval == nil {
				cp := d
				approval = &cp
			}
		}
	}
	if deny != nil {
		return *deny, all
	}
	if approval != nil {
		return *approval, all
	}
	return PolicyDecision{Decision: DecisionAllow, Rationale: "all policies allowed"}, all
}

func MostRestrictivePolicyDecision(all []PolicyDecision) PolicyDecision {
	var deny, approval *PolicyDecision
	for _, d := range all {
		switch d.Decision {
		case DecisionDeny:
			if deny == nil {
				cp := d
				deny = &cp
			}
		case DecisionRequiresApproval:
			if approval == nil {
				cp := d
				approval = &cp
			}
		}
	}
	if deny != nil {
		return *deny
	}
	if approval != nil {
		return *approval
	}
	return PolicyDecision{Decision: DecisionAllow, Rationale: "all policies allowed"}
}

// ApprovalProvider is consulted for `require-approval-prod`.
type ApprovalProvider interface {
	HasApproval(workspaceID, assetID, env, revisionID string) (bool, time.Time, string, error)
}

// InMemoryApprovals lets tests prime approvals.
type InMemoryApprovals struct {
	approvals map[string]inMemoryApproval
}

type inMemoryApproval struct {
	expiresAt  time.Time
	revisionID string
}

func NewInMemoryApprovals() *InMemoryApprovals {
	return &InMemoryApprovals{approvals: map[string]inMemoryApproval{}}
}

func (a *InMemoryApprovals) Approve(workspaceID, assetID, env, revisionID string, expiresAt time.Time) {
	a.approvals[approvalKey(workspaceID, assetID, env)] = inMemoryApproval{expiresAt: expiresAt, revisionID: revisionID}
}

func (a *InMemoryApprovals) HasApproval(workspaceID, assetID, env, revisionID string) (bool, time.Time, string, error) {
	v, ok := a.approvals[approvalKey(workspaceID, assetID, env)]
	if !ok {
		return false, time.Time{}, "", nil
	}
	return true, v.expiresAt, v.revisionID, nil
}

func approvalKey(workspaceID, assetID, env string) string {
	return workspaceID + "/" + assetID + "/" + env
}
