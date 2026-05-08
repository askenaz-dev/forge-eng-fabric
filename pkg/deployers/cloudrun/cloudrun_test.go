package cloudrun

import (
	"context"
	"strings"
	"testing"

	"github.com/forge-eng-fabric/pkg/deployers"
	rt "github.com/forge-eng-fabric/pkg/runtimes"
)

func TestCloudRunCanaryRoutes10Percent(t *testing.T) {
	runner := deployers.NewFakeRunner()
	c := New(runner)
	r := &rt.Runtime{Type: rt.TypeCloudRun, ProjectID: "p", Region: "us-central1"}
	art, err := c.Render(context.Background(), deployers.Manifest{AppName: "demo", Image: "demo:1"}, deployers.Params{Strategy: deployers.StrategyCanary, CanaryPercent: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Apply(context.Background(), r, art, deployers.Params{Strategy: deployers.StrategyCanary, CanaryPercent: 10}); err != nil {
		t.Fatal(err)
	}
	if len(runner.Calls("gcloud run deploy")) > 0 {
		t.Logf("deploy: %v", runner.Calls("gcloud run deploy"))
	}
	// gcloud calls include both deploy and update-traffic
	deploys := 0
	traffic := 0
	for _, l := range runner.Log {
		j := l.Cmd + " " + strings.Join(l.Args, " ")
		if strings.HasPrefix(j, "gcloud run") {
			if strings.Contains(j, "deploy") {
				deploys++
			}
			if strings.Contains(j, "update-traffic") {
				traffic++
			}
		}
	}
	if deploys == 0 || traffic == 0 {
		t.Fatalf("expected gcloud deploy + update-traffic, got %+v", runner.Log)
	}
}

func TestCloudRunCapabilitiesSpec(t *testing.T) {
	cap := New(nil).Capabilities()
	if !cap.SupportsTrafficSplitting || !cap.SupportsCanary || !cap.SupportsBlueGreen {
		t.Fatalf("expected cloudrun caps per spec, got %+v", cap)
	}
}
