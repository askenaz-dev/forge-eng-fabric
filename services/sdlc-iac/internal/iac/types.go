// Package iac implements the SDLC Infrastructure-as-Code generation, validation,
// and apply skills. It generates Terraform modules and Helm values files for
// platform-managed applications, validates them, and opens PRs for review.
package iac

import "fmt"

// Provider identifies a cloud provider target for Terraform generation.
type Provider = string

const (
	ProviderAWS   Provider = "aws"
	ProviderGCP   Provider = "gcp"
	ProviderAzure Provider = "azure"
)

// AllProviders lists all supported cloud providers.
var AllProviders = []string{ProviderAWS, ProviderGCP, ProviderAzure}

// CriticalityTier maps application criticality to resource sizing.
type CriticalityTier = string

const (
	CriticalitySmall  CriticalityTier = "small"
	CriticalityMedium CriticalityTier = "medium"
	CriticalityLarge  CriticalityTier = "large"
)

// GenerateTerraformInput is the request to generate a Terraform module skeleton.
type GenerateTerraformInput struct {
	AppID         string   `json:"app_id"`
	Slug          string   `json:"slug"`
	Provider      string   `json:"provider"`
	CorrelationID string   `json:"correlation_id"`
	ArchDecision  string   `json:"arch_decision"`
	Environments  []string `json:"environments"`
}

// GenerateTerraformOutput describes the artifacts produced by Terraform generation.
type GenerateTerraformOutput struct {
	ModulePath   string   `json:"module_path"`
	ProviderList []string `json:"provider_list"`
	EmittedEvent string   `json:"emitted_event"`
}

// GenerateHelmInput is the request to generate Helm values files.
type GenerateHelmInput struct {
	AppID         string `json:"app_id"`
	Slug          string `json:"slug"`
	Criticality   string `json:"criticality"`
	CorrelationID string `json:"correlation_id"`
}

// GenerateHelmOutput describes the Helm values artifacts produced.
type GenerateHelmOutput struct {
	// ValuesFiles contains one path per environment (local, staging, prod).
	ValuesFiles []string `json:"values_files"`
}

// ValidateIaCInput is the request to validate generated IaC artifacts.
type ValidateIaCInput struct {
	AppID               string `json:"app_id"`
	TerraformModulePath string `json:"terraform_module_path"`
	HelmValuesPath      string `json:"helm_values_path"`
	CorrelationID       string `json:"correlation_id"`
}

// ValidateIaCOutput is the result of an IaC validation run.
type ValidateIaCOutput struct {
	Status string            `json:"status"` // "passed" or "failed"
	Report IaCValidationReport `json:"report"`
}

// IaCValidationReport captures the per-tool outcomes of a validation run.
type IaCValidationReport struct {
	TerraformFmtPassed      bool     `json:"terraform_fmt_passed"`
	TerraformPlanPassed     bool     `json:"terraform_plan_passed"`
	HelmLintPassed          bool     `json:"helm_lint_passed"`
	HelmTemplatePassed      bool     `json:"helm_template_passed"`
	ConftestPassed          bool     `json:"conftest_passed"`
	ConftestViolations      []string `json:"conftest_violations"`
}

// BreakGlassApproval records a dual-approval for the break_glass flow.
// Both security-admin and platform-admin approvals are required.
type BreakGlassApproval struct {
	ApproverRole string `json:"approver_role"` // "security-admin" or "platform-admin"
	ApprovedBy   string `json:"approved_by"`
	Reason       string `json:"reason"`
}

// ErrBreakGlassDualApprovalRequired is returned when break_glass=true but the
// required dual-approval (security-admin + platform-admin) is not present.
var ErrBreakGlassDualApprovalRequired = fmt.Errorf("break_glass requires approval from both security-admin and platform-admin")

// ApplyIaCInput is the request to apply IaC changes via a PR.
type ApplyIaCInput struct {
	AppID               string               `json:"app_id"`
	Slug                string               `json:"slug"`
	TerraformModulePath string               `json:"terraform_module_path"`
	HelmValuesPath      string               `json:"helm_values_path"`
	BreakGlass          bool                 `json:"break_glass"`
	BreakGlassApprovals []BreakGlassApproval `json:"break_glass_approvals,omitempty"`
	CorrelationID       string               `json:"correlation_id"`
	ValidationReport    IaCValidationReport  `json:"validation_report"`
}

// ApplyIaCOutput is the result of an apply request.
type ApplyIaCOutput struct {
	PRUrl   string `json:"pr_url"`
	PRTitle string `json:"pr_title"`
}

// PRMergedEvent is the payload received from the GitOps runner when an IaC PR is merged.
// The service uses this to execute the actual infrastructure apply (terraform apply / helm upgrade).
type PRMergedEvent struct {
	AppID         string `json:"app_id"`
	PRURL         string `json:"pr_url"`
	Slug          string `json:"slug"`
	Provider      string `json:"provider"`
	Environment   string `json:"environment"` // "staging" | "prod"
	CorrelationID string `json:"correlation_id"`
}

// GitOpsApplyResult is emitted after the actual apply completes.
type GitOpsApplyResult struct {
	AppID       string `json:"app_id"`
	Environment string `json:"environment"`
	Status      string `json:"status"` // "applied" | "failed"
	Event       string `json:"event"`
}
