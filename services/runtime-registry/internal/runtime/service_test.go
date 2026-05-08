package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func newTestService(t *testing.T) (*Service, *MemorySink) {
	t.Helper()
	sink := &MemorySink{}
	svc := NewService(NewStore(), sink)
	if b, ok := svc.Backends.(*InMemoryBackends); ok {
		b.Bootstrap("tenant-a")
	}
	return svc, sink
}

func TestRegisterBYOEncryptsCredential(t *testing.T) {
	svc, sink := newTestService(t)
	r, err := svc.Register(context.Background(), RegisterRequest{
		WorkspaceID: "ws-1", TenantID: "tenant-a", Type: TypeGKE, Mode: ModeBYO,
		Name: "byo-prod", Endpoint: "https://gke.example.com", Namespace: "apps",
		Kubeconfig: "apiVersion: v1\nkind: Config\n",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if r.CredentialCipherB64 == "" || r.CredentialKMSKeyRef == "" {
		t.Fatalf("credential not encrypted: %+v", r)
	}
	if strings.Contains(r.CredentialCipherB64, "apiVersion") {
		t.Fatalf("plaintext leaked into ciphertext: %s", r.CredentialCipherB64)
	}
	evs := sink.ByType("runtime.registered.v1")
	if len(evs) != 1 {
		t.Fatalf("expected 1 runtime.registered.v1, got %d", len(evs))
	}
	if !boolField(evs[0].Data, "encrypted") {
		t.Fatalf("expected encrypted=true on register event")
	}
}

func TestRegisterBYORejectsMissingCredential(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Register(context.Background(), RegisterRequest{
		WorkspaceID: "ws-1", TenantID: "tenant-a", Type: TypeGKE, Mode: ModeBYO, Name: "x",
	})
	if !errors.Is(err, ErrCredentialRequired) {
		t.Fatalf("expected ErrCredentialRequired, got %v", err)
	}
}

func TestPreflightRejectsClusterAdmin(t *testing.T) {
	svc, sink := newTestService(t)
	r, err := svc.Register(context.Background(), RegisterRequest{
		WorkspaceID: "ws-1", TenantID: "tenant-a", Type: TypeGKE, Mode: ModeBYO,
		Name: "byo", Endpoint: "https://k8s", Namespace: "apps",
		Kubeconfig: "apiVersion: v1\nkind: Config\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.RunPreflight(context.Background(), r.ID, PreflightHints{KubeconfigSummary: "role=cluster-admin"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != PreflightFailed {
		t.Fatalf("expected failed outcome, got %s", res.Outcome)
	}
	if res.Reason != "excessive_privilege" {
		t.Fatalf("expected reason excessive_privilege, got %s", res.Reason)
	}
	evs := sink.ByType("runtime.preflight.v1")
	if len(evs) != 1 {
		t.Fatalf("expected 1 preflight event, got %d", len(evs))
	}
}

func TestPreflightRejectsRBACInsufficient(t *testing.T) {
	svc, _ := newTestService(t)
	r, _ := svc.Register(context.Background(), RegisterRequest{
		WorkspaceID: "ws-1", TenantID: "tenant-a", Type: TypeGKE, Mode: ModeBYO,
		Name: "byo", Endpoint: "https://k8s",
		Kubeconfig: "apiVersion: v1\nkind: Config\n",
	})
	res, err := svc.RunPreflight(context.Background(), r.ID, PreflightHints{KubeconfigSummary: ""})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != PreflightFailed || res.Reason != "rbac_insufficient" {
		t.Fatalf("expected rbac_insufficient, got %s/%s", res.Outcome, res.Reason)
	}
}

func TestCheckUsableByEnforcesTenancy(t *testing.T) {
	svc, sink := newTestService(t)
	r, _ := svc.Register(context.Background(), RegisterRequest{
		WorkspaceID: "ws-1", TenantID: "tenant-a", Type: TypeGKE, Mode: ModeBYO,
		Name: "byo", Endpoint: "https://k8s", Namespace: "apps",
		Kubeconfig: "apiVersion: v1\nkind: Config\n",
	})
	if err := svc.CheckUsableBy(r.ID, "ws-1"); err != nil {
		t.Fatalf("expected ws-1 allowed: %v", err)
	}
	if err := svc.CheckUsableBy(r.ID, "ws-2"); !errors.Is(err, ErrCrossWorkspace) {
		t.Fatalf("expected cross_workspace_runtime, got %v", err)
	}
	_ = sink
}

func TestRevokeBlocksUse(t *testing.T) {
	svc, _ := newTestService(t)
	r, _ := svc.Register(context.Background(), RegisterRequest{
		WorkspaceID: "ws-1", TenantID: "tenant-a", Type: TypeGKE, Mode: ModeBYO,
		Name: "byo", Endpoint: "https://k8s", Namespace: "apps",
		Kubeconfig: "apiVersion: v1\nkind: Config\n",
	})
	if _, err := svc.Revoke(r.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.CheckUsableBy(r.ID, "ws-1"); !errors.Is(err, ErrRuntimeRevoked) {
		t.Fatalf("expected runtime_revoked, got %v", err)
	}
}

func TestProvisionRequiresStateBackend(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Provision(context.Background(), ProvisionRequest{
		WorkspaceID: "ws-1", TenantID: "tenant-without-backend", Type: TypeGKE, Name: "p",
	})
	if !errors.Is(err, ErrStateBackendMissing) {
		t.Fatalf("expected state_backend_missing, got %v", err)
	}
}

func TestProvisionEmitsEventsAndRegisters(t *testing.T) {
	svc, sink := newTestService(t)
	resp, err := svc.Provision(context.Background(), ProvisionRequest{
		WorkspaceID: "ws-1", TenantID: "tenant-a", Type: TypeGKE, Name: "demo",
		GKEMode: GKEAutopilot, Env: "dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Runtime.Mode != ModeProvisioned {
		t.Fatalf("expected mode=provisioned, got %s", resp.Runtime.Mode)
	}
	if resp.Outputs["project_id"] == nil {
		t.Fatalf("expected project_id in outputs")
	}
	if len(sink.ByType("runtime.provisioned.v1")) != 1 {
		t.Fatalf("expected 1 provisioned event")
	}
}

func TestDestroyBlockedByActiveDeployments(t *testing.T) {
	svc, _ := newTestService(t)
	resp, _ := svc.Provision(context.Background(), ProvisionRequest{
		WorkspaceID: "ws-1", TenantID: "tenant-a", Type: TypeGKE, Name: "demo", Env: "dev",
	})
	svc.ActiveCheck = stubActive{has: true, ids: []string{"dep-1"}}
	if err := svc.Destroy(context.Background(), resp.Runtime.ID); !errors.Is(err, ErrDeploymentsPresent) {
		t.Fatalf("expected deployments_present, got %v", err)
	}
	svc.ActiveCheck = stubActive{}
	if err := svc.Destroy(context.Background(), resp.Runtime.ID); err != nil {
		t.Fatalf("expected destroy ok, got %v", err)
	}
}

type stubActive struct {
	has bool
	ids []string
}

func (s stubActive) HasActiveDeployments(context.Context, string) (bool, []string, error) {
	return s.has, s.ids, nil
}

func TestCapabilitiesDefaults(t *testing.T) {
	gke := DefaultCapabilities(TypeGKE)
	if !gke.SupportsCanary || !gke.SupportsBlueGreen || !gke.SupportsSecretsCSI {
		t.Fatalf("gke capabilities incomplete: %+v", gke)
	}
	cr := DefaultCapabilities(TypeCloudRun)
	if !cr.SupportsTrafficSplitting || !cr.SupportsCanary {
		t.Fatalf("cloudrun capabilities incomplete: %+v", cr)
	}
}

func boolField(m map[string]any, k string) bool {
	v, _ := m[k].(bool)
	return v
}
