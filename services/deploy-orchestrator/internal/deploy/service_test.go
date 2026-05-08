package deploy

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/forge-eng-fabric/pkg/cosign"
	"github.com/forge-eng-fabric/pkg/deployers"
	"github.com/forge-eng-fabric/pkg/deployers/minikube"
	rt "github.com/forge-eng-fabric/pkg/runtimes"
)

func newTestService(t *testing.T) (*Service, *MemorySink, *RuntimeStoreProvider) {
	t.Helper()
	store := NewStore()
	registry := deployers.NewRegistry(minikube.New(deployers.NewFakeRunner()))
	sink := &MemorySink{}
	svc := NewService(store, registry, sink)
	rp := NewRuntimeStoreProvider()
	rp.Set(&rt.Runtime{ID: "rt-1", WorkspaceID: "ws-1", TenantID: "tenant-a", Type: rt.TypeMinikube, Mode: rt.ModeBYO})
	svc.Runtimes = rp
	svc.ImageMetadata = staticMetadataResolver{}
	return svc, sink, rp
}

type staticMetadataResolver struct{}

func (staticMetadataResolver) Resolve(_ context.Context, image, digest string) (cosign.ImageMetadata, error) {
	return cosign.ImageMetadata{
		Image: image, Digest: digest, Signed: true,
		OIDCIssuer:    "https://token.actions.githubusercontent.com",
		OIDCIdentity:  "https://github.com/forge-org/" + image + "@refs/heads/main",
		HasRekorEntry: true, RekorLogIndex: 7,
		AttestationType: "slsaprovenance", AttestationValid: true,
	}, nil
}

type unsignedMetadataResolver struct{}

func (unsignedMetadataResolver) Resolve(_ context.Context, image, digest string) (cosign.ImageMetadata, error) {
	return cosign.ImageMetadata{Image: image, Digest: digest, Signed: false}, nil
}

func sampleRequest() *DeployRequest {
	return &DeployRequest{
		RequestID: "req-1", WorkspaceID: "ws-1", TenantID: "tenant-a",
		AssetID: "app-foo", Env: "dev", RuntimeID: "rt-1",
		Image: "app-foo:abc123", ImageDigest: "sha256:abc",
		PRSHA: "deadbeef", OpenSpecIDs: []string{"spec-1"},
		Strategy: deployers.StrategyRolling,
		Manifest: deployers.Manifest{AppName: "app-foo", Image: "app-foo:abc123", Namespace: "default"},
		Actor:    "alice@example.com",
	}
}

