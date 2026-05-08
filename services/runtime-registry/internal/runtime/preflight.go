package runtime

import (
	"context"
	"strings"
	"time"
)

// PreflightChecker performs runtime validation. The default implementation
// (`StaticPreflightChecker`) inspects the kubeconfig metadata that callers
// pass via `RegisterRequest`; production wires real probes (kubectl auth
// can-i, registry pull, Workload Identity binding lookups).
type PreflightChecker interface {
	Check(ctx context.Context, r *Runtime, hints PreflightHints) PreflightResult
}

type PreflightHints struct {
	// KubeconfigSummary is a free-form caller hint describing the kubeconfig
	// (e.g. "namespace=apps,sa=forge-deployer,role=namespace-admin"). Used
	// by `StaticPreflightChecker` to detect cluster-admin or missing scopes.
	KubeconfigSummary string
	// RegistryPullToken is the registry secret namespace/binding name the
	// caller asserts is present. If empty, preflight reports the check as
	// passing in `mode=byo` (operator responsibility) but failing in
	// `mode=provisioned`.
	RegistryPullToken string
	// HasWorkloadIdentity asserts a workload-identity binding exists.
	HasWorkloadIdentity bool
}

type StaticPreflightChecker struct{}

func NewStaticPreflightChecker() *StaticPreflightChecker { return &StaticPreflightChecker{} }

func (StaticPreflightChecker) Check(_ context.Context, r *Runtime, hints PreflightHints) PreflightResult {
	start := time.Now().UTC()
	checks := []PreflightCheck{}

	// Connectivity: assume reachable if endpoint is set OR runtime is
	// minikube (local dev runs against a kubeconfig context).
	connected := r.Type == TypeMinikube || r.Endpoint != ""
	checks = append(checks, PreflightCheck{Name: "connectivity", Passed: connected, Reason: ifElse(connected, "", "endpoint_unreachable")})

	// RBAC: reject cluster-admin scopes; require namespace-scoped role.
	rbacReason := ""
	rbacPassed := true
	summary := strings.ToLower(hints.KubeconfigSummary)
	if strings.Contains(summary, "cluster-admin") {
		rbacPassed = false
		rbacReason = "excessive_privilege"
	} else if r.Type != TypeCloudRun && r.Mode == ModeBYO && r.Namespace == "" && !strings.Contains(summary, "namespace=") {
		rbacPassed = false
		rbacReason = "rbac_insufficient"
	}
	checks = append(checks, PreflightCheck{Name: "rbac", Passed: rbacPassed, Reason: rbacReason})

	// Namespace creation rights — required only when `mode=provisioned`.
	nsPassed := r.Mode == ModeBYO || strings.Contains(summary, "create-namespace")
	checks = append(checks, PreflightCheck{Name: "namespace_create", Passed: nsPassed, Reason: ifElse(nsPassed, "", "missing_namespace_create_rights")})

	// Registry pull access — required for `provisioned`; advisory for BYO.
	registryPassed := r.Mode == ModeBYO || hints.RegistryPullToken != ""
	checks = append(checks, PreflightCheck{Name: "registry_pull", Passed: registryPassed, Reason: ifElse(registryPassed, "", "registry_pull_missing")})

	// Workload Identity — required on GKE provisioned; advisory elsewhere.
	wiRequired := r.Type == TypeGKE && r.Mode == ModeProvisioned
	wiPassed := !wiRequired || hints.HasWorkloadIdentity
	checks = append(checks, PreflightCheck{Name: "workload_identity", Passed: wiPassed, Reason: ifElse(wiPassed, "", "workload_identity_missing")})

	outcome := PreflightSuccess
	reason := ""
	for _, c := range checks {
		if !c.Passed {
			outcome = PreflightFailed
			reason = c.Reason
			break
		}
	}

	return PreflightResult{
		RuntimeID: r.ID,
		Outcome:   outcome,
		Reason:    reason,
		Checks:    checks,
		StartedAt: start,
		EndedAt:   time.Now().UTC(),
	}
}

func ifElse(cond bool, ifTrue, ifFalse string) string {
	if cond {
		return ifTrue
	}
	return ifFalse
}
