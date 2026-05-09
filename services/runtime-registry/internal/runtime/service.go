package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidType         = errors.New("invalid_runtime_type")
	ErrInvalidMode         = errors.New("invalid_runtime_mode")
	ErrCredentialRequired  = errors.New("credential_required")
	ErrPlaintextForbidden  = errors.New("encryption_required")
	ErrCrossWorkspace      = errors.New("cross_workspace_runtime")
	ErrRuntimeNotFound     = errors.New("runtime_not_found")
	ErrRuntimeRevoked      = errors.New("runtime_revoked")
	ErrStateBackendMissing = errors.New("state_backend_missing")
	ErrDeploymentsPresent  = errors.New("deployments_present")
)

// TerraformProvisioner orchestrates Terraform plan/apply. The default
// implementation is a fake that returns deterministic outputs so tests can
// exercise the full provisioning flow without GCP.
type TerraformProvisioner interface {
	Apply(ctx context.Context, req ProvisionRequest) (outputs map[string]any, plan []TerraformEvent, err error)
}

// ActiveDeploymentChecker is consulted before destroying a Provisioned
// runtime — Forge owns the lifecycle and MUST refuse destroy while
// deployments are running (`forge-provisioned-runtime` spec).
type ActiveDeploymentChecker interface {
	HasActiveDeployments(ctx context.Context, runtimeID string) (bool, []string, error)
}

type NoActiveDeployments struct{}

func (NoActiveDeployments) HasActiveDeployments(context.Context, string) (bool, []string, error) {
	return false, nil, nil
}

// StateBackends tracks per-Tenant Terraform state buckets. A Tenant must
// bootstrap a backend before requesting `Provision`.
type StateBackends interface {
	HasBackend(tenantID string) bool
}

type InMemoryBackends struct {
	tenants map[string]bool
}

func NewInMemoryBackends(tenants ...string) *InMemoryBackends {
	m := map[string]bool{}
	for _, t := range tenants {
		m[t] = true
	}
	return &InMemoryBackends{tenants: m}
}

func (b *InMemoryBackends) HasBackend(tenantID string) bool {
	return b.tenants[tenantID]
}

func (b *InMemoryBackends) Bootstrap(tenantID string) {
	b.tenants[tenantID] = true
}

type Service struct {
	Store         *Store
	KMS           KMS
	Sink          Sink
	Preflight     PreflightChecker
	Verifier      Verifier
	Provisioner   TerraformProvisioner
	ActiveCheck   ActiveDeploymentChecker
	Backends      StateBackends
	OverrideAdmin bool
}

func NewService(store *Store, sink Sink) *Service {
	return &Service{
		Store:       store,
		KMS:         NewFakeKMS(),
		Sink:        sink,
		Preflight:   NewStaticPreflightChecker(),
		Verifier:    NewStaticVerifier(),
		Provisioner: NewFakeTerraformProvisioner(),
		ActiveCheck: NoActiveDeployments{},
		Backends:    NewInMemoryBackends(),
	}
}

// RunVerify executes the verifier on a runtime, persists the report as
// evidence on the runtime record, and emits an audit event. Spec D7 plus
// task 2.13 require timestamp + caller principal in the persisted record.
func (s *Service) RunVerify(ctx context.Context, runtimeID, principal string, hints VerifyHints) (*VerifyReport, error) {
	r, ok := s.Store.Get(runtimeID)
	if !ok {
		return nil, ErrRuntimeNotFound
	}
	report := s.Verifier.Verify(ctx, r, hints)
	report.RuntimeID = r.ID
	report.WorkspaceID = r.WorkspaceID
	report.Type = r.Type
	report.Mode = r.Mode
	report.Principal = principal
	if report.ID == "" {
		report.ID = newID()
	}
	stored := report
	s.Store.AppendVerify(&stored)
	_ = s.Sink.Emit(newEvent(r, "runtime.verified.v1", map[string]any{
		"runtime_id": r.ID,
		"verify_id":  report.ID,
		"status":     string(report.Status),
		"principal":  principal,
		"checks":     report.Checks,
	}))
	return &stored, nil
}

// LatestVerify returns the most recent verification report for a runtime, or
// nil if none has been recorded.
func (s *Service) LatestVerify(runtimeID string) *VerifyReport {
	reports := s.Store.Verifications(runtimeID)
	if len(reports) == 0 {
		return nil
	}
	return reports[len(reports)-1]
}

