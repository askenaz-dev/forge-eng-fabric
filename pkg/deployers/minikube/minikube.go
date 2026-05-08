// Package minikube implements the Deployer interface for local minikube/kind.
package minikube

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

func (c *Connector) Type() rt.Type { return rt.TypeMinikube }

func (c *Connector) Capabilities() rt.Capabilities { return rt.DefaultCapabilities(rt.TypeMinikube) }

func (c *Connector) Preflight(ctx context.Context, runtime *rt.Runtime) deployers.PreflightResult {
	out, _, err := c.Runner.Run(ctx, "kubectl", "version", "--client=true", "--output=yaml")
	if err != nil {
		return deployers.PreflightResult{Passed: false, Reason: "kubectl_unavailable", Detail: map[string]any{"err": err.Error()}}
	}
	return deployers.PreflightResult{Passed: true, Detail: map[string]any{"client": out}}
}

func (c *Connector) Render(_ context.Context, m deployers.Manifest, p deployers.Params) (deployers.RenderedArtifacts, error) {
	yaml := m.K8sYAML
	if yaml == "" {
		yaml = renderInlineYAML(m, p)
	}
	sum := sha256.Sum256([]byte(yaml))
	return deployers.RenderedArtifacts{
		Files:       map[string]string{"deployment.yaml": yaml},
		ManifestSHA: hex.EncodeToString(sum[:]),
	}, nil
}

func (c *Connector) Apply(ctx context.Context, _ *rt.Runtime, art deployers.RenderedArtifacts, p deployers.Params) (deployers.ApplyResult, error) {
	start := time.Now()
	if _, _, err := c.Runner.Run(ctx, "kubectl", "apply", "-f", "-"); err != nil {
		return deployers.ApplyResult{Outcome: "failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	if _, _, err := c.Runner.Run(ctx, "kubectl", "rollout", "status", "deployment", "-n", "default", "--timeout=120s"); err != nil {
		return deployers.ApplyResult{Outcome: "failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	return deployers.ApplyResult{
		Outcome:       "ok",
		RevisionID:    p.RevisionID,
		StageDuration: time.Since(start),
		Detail:        map[string]any{"manifest_sha": art.ManifestSHA},
	}, nil
}

func (c *Connector) Verify(ctx context.Context, _ *rt.Runtime, m deployers.Manifest, _ deployers.Params) (deployers.VerifyResult, error) {
	start := time.Now()
	out, _, err := c.Runner.Run(ctx, "kubectl", "rollout", "status", "deployment", m.AppName, "-n", deflt(m.Namespace, "default"))
	if err != nil {
		return deployers.VerifyResult{Healthy: false, FailReason: "rollout_failed", Detail: map[string]any{"err": err.Error()}}, err
	}
	return deployers.VerifyResult{Healthy: true, StageDuration: time.Since(start), Detail: map[string]any{"rollout": out}}, nil
}

func (c *Connector) Rollback(ctx context.Context, _ *rt.Runtime, prev deployers.Manifest, p deployers.Params) (deployers.RollbackResult, error) {
	start := time.Now()
	if _, _, err := c.Runner.Run(ctx, "kubectl", "rollout", "undo", "deployment", prev.AppName, "-n", deflt(prev.Namespace, "default")); err != nil {
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

func renderInlineYAML(m deployers.Manifest, _ deployers.Params) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: %s\n  namespace: %s\n", m.AppName, deflt(m.Namespace, "default"))
	fmt.Fprintf(&b, "spec:\n  replicas: %d\n  selector:\n    matchLabels:\n      app: %s\n", maxInt(m.Replicas, 1), m.AppName)
	fmt.Fprintf(&b, "  template:\n    metadata:\n      labels:\n        app: %s\n    spec:\n      containers:\n      - name: app\n        image: %s\n", m.AppName, m.Image)
	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
