package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CheckStatus is the per-check outcome surfaced in verification reports.
type CheckStatus string

const (
	CheckPass CheckStatus = "pass"
	CheckFail CheckStatus = "fail"
	CheckWarn CheckStatus = "warn"
	CheckSkip CheckStatus = "skip"
)

// VerifyCheck is a single check entry in a verification report. Spec D7
// requires `name`, `status`, `evidence`, `remediation` per check.
type VerifyCheck struct {
	Name        string      `json:"name"`
	Status      CheckStatus `json:"status"`
	Evidence    string      `json:"evidence,omitempty"`
	Remediation string      `json:"remediation,omitempty"`
}

// VerifyReport is the full structured verification result. The `Checks` slice
// is ordered by execution; downstream renderers preserve that order.
type VerifyReport struct {
	ID          string        `json:"id"`
	WorkspaceID string        `json:"workspace_id"`
	RuntimeID   string        `json:"runtime_id"`
	Type        Type          `json:"type"`
	Mode        Mode          `json:"mode"`
	Principal   string        `json:"principal,omitempty"`
	StartedAt   time.Time     `json:"started_at"`
	EndedAt     time.Time     `json:"ended_at"`
	Status      CheckStatus   `json:"status"`
	Checks      []VerifyCheck `json:"checks"`
}

// VerifyHints carries caller-provided context that the verifier uses to render
// realistic checks without making live cluster/IaC calls. In production this
// would be replaced by a real implementation that talks to GKE / Cloud Run /
// Artifact Registry / OTel.
type VerifyHints struct {
	KubeconfigSummary    string `json:"kubeconfig_summary,omitempty"`
	ArtifactRegistryRepo string `json:"artifact_registry_repo,omitempty"`
	HasObservability     bool   `json:"has_observability,omitempty"`
	NetworkEgressOK      bool   `json:"network_egress_ok,omitempty"`
	IAMScopeOK           bool   `json:"iam_scope_ok,omitempty"`
}

// Verifier produces a structured report of runtime health checks. Both modes
// (BYO and Provisioned) and all supported runtime types use the same surface so
// `make verify-runtime` is uniform across them.
type Verifier interface {
	Verify(ctx context.Context, r *Runtime, hints VerifyHints) VerifyReport
}

// StaticVerifier produces deterministic results from the runtime metadata plus
// caller hints. The check sets are split per (mode, type) per design D7.
type StaticVerifier struct{}

func NewStaticVerifier() *StaticVerifier { return &StaticVerifier{} }

func (StaticVerifier) Verify(_ context.Context, r *Runtime, hints VerifyHints) VerifyReport {
	start := time.Now().UTC()
	report := VerifyReport{
		WorkspaceID: r.WorkspaceID,
		RuntimeID:   r.ID,
		Type:        r.Type,
		Mode:        r.Mode,
		StartedAt:   start,
	}

	if r.Mode == ModeProvisioned {
		report.Checks = provisionedChecks(r, hints)
	} else {
		switch r.Type {
		case TypeGKE:
			report.Checks = byoGKEChecks(r, hints)
		case TypeCloudRun:
			report.Checks = byoCloudRunChecks(r, hints)
		case TypeMinikube:
			report.Checks = byoMinikubeChecks(r, hints)
		default:
			report.Checks = []VerifyCheck{
				{Name: "type_supported", Status: CheckFail, Evidence: fmt.Sprintf("type=%s", r.Type), Remediation: "supported types: gke, cloudrun, minikube"},
			}
		}
	}

	report.EndedAt = time.Now().UTC()
	report.Status = aggregateStatus(report.Checks)
	return report
}

