// Package opa wraps OPA evaluation with bundle-hash capture.
package opa

import (
	"context"
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/rego"
)

// Client evaluates OPA policies from a bundle directory.
type Client struct {
	bundleDir  string
	bundleHash string
}

// New creates a Client loading policies from bundleDir.
// The bundle hash is read from the .bundle-hash file produced by build-alfred-bundle.sh.
func New(bundleDir string) (*Client, error) {
	hash := ""
	if data, err := os.ReadFile(".bundle-hash"); err == nil {
		hash = string(data)
	}
	return &Client{bundleDir: bundleDir, bundleHash: hash}, nil
}

// BundleHash returns the SHA-256 hash of the loaded policy bundle.
func (c *Client) BundleHash() string { return c.bundleHash }

// NewWithHash creates a Client with an explicit bundle hash — intended for tests.
func NewWithHash(bundleDir, hash string) *Client {
	return &Client{bundleDir: bundleDir, bundleHash: hash}
}

// RiskDecision holds the full output of the risk-classifier policy evaluation.
type RiskDecision struct {
	AutonomyDecision      string
	SandboxMinTier        int
	Approvers             []string
	ApprovalMode          string // "any" | "dual"
	SelfRevokeWindowSecs  int
}

// EvalRiskClassifier evaluates the alfred risk-classifier policy.
// Returns (autonomy_decision, sandbox_min_tier, approvers, error) for backwards compat.
// Use EvalRiskClassifierFull for the complete decision.
func (c *Client) EvalRiskClassifier(ctx context.Context, input map[string]any) (string, int, []string, error) {
	d, err := c.EvalRiskClassifierFull(ctx, input)
	if err != nil {
		return "deny", 0, nil, err
	}
	return d.AutonomyDecision, d.SandboxMinTier, d.Approvers, nil
}

// EvalRiskClassifierFull returns the full risk decision including approval_mode.
func (c *Client) EvalRiskClassifierFull(ctx context.Context, input map[string]any) (RiskDecision, error) {
	r := rego.New(
		rego.Query("data.forge.alfred.risk_classifier"),
		rego.Load([]string{c.bundleDir}, nil),
		rego.Input(input),
	)
	rs, err := r.Eval(ctx)
	if err != nil {
		return RiskDecision{AutonomyDecision: "deny"}, fmt.Errorf("opa eval: %w", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return RiskDecision{AutonomyDecision: "deny"}, fmt.Errorf("opa: empty result set")
	}
	result, _ := rs[0].Expressions[0].Value.(map[string]any)

	d := RiskDecision{SelfRevokeWindowSecs: 60}

	d.AutonomyDecision, _ = result["autonomy_decision"].(string)
	if d.AutonomyDecision == "" {
		d.AutonomyDecision = "deny"
	}
	if tierFloat, ok := result["sandbox_min_tier"].(float64); ok {
		d.SandboxMinTier = int(tierFloat)
	}
	if raw, ok := result["approvers"].([]any); ok {
		for _, a := range raw {
			if s, ok := a.(string); ok {
				d.Approvers = append(d.Approvers, s)
			}
		}
	}
	d.ApprovalMode, _ = result["approval_mode"].(string)
	if d.ApprovalMode == "" {
		d.ApprovalMode = "any"
	}
	if w, ok := result["self_revoke_window_secs"].(float64); ok {
		d.SelfRevokeWindowSecs = int(w)
	}
	return d, nil
}

// ValidateBundleHash returns true when hash matches the loaded bundle hash.
// An empty rowHash is treated as a mismatch (unset field in the audit row).
func (c *Client) ValidateBundleHash(rowHash string) bool {
	if rowHash == "" || c.bundleHash == "" {
		return false
	}
	return rowHash == c.bundleHash
}

// EvalSelfProtection returns true if the target is in the self-protection denylist.
func (c *Client) EvalSelfProtection(ctx context.Context, target string) (bool, error) {
	r := rego.New(
		rego.Query("data.forge.alfred.self_protection.denied"),
		rego.Load([]string{c.bundleDir}, nil),
		rego.Input(map[string]any{"target": target}),
	)
	rs, err := r.Eval(ctx)
	if err != nil {
		return true, fmt.Errorf("opa self-protection eval: %w", err)
	}
	if len(rs) == 0 {
		return false, nil
	}
	denied, _ := rs[0].Expressions[0].Value.(bool)
	return denied, nil
}
