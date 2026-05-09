# iac-drift-detection Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Periodic drift detection

A scheduler MUST run drift detection at least hourly against every Provisioned IaC workspace using `terraform plan -detailed-exitcode`; non-zero diff results MUST create findings.

#### Scenario: Drift created when manual change exists

- **GIVEN** a GKE cluster whose node pool was changed via `gcloud`
- **WHEN** the hourly job runs `terraform plan`
- **THEN** the exit code MUST be `2`
- **AND** an `iac_drift_finding` record MUST be created with `resource`, `field`, `expected`, `actual`, `severity`
- **AND** emit `iac.drift.detected.v1`

### Requirement: Severity classification

Findings MUST be classified `low | medium | high | critical` based on resource sensitivity (network/IAM = high+, tags/labels = low) and runtime impact.

#### Scenario: IAM change classified high

- **GIVEN** a manual IAM binding addition outside Terraform
- **WHEN** drift detection runs
- **THEN** the finding MUST be classified `high` or `critical`
- **AND** alert routing MUST notify Workspace owners and Security

### Requirement: Suppressions

Findings MAY be suppressed via `.forge-drift-ignore.yaml` versioned in the IaC repo; suppressions MUST be explicit (resource pattern + field pattern + reason + expiration).

#### Scenario: Reject suppression without expiration

- **GIVEN** a suppression entry without `expires_at`
- **WHEN** the drift job parses the file
- **THEN** the entry MUST be rejected with `suppression_missing_expiration`
- **AND** emit `iac.drift.suppression.invalid.v1`

### Requirement: Remediation proposal by Alfred

On `iac.drift.detected.v1`, Alfred MUST propose a remediation PR containing either reverting `terraform apply` or updating IaC to match reality, with rationale and risks documented.

#### Scenario: Alfred opens remediation PR

- **GIVEN** a `medium` drift finding on a GKE node pool
- **WHEN** Alfred receives the event
- **THEN** Alfred MUST open a PR in the IaC repo with the proposed change
- **AND** link the PR to the finding via `iac_drift_finding.remediation_pr_url`
- **AND** emit `iac.drift.remediation.proposed.v1`