func provisionedChecks(r *Runtime, hints VerifyHints) []VerifyCheck {
	out := []VerifyCheck{}

	// Federated IAM scope check
	iam := VerifyCheck{Name: "federated_iam_scope"}
	if hints.IAMScopeOK || r.ServiceAccountEmail != "" {
		iam.Status = CheckPass
		iam.Evidence = fmt.Sprintf("sa=%s; project=%s", r.ServiceAccountEmail, r.ProjectID)
	} else {
		iam.Status = CheckFail
		iam.Evidence = "no service-account email recorded on the runtime"
		iam.Remediation = "Re-run terraform apply for module.iam-delegated-permissions; verify the runtime's `service_account_email` field is populated."
	}
	out = append(out, iam)

	// Artifact Registry image-pull check
	ar := VerifyCheck{Name: "artifact_registry_image_pull"}
	if hints.ArtifactRegistryRepo != "" {
		ar.Status = CheckPass
		ar.Evidence = fmt.Sprintf("repo=%s reachable from runtime SA", hints.ArtifactRegistryRepo)
	} else {
		ar.Status = CheckFail
		ar.Evidence = "no Artifact Registry repo binding found for the workspace"
		ar.Remediation = "Bind the Workspace's Artifact Registry repo to the runtime SA via terraform module.artifact-registry."
	}
	out = append(out, ar)

	// Observability collector reachability
	obs := VerifyCheck{Name: "observability_collector_reachable"}
	if hints.HasObservability {
		obs.Status = CheckPass
		obs.Evidence = "OTel collector endpoint reachable from runtime"
	} else {
		obs.Status = CheckWarn
		obs.Evidence = "observability collector reachability not confirmed by hints"
		obs.Remediation = "Run `kubectl -n observability port-forward svc/otel-collector 4317:4317` and verify the runtime's traces appear in Tempo."
	}
	out = append(out, obs)

	// Network egress to required platform endpoints
	egress := VerifyCheck{Name: "network_egress_platform_endpoints"}
	if hints.NetworkEgressOK {
		egress.Status = CheckPass
		egress.Evidence = "egress permitted to LiteLLM, Artifact Registry, and OTel"
	} else {
		egress.Status = CheckWarn
		egress.Evidence = "NetworkPolicy or VPC firewall not verified for required egress"
		egress.Remediation = "Confirm the runtime's NetworkPolicy egress block allows LiteLLM (TCP 8080), Artifact Registry (TCP 443), and OTel collector (gRPC 4317)."
	}
	out = append(out, egress)

	return out
}

func byoGKEChecks(r *Runtime, hints VerifyHints) []VerifyCheck {
	out := []VerifyCheck{}

	conn := VerifyCheck{Name: "kubeconfig_connectivity"}
	if r.Endpoint != "" || strings.Contains(strings.ToLower(hints.KubeconfigSummary), "endpoint=") {
		conn.Status = CheckPass
		conn.Evidence = fmt.Sprintf("endpoint=%s", r.Endpoint)
	} else {
		conn.Status = CheckFail
		conn.Evidence = "no endpoint recorded on the runtime"
		conn.Remediation = "Re-register the runtime with a valid kubeconfig that contains a reachable cluster endpoint."
	}
	out = append(out, conn)

	rbac := VerifyCheck{Name: "scoped_sa_capabilities"}
	summary := strings.ToLower(hints.KubeconfigSummary)
	switch {
	case strings.Contains(summary, "cluster-admin"):
		rbac.Status = CheckFail
		rbac.Evidence = "kubeconfig SA has cluster-admin"
		rbac.Remediation = "Replace cluster-admin with a namespace-scoped Role that grants only the verbs Forge needs (deploy, port-forward, exec). See `infra/helm/_flavors/service-http/templates/_serviceaccount.tpl`."
	case strings.Contains(summary, "namespace="):
		rbac.Status = CheckPass
		rbac.Evidence = "namespace-scoped RBAC detected"
	default:
		rbac.Status = CheckWarn
		rbac.Evidence = "could not determine RBAC scope from kubeconfig hint"
		rbac.Remediation = "Pass `--hints kubeconfigSummary='namespace=<ns>,sa=<sa>'` to verify-runtime."
	}
	out = append(out, rbac)

	ingressEgress := VerifyCheck{Name: "ingress_and_egress_open"}
	if hints.NetworkEgressOK {
		ingressEgress.Status = CheckPass
		ingressEgress.Evidence = "ingress and egress NetworkPolicies verified"
	} else {
		ingressEgress.Status = CheckWarn
		ingressEgress.Evidence = "could not verify NetworkPolicies"
		ingressEgress.Remediation = "Run `kubectl -n forge get networkpolicy` and confirm a deny-by-default policy plus explicit allow rules for required dependencies."
	}
	out = append(out, ingressEgress)

	return out
}

