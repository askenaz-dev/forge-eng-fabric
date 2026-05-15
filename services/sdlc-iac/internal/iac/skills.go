package iac

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// IaCSkills provides the IaC generation, validation, and apply skills.
type IaCSkills struct {
	sink Sink
}

// NewIaCSkills creates an IaCSkills instance backed by the given event sink.
func NewIaCSkills(sink Sink) *IaCSkills {
	if sink == nil {
		sink = &MemorySink{}
	}
	return &IaCSkills{sink: sink}
}

// GenerateTerraform generates a skeleton Terraform module for the given application.
// It validates that the requested provider is supported, creates environment overlays
// for local/staging/prod, and emits sdlc.iac.generated.v1.
func (s *IaCSkills) GenerateTerraform(_ context.Context, in GenerateTerraformInput) (GenerateTerraformOutput, error) {
	if !validProvider(in.Provider) {
		return GenerateTerraformOutput{}, fmt.Errorf("unsupported provider %q: must be one of %v", in.Provider, AllProviders)
	}

	envs := in.Environments
	if len(envs) == 0 {
		envs = []string{"local", "staging", "prod"}
	}

	modulePath := fmt.Sprintf("infra/%s/terraform", in.Slug)

	// Skeleton files that would be written: main.tf, variables.tf, outputs.tf, versions.tf
	// plus per-environment overlay directories under envs/<env>/.
	// In this stub implementation we record what would be created.
	_ = []string{
		modulePath + "/main.tf",
		modulePath + "/variables.tf",
		modulePath + "/outputs.tf",
		modulePath + "/versions.tf",
	}
	for _, env := range envs {
		_ = modulePath + "/envs/" + env + "/main.tf"
	}

	providerList := []string{in.Provider}

	const eventType = "sdlc.iac.generated.v1"
	_ = s.sink.Emit(newEvent(eventType, "app/"+in.AppID, map[string]any{
		"app_id":         in.AppID,
		"slug":           in.Slug,
		"provider":       in.Provider,
		"module_path":    modulePath,
		"environments":   envs,
		"correlation_id": in.CorrelationID,
		"arch_decision":  in.ArchDecision,
	}))

	return GenerateTerraformOutput{
		ModulePath:   modulePath,
		ProviderList: providerList,
		EmittedEvent: eventType,
	}, nil
}

// GenerateHelmValues generates Helm values files for local, staging, and prod
// environments. Replica counts and resource requests are sized according to the
// criticality tier. Emits sdlc.iac.helm_values.generated.v1.
func (s *IaCSkills) GenerateHelmValues(_ context.Context, in GenerateHelmInput) (GenerateHelmOutput, error) {
	type tierSpec struct {
		replicas int
		cpuReq   string
	}
	specs := map[string]tierSpec{
		CriticalitySmall:  {replicas: 1, cpuReq: "100m"},
		CriticalityMedium: {replicas: 3, cpuReq: "500m"},
		CriticalityLarge:  {replicas: 5, cpuReq: "1000m"},
	}
	spec, ok := specs[in.Criticality]
	if !ok {
		// Default to small if criticality is unrecognised.
		spec = specs[CriticalitySmall]
	}

	envs := []string{"local", "staging", "prod"}
	basePath := fmt.Sprintf("infra/%s/helm", in.Slug)
	var valuesFiles []string
	for _, env := range envs {
		path := fmt.Sprintf("%s/values-%s.yaml", basePath, env)
		valuesFiles = append(valuesFiles, path)
		// Stub: content would be:
		//   replicaCount: <spec.replicas>
		//   resources:
		//     requests:
		//       cpu: <spec.cpuReq>
		_ = fmt.Sprintf("replicaCount: %d\nresources:\n  requests:\n    cpu: %s\n", spec.replicas, spec.cpuReq)
	}

	const eventType = "sdlc.iac.helm_values.generated.v1"
	_ = s.sink.Emit(newEvent(eventType, "app/"+in.AppID, map[string]any{
		"app_id":         in.AppID,
		"slug":           in.Slug,
		"criticality":    in.Criticality,
		"values_files":   valuesFiles,
		"correlation_id": in.CorrelationID,
	}))

	return GenerateHelmOutput{ValuesFiles: valuesFiles}, nil
}

