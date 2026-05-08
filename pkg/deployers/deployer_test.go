package deployers

import (
	"context"
	"testing"

	rt "github.com/forge-eng-fabric/pkg/runtimes"
)

func TestRegistryLookup(t *testing.T) {
	r := NewRegistry(stubDeployer{rt.TypeGKE})
	d, err := r.For(rt.TypeGKE)
	if err != nil {
		t.Fatal(err)
	}
	if d.Type() != rt.TypeGKE {
		t.Fatalf("wrong deployer: %s", d.Type())
	}
	if _, err := r.For(rt.TypeCloudRun); err == nil {
		t.Fatalf("expected error for unregistered type")
	}
}

type stubDeployer struct{ t rt.Type }

func (s stubDeployer) Type() rt.Type                                          { return s.t }
func (s stubDeployer) Capabilities() rt.Capabilities                          { return rt.Capabilities{} }
func (s stubDeployer) Preflight(context.Context, *rt.Runtime) PreflightResult { return PreflightResult{Passed: true} }
func (s stubDeployer) Render(context.Context, Manifest, Params) (RenderedArtifacts, error) {
	return RenderedArtifacts{}, nil
}
func (s stubDeployer) Apply(context.Context, *rt.Runtime, RenderedArtifacts, Params) (ApplyResult, error) {
	return ApplyResult{Outcome: "ok"}, nil
}
func (s stubDeployer) Verify(context.Context, *rt.Runtime, Manifest, Params) (VerifyResult, error) {
	return VerifyResult{Healthy: true}, nil
}
func (s stubDeployer) Rollback(context.Context, *rt.Runtime, Manifest, Params) (RollbackResult, error) {
	return RollbackResult{Outcome: "ok"}, nil
}

func TestFakeRunnerLogsAndPrefixMatch(t *testing.T) {
	r := NewFakeRunner()
	r.Stdout["kubectl rollout"] = "rollout ok"
	out, _, err := r.Run(context.Background(), "kubectl", "rollout", "status", "deployment", "x")
	if err != nil {
		t.Fatal(err)
	}
	if out != "rollout ok" {
		t.Fatalf("expected stdout, got %q", out)
	}
	if len(r.Calls("kubectl rollout")) != 1 {
		t.Fatalf("expected 1 kubectl rollout call")
	}
}