// Register implements `byo-runtime-onboarding` register flow.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*Runtime, error) {
	if !validType(req.Type) {
		return nil, ErrInvalidType
	}
	if !validMode(req.Mode) {
		return nil, ErrInvalidMode
	}
	if req.WorkspaceID == "" || req.TenantID == "" {
		return nil, errors.New("missing_workspace_or_tenant")
	}
	if req.Mode == ModeBYO {
		// `mode=byo` requires either kubeconfig (k8s runtimes) or SA key
		// (Cloud Run); never both, never plaintext stored on the runtime.
		credential := req.Kubeconfig
		if credential == "" {
			credential = req.SAKey
		}
		if credential == "" {
			return nil, ErrCredentialRequired
		}
		cipher, keyRef, err := s.KMS.Encrypt(req.TenantID, credential)
		if err != nil {
			return nil, fmt.Errorf("kms encrypt: %w", err)
		}
		req.Kubeconfig = ""
		req.SAKey = ""
		visibility := req.Visibility
		if visibility == "" {
			visibility = VisibilityWorkspace
		}
		r := &Runtime{
			WorkspaceID:         req.WorkspaceID,
			TenantID:            req.TenantID,
			Type:                req.Type,
			Mode:                req.Mode,
			Visibility:          visibility,
			Name:                req.Name,
			Region:              req.Region,
			GKEMode:             req.GKEMode,
			ProjectID:           req.ProjectID,
			ClusterName:         req.ClusterName,
			Endpoint:            req.Endpoint,
			ServiceAccountEmail: req.ServiceAccountEmail,
			Namespace:           req.Namespace,
			CredentialKMSKeyRef: keyRef,
			CredentialCipherB64: cipher,
			Labels:              req.Labels,
			Capabilities:        DefaultCapabilities(req.Type),
			Status:              "registered",
		}
		if err := s.Store.Insert(r); err != nil {
			return nil, err
		}
		_ = s.Sink.Emit(newEvent(r, "runtime.registered.v1", map[string]any{
			"runtime_id":    r.ID,
			"workspace_id":  r.WorkspaceID,
			"tenant_id":     r.TenantID,
			"type":          string(r.Type),
			"mode":          string(r.Mode),
			"encrypted":     true,
			"kms_key_ref":   r.CredentialKMSKeyRef,
		}))
		return r, nil
	}
	// `mode=provisioned` is registered post-provision via Provision().
	visibility := req.Visibility
	if visibility == "" {
		visibility = VisibilityWorkspace
	}
	r := &Runtime{
		WorkspaceID:         req.WorkspaceID,
		TenantID:            req.TenantID,
		Type:                req.Type,
		Mode:                req.Mode,
		Visibility:          visibility,
		Name:                req.Name,
		Region:              req.Region,
		GKEMode:             req.GKEMode,
		ProjectID:           req.ProjectID,
		ClusterName:         req.ClusterName,
		Endpoint:            req.Endpoint,
		ServiceAccountEmail: req.ServiceAccountEmail,
		Namespace:           req.Namespace,
		Labels:              req.Labels,
		Capabilities:        DefaultCapabilities(req.Type),
		Status:              "registered",
	}
	if err := s.Store.Insert(r); err != nil {
		return nil, err
	}
	_ = s.Sink.Emit(newEvent(r, "runtime.registered.v1", map[string]any{
		"runtime_id":   r.ID,
		"workspace_id": r.WorkspaceID,
		"tenant_id":    r.TenantID,
		"type":         string(r.Type),
		"mode":         string(r.Mode),
		"encrypted":    false,
	}))
	return r, nil
}

func (s *Service) Get(id string) (*Runtime, error) {
	r, ok := s.Store.Get(id)
	if !ok {
		return nil, ErrRuntimeNotFound
	}
	return r, nil
}

func (s *Service) List(workspaceID string) []*Runtime {
	return s.Store.List(workspaceID)
}

// CheckUsableBy enforces the tenancy boundary required by
// `byo-runtime-onboarding`.
func (s *Service) CheckUsableBy(runtimeID, workspaceID string) error {
	r, ok := s.Store.Get(runtimeID)
	if !ok {
		return ErrRuntimeNotFound
	}
	if r.Revoked {
		return ErrRuntimeRevoked
	}
	if r.WorkspaceID == workspaceID {
		return nil
	}
	if r.Visibility == VisibilityTenant {
		return nil
	}
	return ErrCrossWorkspace
}

// RunPreflight executes preflight on a runtime and persists the result.
func (s *Service) RunPreflight(ctx context.Context, runtimeID string, hints PreflightHints) (*PreflightResult, error) {
	r, ok := s.Store.Get(runtimeID)
	if !ok {
		return nil, ErrRuntimeNotFound
	}
	res := s.Preflight.Check(ctx, r, hints)
	res.RuntimeID = r.ID
	resCopy := res
	s.Store.AppendPreflight(&resCopy)
	if res.Outcome == PreflightSuccess {
		r.Status = "ready"
	} else {
		r.Status = "preflight_failed"
	}
	s.Store.Update(r)
	_ = s.Sink.Emit(newEvent(r, "runtime.preflight.v1", map[string]any{
		"runtime_id": r.ID,
		"outcome":    string(res.Outcome),
		"reason":     res.Reason,
		"checks":     res.Checks,
	}))
	return &resCopy, nil
}

// Revoke marks the runtime as revoked; subsequent Apply calls must fail.
func (s *Service) Revoke(runtimeID string) (*Runtime, error) {
	r, ok := s.Store.Get(runtimeID)
	if !ok {
		return nil, ErrRuntimeNotFound
	}
	r.Revoked = true
	r.Status = "revoked"
	s.Store.Update(r)
	_ = s.Sink.Emit(newEvent(r, "runtime.revoked.v1", map[string]any{
		"runtime_id": r.ID,
	}))
	return r, nil
}