// ValidateIaC simulates running terraform fmt, terraform plan, helm lint,
// helm template, and conftest against the generated artifacts. The stubs return
// pass for all checks; callers may inject error triggers by setting specific
// sentinel values in TerraformModulePath or HelmValuesPath (see tests).
// Emits sdlc.iac.validated.v1.
func (s *IaCSkills) ValidateIaC(_ context.Context, in ValidateIaCInput) (ValidateIaCOutput, error) {
	// Stub validation: always passes unless sentinel strings are present.
	report := IaCValidationReport{
		TerraformFmtPassed:  true,
		TerraformPlanPassed: true,
		HelmLintPassed:      true,
		HelmTemplatePassed:  true,
		ConftestPassed:      true,
		ConftestViolations:  []string{},
	}

	// Sentinel-based error triggers for testing.
	if in.TerraformModulePath == "FAIL_TF_FMT" {
		report.TerraformFmtPassed = false
	}
	if in.TerraformModulePath == "FAIL_TF_PLAN" {
		report.TerraformPlanPassed = false
	}
	if in.HelmValuesPath == "FAIL_HELM_LINT" {
		report.HelmLintPassed = false
	}
	if in.HelmValuesPath == "FAIL_CONFTEST" {
		report.ConftestPassed = false
		report.ConftestViolations = []string{"policy/no-privileged-containers: violation"}
	}

	allPassed := report.TerraformFmtPassed &&
		report.TerraformPlanPassed &&
		report.HelmLintPassed &&
		report.HelmTemplatePassed &&
		report.ConftestPassed

	status := "passed"
	if !allPassed {
		status = "failed"
	}

	const eventType = "sdlc.iac.validated.v1"
	_ = s.sink.Emit(newEvent(eventType, "app/"+in.AppID, map[string]any{
		"app_id":         in.AppID,
		"status":         status,
		"correlation_id": in.CorrelationID,
	}))

	return ValidateIaCOutput{Status: status, Report: report}, nil
}

// ApplyIaC opens a pull request containing the IaC changes. It NEVER runs
// terraform apply directly — all infrastructure changes flow through PR review.
// Break-glass flows require dual approval (security-admin + platform-admin) and
// emit sdlc.iac.break_glass_applied.v1 instead of sdlc.iac.applied.v1.
func (s *IaCSkills) ApplyIaC(_ context.Context, in ApplyIaCInput) (ApplyIaCOutput, error) {
	if in.BreakGlass {
		if err := validateBreakGlassApprovals(in.BreakGlassApprovals); err != nil {
			return ApplyIaCOutput{}, err
		}
	}

	// Stub: generate a PR URL.
	prID := uuid.NewString()[:8]
	prURL := fmt.Sprintf("https://github.com/forge-eng-fabric/infra/pull/%s", prID)
	prTitle := fmt.Sprintf("iac: provision %s (%s)", in.Slug, in.AppID)
	if in.BreakGlass {
		prTitle = "[BREAK-GLASS] " + prTitle
	}

	eventType := "sdlc.iac.applied.v1"
	if in.BreakGlass {
		eventType = "sdlc.iac.break_glass_applied.v1"
	}

	_ = s.sink.Emit(newEvent(eventType, "app/"+in.AppID, map[string]any{
		"app_id":         in.AppID,
		"slug":           in.Slug,
		"pr_url":         prURL,
		"pr_title":       prTitle,
		"break_glass":    in.BreakGlass,
		"correlation_id": in.CorrelationID,
	}))

	return ApplyIaCOutput{PRUrl: prURL, PRTitle: prTitle}, nil
}

// GitOpsApply handles the actual infrastructure apply triggered when an IaC PR is merged.
// This is the GitOps runner integration: the PR merge webhook calls this endpoint, which
// executes terraform apply / helm upgrade in the appropriate environment.
func (s *IaCSkills) GitOpsApply(_ context.Context, in PRMergedEvent) (GitOpsApplyResult, error) {
	if in.AppID == "" || in.Slug == "" || in.Environment == "" {
		return GitOpsApplyResult{}, fmt.Errorf("app_id, slug, and environment are required")
	}

	// Stub: in production this would run terraform apply or helm upgrade
	// against the environment-specific backend. Here we record the intent.
	eventType := "sdlc.iac.gitops_applied.v1"

	_ = s.sink.Emit(newEvent(eventType, "app/"+in.AppID, map[string]any{
		"app_id":         in.AppID,
		"slug":           in.Slug,
		"pr_url":         in.PRURL,
		"environment":    in.Environment,
		"provider":       in.Provider,
		"correlation_id": in.CorrelationID,
	}))

	return GitOpsApplyResult{
		AppID:       in.AppID,
		Environment: in.Environment,
		Status:      "applied",
		Event:       eventType,
	}, nil
}

// validateBreakGlassApprovals ensures both security-admin and platform-admin have approved.
func validateBreakGlassApprovals(approvals []BreakGlassApproval) error {
	hasSecAdmin, hasPlatAdmin := false, false
	for _, a := range approvals {
		switch a.ApproverRole {
		case "security-admin":
			if a.ApprovedBy != "" && a.Reason != "" {
				hasSecAdmin = true
			}
		case "platform-admin":
			if a.ApprovedBy != "" && a.Reason != "" {
				hasPlatAdmin = true
			}
		}
	}
	if !hasSecAdmin || !hasPlatAdmin {
		return ErrBreakGlassDualApprovalRequired
	}
	return nil
}

// validProvider reports whether p is in AllProviders.
func validProvider(p string) bool {
	for _, allowed := range AllProviders {
		if p == allowed {
			return true
		}
	}
	return false
}