func TestSuccessfulDeployEmitsEventsInOrder(t *testing.T) {
	svc, sink, _ := newTestService(t)
	resp, err := svc.Deploy(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if resp.Status != string(StatusCompleted) {
		t.Fatalf("expected completed, got %s reason=%s", resp.Status, resp.Reason)
	}
	want := []string{"deployment.requested.v1", "deployment.policy_evaluated.v1", "deployment.image_verified.v1", "deployment.applied.v1", "deployment.verified.v1"}
	idx := 0
	for _, ev := range sink.Events {
		if idx < len(want) && ev.Type == want[idx] {
			idx++
		}
	}
	if idx != len(want) {
		t.Fatalf("expected events in order %v, got %s", want, eventTypes(sink.Events))
	}
	if !resp.Deployment.VerifiedSignature || !resp.Deployment.VerifiedAttestation {
		t.Fatalf("expected verified flags, got %+v", resp.Deployment)
	}
}

func TestDuplicateRequestIsIdempotent(t *testing.T) {
	svc, _, _ := newTestService(t)
	first, err := svc.Deploy(context.Background(), sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Deploy(context.Background(), sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || second.Created {
		t.Fatalf("expected first.created=true second.created=false, got %v/%v", first.Created, second.Created)
	}
	if first.Deployment.ID != second.Deployment.ID {
		t.Fatalf("expected same deployment id, got %s vs %s", first.Deployment.ID, second.Deployment.ID)
	}
}

func TestUnsignedImageBlocksDeploy(t *testing.T) {
	svc, sink, _ := newTestService(t)
	svc.ImageMetadata = unsignedMetadataResolver{}
	resp, err := svc.Deploy(context.Background(), sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != string(StatusFailed) {
		t.Fatalf("expected failed, got %s", resp.Status)
	}
	verEvs := sink.ByType("deployment.image_verified.v1")
	if len(verEvs) != 1 {
		t.Fatalf("expected 1 image_verified event, got %d", len(verEvs))
	}
	if verEvs[0].Data["outcome"] != "failed" {
		t.Fatalf("expected failed outcome, got %v", verEvs[0].Data)
	}
}

func TestProdRequiresApprovalCreatesPendingApproval(t *testing.T) {
	svc, sink, _ := newTestService(t)
	svc.ApprovalsInbox = &fakeInbox{}
	req := sampleRequest()
	req.Env = "prod"
	resp, err := svc.Deploy(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != string(StatusPendingApproval) {
		t.Fatalf("expected pending_approval, got %s reason=%s", resp.Status, resp.Reason)
	}
	if resp.Reason != "pending_approval" {
		t.Fatalf("expected pending_approval reason, got %s", resp.Reason)
	}
	if len(svc.ApprovalsInbox.(*fakeInbox).items) != 1 {
		t.Fatalf("expected 1 approval inbox item")
	}
	if len(sink.ByType("deployment.policy_evaluated.v1")) != 1 {
		t.Fatalf("expected policy_evaluated event")
	}
}

func TestFreezeWindowBlocksDeploy(t *testing.T) {
	svc, _, _ := newTestService(t)
	svc.FreezeWindows = []FreezeWindow{{Env: "prod", StartDOW: time.Friday, StartHour: 18, EndDOW: time.Monday, EndHour: 8}}
	svc.Now = func() time.Time {
		return time.Date(2025, 1, 4, 10, 0, 0, 0, time.UTC) // Saturday 10:00 UTC
	}
	approvals := NewInMemoryApprovals()
	approvals.Approve("ws-1", "app-foo", "prod", "sha256:abc", svc.Now().Add(2*time.Hour))
	svc.Approvals = approvals
	req := sampleRequest()
	req.Env = "prod"
	resp, err := svc.Deploy(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != string(StatusBlocked) {
		t.Fatalf("expected blocked, got %s reason=%s", resp.Status, resp.Reason)
	}
	if !strings.Contains(resp.Reason, "freeze_window_active") {
		t.Fatalf("expected freeze_window_active, got %s", resp.Reason)
	}
}

func TestRollingOnCriticalProdDenied(t *testing.T) {
	svc, _, _ := newTestService(t)
	approvals := NewInMemoryApprovals()
	approvals.Approve("ws-1", "app-foo", "prod", "sha256:abc", time.Now().Add(2*time.Hour))
	svc.Approvals = approvals
	req := sampleRequest()
	req.Env = "prod"
	req.Criticality = "critical"
	req.RollbackPlan = "drain workers, restore previous revision"
	req.Strategy = deployers.StrategyRolling
	resp, err := svc.Deploy(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != string(StatusBlocked) {
		t.Fatalf("expected blocked, got %s reason=%s", resp.Status, resp.Reason)
	}
	if !strings.Contains(resp.Reason, "strategy_not_allowed_for_criticality") {
		t.Fatalf("expected strategy denied, got %s", resp.Reason)
	}
}

func TestRollbackPlanRequiredForHighCriticality(t *testing.T) {
	svc, _, _ := newTestService(t)
	approvals := NewInMemoryApprovals()
	approvals.Approve("ws-1", "app-foo", "prod", "sha256:abc", time.Now().Add(2*time.Hour))
	svc.Approvals = approvals
	req := sampleRequest()
	req.Env = "prod"
	req.Criticality = "high"
	req.Strategy = deployers.StrategyCanary
	req.CanaryPercent = 10
	resp, err := svc.Deploy(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != string(StatusBlocked) || !strings.Contains(resp.Reason, "rollback_plan_missing") {
		t.Fatalf("expected rollback_plan_missing, got status=%s reason=%s", resp.Status, resp.Reason)
	}
}

func TestPolicyEngineDecisionParticipatesInPolicyStage(t *testing.T) {
	svc, _, _ := newTestService(t)
	svc.PolicyEngine = fakePolicyEngine{decision: PolicyDecision{
		PolicyID: "external-deny", Decision: DecisionDeny, Reason: "external_policy_denied", Rationale: "central policy denied deployment",
	}}
	resp, err := svc.Deploy(context.Background(), sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != string(StatusBlocked) || resp.Reason != "external_policy_denied" {
		t.Fatalf("expected external policy denial, got status=%s reason=%s", resp.Status, resp.Reason)
	}
	evals := svc.Store.PolicyEvals(resp.Deployment.ID)
	if len(evals) == 0 || evals[0].PolicyID != "external-deny" {
		t.Fatalf("expected external policy eval first, got %+v", evals)
	}
}

func TestRollbackRestoresPreviousRevision(t *testing.T) {
	svc, sink, _ := newTestService(t)
	first, err := svc.Deploy(context.Background(), sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	second := sampleRequest()
	second.RequestID = "req-2"
	second.Image = "app-foo:def456"
	second.ImageDigest = "sha256:def"
	second.Manifest.Image = second.Image
	if _, err := svc.Deploy(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	rb, err := svc.Rollback(context.Background(), latestDeployment(svc, "app-foo", "dev").ID, RollbackRequest{Reason: "manual rollback", Manual: true, Approved: true, Actor: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if rb.Status != string(StatusCompleted) {
		t.Fatalf("expected rollback completed, got %s reason=%s", rb.Status, rb.Reason)
	}
	if rb.Deployment.RollbackOf == "" {
		t.Fatalf("expected rollback_of pointer to source deployment")
	}
	if rb.Deployment.SourceRevisionID == "" {
		t.Fatalf("expected source_revision_id")
	}
	if len(sink.ByType("deployment.rolled_back.v1")) != 1 {
		t.Fatalf("expected rolled_back event")
	}
	_ = first
}

func TestAutoRollbackOnVerifyFailure(t *testing.T) {
	store := NewStore()
	registry := deployers.NewRegistry(minikube.New(deployers.NewFakeRunner()))
	sink := &MemorySink{}
	svc := NewService(store, registry, sink)
	rp := NewRuntimeStoreProvider()
	rp.Set(&rt.Runtime{ID: "rt-1", WorkspaceID: "ws-1", TenantID: "tenant-a", Type: rt.TypeMinikube})
	svc.Runtimes = rp
	svc.ImageMetadata = staticMetadataResolver{}
	if _, err := svc.Deploy(context.Background(), sampleRequest()); err != nil {
		t.Fatal(err)
	}
	svc.Deployers = deployers.NewRegistry(failingVerifyDeployer{})
	second := sampleRequest()
	second.RequestID = "req-2"
	svc.AutoRollbackByEnv = map[string]bool{"dev": true}
	resp, err := svc.Deploy(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != string(StatusCompleted) {
		t.Fatalf("expected rollback deployment completed, got %s reason=%s", resp.Status, resp.Reason)
	}
	if resp.Deployment.SourceRevisionID == "" || resp.Deployment.RollbackOf == "" {
		t.Fatalf("expected rollback deployment to link source revision and rollback_of, got %+v", resp.Deployment)
	}
	if !contains(eventTypes(sink.Events), "deployment.failed.v1") {
		t.Fatalf("expected deployment.failed.v1 emitted")
	}
	if !contains(eventTypes(sink.Events), "deployment.rolled_back.v1") {
		t.Fatalf("expected deployment.rolled_back.v1 emitted, got %v", eventTypes(sink.Events))
	}
	_ = resp
}

func TestStreamReplaysAndStreams(t *testing.T) {
	svc, _, _ := newTestService(t)
	resp, err := svc.Deploy(context.Background(), sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	events := svc.Store.Events(resp.Deployment.ID)
	if len(events) == 0 {
		t.Fatalf("expected stage events to be persisted")
	}
}

type fakeInbox struct {
	items []ApprovalInboxItem
}

type fakePolicyEngine struct {
	decision PolicyDecision
}

func (f fakePolicyEngine) Evaluate(context.Context, *DeployRequest) (PolicyDecision, error) {
	return f.decision, nil
}

func (f *fakeInbox) CreateDeploymentApproval(_ context.Context, item ApprovalInboxItem) error {
	f.items = append(f.items, item)
	return nil
}

type failingVerifyDeployer struct{}

func (failingVerifyDeployer) Type() rt.Type                 { return rt.TypeMinikube }
func (failingVerifyDeployer) Capabilities() rt.Capabilities { return rt.Capabilities{} }
func (failingVerifyDeployer) Preflight(context.Context, *rt.Runtime) deployers.PreflightResult {
	return deployers.PreflightResult{Passed: true}
}
func (failingVerifyDeployer) Render(context.Context, deployers.Manifest, deployers.Params) (deployers.RenderedArtifacts, error) {
	return deployers.RenderedArtifacts{ManifestSHA: "abc"}, nil
}
func (failingVerifyDeployer) Apply(context.Context, *rt.Runtime, deployers.RenderedArtifacts, deployers.Params) (deployers.ApplyResult, error) {
	return deployers.ApplyResult{Outcome: "ok"}, nil
}
func (failingVerifyDeployer) Verify(context.Context, *rt.Runtime, deployers.Manifest, deployers.Params) (deployers.VerifyResult, error) {
	return deployers.VerifyResult{Healthy: false, FailReason: "rollout_failed"}, nil
}
func (failingVerifyDeployer) Rollback(context.Context, *rt.Runtime, deployers.Manifest, deployers.Params) (deployers.RollbackResult, error) {
	return deployers.RollbackResult{Outcome: "ok"}, nil
}

func eventTypes(evs []CloudEvent) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.Type
	}
	return out
}

func contains(xs []string, target string) bool {
	for _, x := range xs {
		if x == target {
			return true
		}
	}
	return false
}

func latestDeployment(svc *Service, asset, env string) *Deployment {
	all := svc.Store.List("", "")
	for _, d := range all {
		if d.AssetID == asset && d.Env == env && d.RollbackOf == "" {
			return d
		}
	}
	return nil
}
