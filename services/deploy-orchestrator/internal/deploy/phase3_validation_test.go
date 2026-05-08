package deploy

import (
	"context"
	"testing"
	"time"

	"github.com/forge-eng-fabric/pkg/deployers"
	"github.com/forge-eng-fabric/pkg/deployers/cloudrun"
	"github.com/forge-eng-fabric/pkg/deployers/gke"
	"github.com/forge-eng-fabric/pkg/deployers/minikube"
	rt "github.com/forge-eng-fabric/pkg/runtimes"
)

func TestPhase3E2EOnboardedAppDeploysAcrossInitialRuntimes(t *testing.T) {
	store := NewStore()
	sink := &MemorySink{}
	svc := NewService(store, deployers.NewRegistry(
		minikube.New(deployers.NewFakeRunner()),
		gke.New(deployers.NewFakeRunner()),
		cloudrun.New(deployers.NewFakeRunner()),
	), sink)
	svc.ImageMetadata = staticMetadataResolver{}
	runtimes := NewRuntimeStoreProvider()
	runtimes.Set(&rt.Runtime{ID: "rt-minikube", WorkspaceID: "ws-1", TenantID: "tenant-a", Type: rt.TypeMinikube, Mode: rt.ModeBYO})
	runtimes.Set(&rt.Runtime{ID: "rt-gke-byo", WorkspaceID: "ws-1", TenantID: "tenant-a", Type: rt.TypeGKE, Mode: rt.ModeBYO, ProjectID: "pilot-prod", Region: "us-central1", ClusterName: "pilot-prod", Endpoint: "https://gke.example.com", Namespace: "apps"})
	runtimes.Set(&rt.Runtime{ID: "rt-cloudrun-provisioned", WorkspaceID: "ws-1", TenantID: "tenant-a", Type: rt.TypeCloudRun, Mode: rt.ModeProvisioned, ProjectID: "forge-ws-1-dev", Region: "us-central1"})
	svc.Runtimes = runtimes

	deploys := []struct {
		name      string
		runtimeID string
		env       string
		strategy  deployers.Strategy
	}{
		{name: "minikube dev", runtimeID: "rt-minikube", env: "dev", strategy: deployers.StrategyRolling},
		{name: "gke byo stage", runtimeID: "rt-gke-byo", env: "stage", strategy: deployers.StrategyCanary},
		{name: "cloud run provisioned dev", runtimeID: "rt-cloudrun-provisioned", env: "dev", strategy: deployers.StrategyCanary},
	}

	for i, tc := range deploys {
		t.Run(tc.name, func(t *testing.T) {
			req := sampleRequest()
			req.RequestID = "phase3-e2e-" + tc.runtimeID
			req.AssetID = "application:ws-1:app-foo"
			req.RuntimeID = tc.runtimeID
			req.Env = tc.env
			req.Strategy = tc.strategy
			req.CanaryPercent = 10
			req.ImageDigest = []string{"sha256:minikube", "sha256:gke", "sha256:cloudrun"}[i]
			req.OpenSpecIDs = []string{"phase-2-app-onboarding", "phase-3-deployable-apps"}
			req.Manifest.AppName = "app-foo"
			req.Manifest.Namespace = "apps"
			resp, err := svc.Deploy(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.Status != string(StatusCompleted) {
				t.Fatalf("expected completed, got %s reason=%s", resp.Status, resp.Reason)
			}
			if resp.Deployment.RevisionID == "" || resp.Deployment.RuntimeID != tc.runtimeID {
				t.Fatalf("deployment missing revision/runtime linkage: %+v", resp.Deployment)
			}
		})
	}
}

func TestPhase3UnsignedImageOverrideFlowCompletesOneDeployment(t *testing.T) {
	svc, sink, _ := newTestService(t)
	svc.ImageMetadata = unsignedMetadataResolver{}
	svc.OverrideAllowUnsigned = func(workspaceID, deploymentID string) (bool, time.Time) {
		return workspaceID == "ws-1" && deploymentID != "", time.Now().Add(30 * time.Minute)
	}
	resp, err := svc.Deploy(context.Background(), sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != string(StatusCompleted) {
		t.Fatalf("expected override-backed deploy to complete, got %s", resp.Status)
	}
	results := svc.Store.ImageVerifications(resp.Deployment.ID)
	if len(results) != 1 || results[0].Outcome != "skipped" {
		t.Fatalf("expected skipped image verification result, got %+v", results)
	}
	if len(sink.ByType("policy.override.consumed.v1")) != 1 {
		t.Fatalf("expected override consumed event")
	}
}
