package gke

import (
	"context"
	"strings"
	"testing"

	"github.com/forge-eng-fabric/pkg/deployers"
	rt "github.com/forge-eng-fabric/pkg/runtimes"
)

func TestGKEApplyCanaryUsesHelmAndKubectl(t *testing.T) {
	runner := deployers.NewFakeRunner()
	c := New(runner)
	r := &rt.Runtime{Type: rt.TypeGKE, Endpoint: "https://gke", ProjectID: "p", ClusterName: "c", Region: "us-central1", Namespace: "apps"}
	art, err := c.Render(context.Background(), deployers.Manifest{AppName: "demo", Image: "demo:1"}, deployers.Params{Strategy: deployers.StrategyCanary, CanaryPercent: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Apply(context.Background(), r, art, deployers.Params{Strategy: deployers.StrategyCanary, CanaryPercent: 10}); err != nil {
		t.Fatal(err)
	}
	if len(runner.Calls("helm")) == 0 {
		t.Fatalf("expected helm call, got %v", runner.Log)
	}
	if len(runner.Calls("kubectl rollout")) == 0 {
		t.Fatalf("expected kubectl rollout status, got %v", runner.Log)
	}
	// canary args present
	found := false
	for _, c := range runner.Log {
		for _, a := range c.Args {
			if strings.Contains(a, "canary.enabled=true") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected canary helm flag, got %+v", runner.Log)
	}
}

func TestGKECapabilitiesSpecConformance(t *testing.T) {
	cap := New(nil).Capabilities()
	if !cap.SupportsCanary || !cap.SupportsBlueGreen || !cap.SupportsSecretsCSI {
		t.Fatalf("expected gke caps per spec, got %+v", cap)
	}
}
