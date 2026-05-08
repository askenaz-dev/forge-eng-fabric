// Package gke implements the Deployer interface for GKE (Standard or
// Autopilot), driving kubectl + Helm with Workload Identity.
package gke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/forge-eng-fabric/pkg/deployers"
	rt "github.com/forge-eng-fabric/pkg/runtimes"
)

type Connector struct {
	Runner deployers.CommandRunner
	Chart  string
}

func New(runner deployers.CommandRunner) *Connector {
	if runner == nil {
		runner = deployers.NewFakeRunner()
	}
	return &Connector{Runner: runner, Chart: "forge-app-chart"}
}

func (c *Connector) Type() rt.Type { return rt.TypeGKE }

func (c *Connector) Capabilities() rt.Capabilities { return rt.DefaultCapabilities(rt.TypeGKE) }

func (c *Connector) Preflight(ctx context.Context, runtime *rt.Runtime) deployers.PreflightResult {
	if runtime.Endpoint == "" {
		return deployers.PreflightResult{Passed: false, Reason: "endpoint_unreachable"}
	}
	if _, _, err := c.Runner.Run(ctx, "gcloud", "container", "clusters", "describe", runtime.ClusterName, "--region", runtime.Region, "--project", runtime.ProjectID); err != nil {
		return deployers.PreflightResult{Passed: false, Reason: "gcloud_describe_failed", Detail: map[string]any{"err": err.Error()}}
	}
	return deployers.PreflightResult{Passed: true}
}

func (c *Connector) Render(_ context.Context, m deployers.Manifest, p deployers.Params) (deployers.RenderedArtifacts, error) {
	chart := m.HelmChart
	if chart == "" {
		chart = c.Chart
	}
	values := map[string]any{
		"image":      m.Image,
		"replicas":   maxInt(m.Replicas, 1),
		"app":        m.AppName,
		"namespace":  deflt(m.Namespace, "default"),
		"strategy":   string(p.Strategy),
		"healthcheck": map[string]string{"path": "/healthz"},
	}
	for k, v := range m.HelmValues {
		values[k] = v
	}
	rendered := fmt.Sprintf("chart=%s\napp=%s\nimage=%s\nstrategy=%s", chart, m.AppName, m.Image, p.Strategy)
	sum := sha256.Sum256([]byte(rendered))
	return deployers.RenderedArtifacts{
		Files: map[string]string{
			"values.yaml": rendered,
			"chart":       chart,
		},
		ManifestSHA: hex.EncodeToString(sum[:]),
		Notes:       []string{fmt.Sprintf("rendered helm chart %s for %s", chart, m.AppName)},
	}, nil
}

func (c *Connector) Apply(ctx context.Context, runtime *rt.Runtime, art deployers.RenderedArtifacts, p deployers.Params) (deployers.ApplyResult, error) {
	start := time.Now()
	args := []string{"upgrade", "--install", "--namespace", deflt(runtime.Namespace, "default")}
	if p.Strategy == deployers.StrategyCanary {
		args = append(args, "--set", fmt.Sprintf("canary.enabled=true,canary.weight=%d", maxInt(p.CanaryPercent, 10)))
	}
	if p.Strategy == deployers.StrategyBlueGreen {
		args = append(args, "--set", "blueGreen.enabled=true")
	}
	if _, _, err := c.Runner.Run(ctx, "helm", append([]string{"helm"}, args...)...); err != nil {
		return deployers.ApplyResult{Outcome: "failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	if _, _, err := c.Runner.Run(ctx, "kubectl", "rollout", "status", "deployment", "-n", deflt(runtime.Namespace, "default"), "--timeout=600s"); err != nil {
		return deployers.ApplyResult{Outcome: "failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	return deployers.ApplyResult{
		Outcome:       "ok",
		RevisionID:    p.RevisionID,
		StageDuration: time.Since(start),
		Detail:        map[string]any{"manifest_sha": art.ManifestSHA, "chart": c.Chart},
	}, nil
}

func (c *Connector) Verify(ctx context.Context, runtime *rt.Runtime, m deployers.Manifest, _ deployers.Params) (deployers.VerifyResult, error) {
	start := time.Now()
	if _, _, err := c.Runner.Run(ctx, "kubectl", "rollout", "status", "deployment", m.AppName, "-n", deflt(runtime.Namespace, "default"), "--timeout=300s"); err != nil {
		return deployers.VerifyResult{Healthy: false, FailReason: "rollout_failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	url := m.HealthCheckURL
	if url == "" {
		url = fmt.Sprintf("http://%s.%s.svc.cluster.local/healthz", m.AppName, deflt(runtime.Namespace, "default"))
	}
	if _, _, err := c.Runner.Run(ctx, "curl", "-fsSL", url); err != nil {
		return deployers.VerifyResult{Healthy: false, FailReason: "healthcheck_failed", Detail: map[string]any{"err": err.Error(), "url": url}}, err
	}
	return deployers.VerifyResult{Healthy: true, StageDuration: time.Since(start), Detail: map[string]any{"url": url}}, nil
}

func (c *Connector) Rollback(ctx context.Context, runtime *rt.Runtime, prev deployers.Manifest, p deployers.Params) (deployers.RollbackResult, error) {
	start := time.Now()
	if _, _, err := c.Runner.Run(ctx, "helm", "rollback", prev.AppName); err != nil {
		return deployers.RollbackResult{Outcome: "failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	if _, _, err := c.Runner.Run(ctx, "kubectl", "rollout", "status", "deployment", prev.AppName, "-n", deflt(runtime.Namespace, "default")); err != nil {
		return deployers.RollbackResult{Outcome: "failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	return deployers.RollbackResult{
		Outcome:       "ok",
		RestoredRevID: p.PrevRevisionID,
		StageDuration: time.Since(start),
	}, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func deflt(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// Helper kept exported for tests inspecting Helm args.
func HelmCanaryArg(percent int) string {
	return strings.ReplaceAll(fmt.Sprintf("canary.enabled=true,canary.weight=%d", percent), " ", "")
}
