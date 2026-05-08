package minikube

import (
	"context"
	"testing"

	"github.com/forge-eng-fabric/pkg/deployers"
	rt "github.com/forge-eng-fabric/pkg/runtimes"
)

func TestMinikubeApplyInvokesKubectl(t *testing.T) {
	runner := deployers.NewFakeRunner()
	c := New(runner)
	if c.Type() != rt.TypeMinikube {
		t.Fatalf("type")
	}
	art, err := c.Render(context.Background(), deployers.Manifest{AppName: "demo", Image: "demo:abc"}, deployers.Params{})
	if err != nil {
		t.Fatal(err)
	}
	if art.ManifestSHA == "" {
		t.Fatalf("expected manifest sha")
	}
	if _, err := c.Apply(context.Background(), &rt.Runtime{Type: rt.TypeMinikube}, art, deployers.Params{RevisionID: "rev-1"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.Calls("kubectl apply")) != 1 {
		t.Fatalf("expected kubectl apply, got %v", runner.Log)
	}
	if len(runner.Calls("kubectl rollout")) != 1 {
		t.Fatalf("expected kubectl rollout status")
	}
}

func TestMinikubeRollbackUndoes(t *testing.T) {
	runner := deployers.NewFakeRunner()
	c := New(runner)
	if _, err := c.Rollback(context.Background(), &rt.Runtime{Type: rt.TypeMinikube}, deployers.Manifest{AppName: "demo"}, deployers.Params{PrevRevisionID: "rev-prev"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.Calls("kubectl rollout undo")) != 1 {
		t.Fatalf("expected kubectl rollout undo, got %v", runner.Log)
	}
}
