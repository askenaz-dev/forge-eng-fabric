// Package cloudrun implements the Deployer interface for Cloud Run, using
// the gcloud SDK + revision traffic split.
package cloudrun

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
}

func New(runner deployers.CommandRunner) *Connector {
	if runner == nil {
		runner = deployers.NewFakeRunner()
	}
	return &Connector{Runner: runner}
}

func (c *Connector) Type() rt.Type { return rt.TypeCloudRun }

func (c *Connector) Capabilities() rt.Capabilities { return rt.DefaultCapabilities(rt.TypeCloudRun) }

func (c *Connector) Preflight(ctx context.Context, runtime *rt.Runtime) deployers.PreflightResult {
	if runtime.ProjectID == "" {
		return deployers.PreflightResult{Passed: false, Reason: "project_id_missing"}
	}
	if _, _, err := c.Runner.Run(ctx, "gcloud", "run", "services", "list", "--project", runtime.ProjectID, "--region", deflt(runtime.Region, "us-central1")); err != nil {
		return deployers.PreflightResult{Passed: false, Reason: "gcloud_run_list_failed", Detail: map[string]any{"err": err.Error()}}
	}
	return deployers.PreflightResult{Passed: true}
}

func (c *Connector) Render(_ context.Context, m deployers.Manifest, p deployers.Params) (deployers.RenderedArtifacts, error) {
	image := m.CloudRunImage
	if image == "" {
		image = m.Image
	}
	rendered := fmt.Sprintf("service=%s\nimage=%s\nstrategy=%s", m.AppName, image, p.Strategy)
	sum := sha256.Sum256([]byte(rendered))
	return deployers.RenderedArtifacts{
		Files:       map[string]string{"service.yaml": rendered},
		ManifestSHA: hex.EncodeToString(sum[:]),
	}, nil
}

func (c *Connector) Apply(ctx context.Context, runtime *rt.Runtime, _ deployers.RenderedArtifacts, p deployers.Params) (deployers.ApplyResult, error) {
	start := time.Now()
	args := []string{"run", "deploy", "--platform", "managed", "--region", deflt(runtime.Region, "us-central1"), "--project", runtime.ProjectID}
	if p.Strategy == deployers.StrategyCanary || p.Strategy == deployers.StrategyBlueGreen {
		// New revision without auto-traffic; we route after.
		args = append(args, "--no-traffic", "--tag", "candidate")
	}
	if _, _, err := c.Runner.Run(ctx, "gcloud", args...); err != nil {
		return deployers.ApplyResult{Outcome: "failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	if p.Strategy == deployers.StrategyCanary {
		percent := p.CanaryPercent
		if percent <= 0 {
			percent = 10
		}
		split := fmt.Sprintf("candidate=%d", percent)
		if _, _, err := c.Runner.Run(ctx, "gcloud", "run", "services", "update-traffic", "--to-tags", split, "--project", runtime.ProjectID, "--region", deflt(runtime.Region, "us-central1")); err != nil {
			return deployers.ApplyResult{Outcome: "failed", Detail: map[string]any{"err": err.Error()}}, err
		}
	}
	if p.Strategy == deployers.StrategyRolling || p.Strategy == "" {
		if _, _, err := c.Runner.Run(ctx, "gcloud", "run", "services", "update-traffic", "--to-latest", "--project", runtime.ProjectID, "--region", deflt(runtime.Region, "us-central1")); err != nil {
			return deployers.ApplyResult{Outcome: "failed", Detail: map[string]any{"err": err.Error()}}, err
		}
	}
	return deployers.ApplyResult{
		Outcome:       "ok",
		RevisionID:    p.RevisionID,
		StageDuration: time.Since(start),
		Detail:        map[string]any{"strategy": string(p.Strategy)},
	}, nil
}

func (c *Connector) Verify(ctx context.Context, runtime *rt.Runtime, m deployers.Manifest, _ deployers.Params) (deployers.VerifyResult, error) {
	start := time.Now()
	out, _, err := c.Runner.Run(ctx, "gcloud", "run", "services", "describe", m.AppName, "--project", runtime.ProjectID, "--region", deflt(runtime.Region, "us-central1"), "--format", "value(status.conditions[?(@.type==Ready)].status)")
	if err != nil {
		return deployers.VerifyResult{Healthy: false, FailReason: "describe_failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	if !strings.Contains(out, "True") && out != "" {
		return deployers.VerifyResult{Healthy: false, FailReason: "service_not_ready", Detail: map[string]any{"status": out}}, nil
	}
	return deployers.VerifyResult{Healthy: true, StageDuration: time.Since(start), Detail: map[string]any{"status": out}}, nil
}

func (c *Connector) Rollback(ctx context.Context, runtime *rt.Runtime, prev deployers.Manifest, p deployers.Params) (deployers.RollbackResult, error) {
	start := time.Now()
	if _, _, err := c.Runner.Run(ctx, "gcloud", "run", "services", "update-traffic", prev.AppName, "--to-revisions", p.PrevRevisionID+"=100", "--project", runtime.ProjectID, "--region", deflt(runtime.Region, "us-central1")); err != nil {
		return deployers.RollbackResult{Outcome: "failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	return deployers.RollbackResult{
		Outcome:       "ok",
		RestoredRevID: p.PrevRevisionID,
		StageDuration: time.Since(start),
	}, nil
}

func deflt(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