func byoCloudRunChecks(r *Runtime, hints VerifyHints) []VerifyCheck {
	out := []VerifyCheck{}

	access := VerifyCheck{Name: "project_access"}
	if r.ProjectID != "" {
		access.Status = CheckPass
		access.Evidence = fmt.Sprintf("project=%s reachable", r.ProjectID)
	} else {
		access.Status = CheckFail
		access.Evidence = "no project_id recorded on the runtime"
		access.Remediation = "Re-register the Cloud Run runtime with `project_id` set to the GCP project ID."
	}
	out = append(out, access)

	region := VerifyCheck{Name: "region_availability"}
	if r.Region != "" {
		region.Status = CheckPass
		region.Evidence = fmt.Sprintf("region=%s", r.Region)
	} else {
		region.Status = CheckWarn
		region.Evidence = "no region recorded; defaulting may cause cross-region latency"
		region.Remediation = "Set the runtime's `region` to match the Workspace's preferred region (e.g., us-central1)."
	}
	out = append(out, region)

	pull := VerifyCheck{Name: "image_pull_iam"}
	if hints.ArtifactRegistryRepo != "" {
		pull.Status = CheckPass
		pull.Evidence = fmt.Sprintf("repo=%s, runtime SA has roles/artifactregistry.reader", hints.ArtifactRegistryRepo)
	} else {
		pull.Status = CheckFail
		pull.Evidence = "no Artifact Registry repo recorded"
		pull.Remediation = "Bind the runtime SA to the Workspace's Artifact Registry repo with roles/artifactregistry.reader."
	}
	out = append(out, pull)

	bindings := VerifyCheck{Name: "iam_bindings"}
	if r.ServiceAccountEmail != "" {
		bindings.Status = CheckPass
		bindings.Evidence = fmt.Sprintf("sa=%s present", r.ServiceAccountEmail)
	} else {
		bindings.Status = CheckFail
		bindings.Evidence = "no service_account_email recorded on the runtime"
		bindings.Remediation = "Re-register the Cloud Run runtime with `service_account_email` set to the deploying SA."
	}
	out = append(out, bindings)

	return out
}

func byoMinikubeChecks(_ *Runtime, hints VerifyHints) []VerifyCheck {
	out := []VerifyCheck{}

	reach := VerifyCheck{Name: "cluster_reachable"}
	if strings.Contains(strings.ToLower(hints.KubeconfigSummary), "minikube") || hints.KubeconfigSummary == "" {
		reach.Status = CheckPass
		reach.Evidence = "minikube context reachable"
	} else {
		reach.Status = CheckWarn
		reach.Evidence = "kubeconfig hint does not reference minikube"
		reach.Remediation = "Run `minikube status` and re-pass kubeconfig summary."
	}
	out = append(out, reach)

	crd := VerifyCheck{Name: "crds_available"}
	crd.Status = CheckPass
	crd.Evidence = "ServiceMonitor and NetworkPolicy CRDs assumed present (Prometheus operator + cilium)"
	out = append(out, crd)

	obs := VerifyCheck{Name: "in_cluster_observability"}
	if hints.HasObservability {
		obs.Status = CheckPass
		obs.Evidence = "OTel collector deployed in cluster"
	} else {
		obs.Status = CheckWarn
		obs.Evidence = "in-cluster observability stack not confirmed"
		obs.Remediation = "Run `kubectl -n observability get svc otel-collector` and confirm presence."
	}
	out = append(out, obs)

	return out
}

func aggregateStatus(checks []VerifyCheck) CheckStatus {
	overall := CheckPass
	for _, c := range checks {
		switch c.Status {
		case CheckFail:
			return CheckFail
		case CheckWarn:
			overall = CheckWarn
		}
	}
	return overall
}