// Provision drives Terraform apply for a Provisioned runtime.
func (s *Service) Provision(ctx context.Context, req ProvisionRequest) (*ProvisionResponse, error) {
	if !validType(req.Type) {
		return nil, ErrInvalidType
	}
	if req.WorkspaceID == "" || req.TenantID == "" {
		return nil, errors.New("missing_workspace_or_tenant")
	}
	if !s.Backends.HasBackend(req.TenantID) {
		return nil, ErrStateBackendMissing
	}
	outputs, plan, err := s.Provisioner.Apply(ctx, req)
	if err != nil {
		return nil, err
	}
	r := &Runtime{
		WorkspaceID:  req.WorkspaceID,
		TenantID:     req.TenantID,
		Type:         req.Type,
		Mode:         ModeProvisioned,
		Visibility:   VisibilityWorkspace,
		Name:         req.Name,
		Region:       req.Region,
		GKEMode:      req.GKEMode,
		Capabilities: DefaultCapabilities(req.Type),
		Status:       "ready",
		Labels:       map[string]any{"env": req.Env, "provisioned_by": "forge"},
	}
	if v, ok := outputs["project_id"].(string); ok {
		r.ProjectID = v
	}
	if v, ok := outputs["cluster_name"].(string); ok {
		r.ClusterName = v
	}
	if v, ok := outputs["endpoint"].(string); ok {
		r.Endpoint = v
	}
	if v, ok := outputs["sa_email"].(string); ok {
		r.ServiceAccountEmail = v
	}
	if err := s.Store.Insert(r); err != nil {
		return nil, err
	}
	_ = s.Sink.Emit(newEvent(r, "runtime.registered.v1", map[string]any{
		"runtime_id":   r.ID,
		"workspace_id": r.WorkspaceID,
		"tenant_id":    r.TenantID,
		"type":         string(r.Type),
		"mode":         string(r.Mode),
	}))
	_ = s.Sink.Emit(newEvent(r, "runtime.provisioned.v1", map[string]any{
		"runtime_id": r.ID,
		"outputs":    outputs,
	}))
	for _, ev := range plan {
		_ = s.Sink.Emit(newEvent(r, "iac.terraform.plan.v1", map[string]any{
			"runtime_id": r.ID,
			"action":     ev.Action,
			"resource":   ev.Resource,
			"outcome":    ev.Outcome,
			"outputs":    ev.Outputs,
		}))
	}
	return &ProvisionResponse{Runtime: r, Outputs: outputs, Plan: plan}, nil
}

// Destroy enforces lifecycle ownership: blocked while deployments exist.
func (s *Service) Destroy(ctx context.Context, runtimeID string) error {
	r, ok := s.Store.Get(runtimeID)
	if !ok {
		return ErrRuntimeNotFound
	}
	if r.Mode != ModeProvisioned {
		return errors.New("not_provisioned_runtime")
	}
	hasActive, _, err := s.ActiveCheck.HasActiveDeployments(ctx, runtimeID)
	if err != nil {
		return err
	}
	if hasActive {
		return ErrDeploymentsPresent
	}
	r.Status = "destroyed"
	s.Store.Update(r)
	_ = s.Sink.Emit(newEvent(r, "runtime.destroyed.v1", map[string]any{"runtime_id": r.ID}))
	return nil
}

func validType(t Type) bool {
	switch t {
	case TypeGKE, TypeCloudRun, TypeMinikube:
		return true
	}
	return false
}

func validMode(m Mode) bool {
	switch m {
	case ModeBYO, ModeProvisioned:
		return true
	}
	return false
}

// FakeTerraformProvisioner returns deterministic outputs without invoking
// terraform. Used in tests and local dev.
type FakeTerraformProvisioner struct{}

func NewFakeTerraformProvisioner() *FakeTerraformProvisioner { return &FakeTerraformProvisioner{} }

func (FakeTerraformProvisioner) Apply(_ context.Context, req ProvisionRequest) (map[string]any, []TerraformEvent, error) {
	now := time.Now().UTC().Unix()
	projectID := fmt.Sprintf("forge-%s-%s", req.WorkspaceID, req.Env)
	cluster := fmt.Sprintf("forge-%s-%s", req.Name, req.Env)
	outputs := map[string]any{
		"project_id":   projectID,
		"cluster_name": cluster,
		"endpoint":     fmt.Sprintf("https://%s.example.com", cluster),
		"sa_email":     fmt.Sprintf("forge-deployer@%s.iam.gserviceaccount.com", projectID),
		"applied_at":   now,
	}
	plan := []TerraformEvent{
		{Type: "plan", Resource: "module.gcp_project", Action: "create", Outcome: "ok"},
		{Type: "plan", Resource: "module.gke_autopilot", Action: "create", Outcome: "ok"},
		{Type: "plan", Resource: "module.artifact_registry_binding", Action: "create", Outcome: "ok"},
		{Type: "plan", Resource: "module.workload_identity", Action: "create", Outcome: "ok"},
		{Type: "apply", Action: "applied", Outcome: "ok", Outputs: outputs},
	}
	return outputs, plan, nil
}
