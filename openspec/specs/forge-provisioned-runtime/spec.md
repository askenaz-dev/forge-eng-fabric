# forge-provisioned-runtime Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Provisioned mode with Terraform

When a Workspace requests Provisioned by Forge, infra SHALL be created via Terraform modules from `forge-iac-modules/` with state stored in a per-Tenant GCS backend with locking, encryption and versioning.

#### Scenario: Provision GKE Autopilot for a Workspace

- **GIVEN** a Workspace requesting `runtime.mode=provisioned, type=gke, gke_mode=autopilot`
- **WHEN** `POST /v1/runtimes/provision` is called
- **THEN** Terraform MUST plan and apply the modules `gcp-project`, `gke-autopilot`, `artifact-registry-binding`, `workload-identity`
- **AND** persist outputs (`project_id`, `cluster_name`, `endpoint`, `sa_email`)
- **AND** register the runtime automatically with `mode=provisioned`
- **AND** emit `runtime.provisioned.v1`

### Requirement: Per-Tenant state backend

Each Tenant MUST have a dedicated GCS bucket for Terraform state with locking, encryption (CMEK), and versioning enabled.

#### Scenario: Reject provision without Tenant state backend

- **GIVEN** a Tenant lacking a configured state backend
- **WHEN** provision is attempted
- **THEN** the orchestrator MUST refuse with `412 state_backend_missing`
- **AND** instruct the operator to bootstrap the backend

### Requirement: Lifecycle ownership

For Provisioned runtimes, Forge owns the lifecycle (create / update / destroy). Destroy MUST require explicit approval and audit, and MUST be blocked while active deployments exist.

#### Scenario: Block destroy while deployments exist

- **GIVEN** a Provisioned runtime with active `deployment.status=running`
- **WHEN** destroy is requested
- **THEN** the operation MUST fail with `409 deployments_present`
- **AND** list the blocking deployments

### Requirement: Drift between Terraform and reality reported

Drift detection MUST run hourly against Provisioned runtimes; findings emit `iac.drift.detected.v1` with severity and proposed remediation.

#### Scenario: Manual change detected as drift

- **GIVEN** a Provisioned GKE cluster whose node pool size was edited via `gcloud`
- **WHEN** the drift job runs
- **THEN** a finding MUST be created with `resource=node_pool`, `severity=medium`
- **AND** Alfred MUST receive `iac.drift.detected.v1` and propose a remediation PR

### Requirement: Runtime onboarding verification API and tooling

The `runtime-registry` service SHALL expose `POST /v1/runtimes/{id}/verify` returning a structured report of preflight and post-provision checks for Provisioned-by-Forge runtimes. The Makefile target `verify-runtime` SHALL invoke this API and render a human-readable summary.

#### Scenario: Verification API returns structured report

- **WHEN** an authorized caller invokes `POST /v1/runtimes/{id}/verify`
- **THEN** the response SHALL be a structured report including `workspace_id`, `runtime_id`, `type`, `mode=provisioned`, and a list of checks with `name`, `status` (`pass|fail|warn|skipped`), `evidence`, and `remediation`

#### Scenario: Verifier checks federated IAM scopes

- **WHEN** the verifier runs against a Provisioned GKE runtime
- **THEN** the report SHALL include a check confirming that the federated project's delegated service account has the minimum required roles and SHALL flag any over-privileged grants

#### Scenario: Verifier checks Artifact Registry image pull

- **WHEN** the verifier runs against a Provisioned runtime
- **THEN** the report SHALL include a check that pulls a known canary image from the Workspace's Artifact Registry, confirming network and IAM viability

#### Scenario: Make target wraps the API

- **WHEN** an operator runs `make verify-runtime WORKSPACE=<id> RUNTIME=<runtime_id>`
- **THEN** the target SHALL call the verification API for the specified runtime, render check results to the terminal, and exit non-zero if any check has status `fail`

### Requirement: Federated project IAM minimization documented

For Provisioned-by-Forge runtimes on federated GCP projects, the Terraform modules SHALL grant the platform's delegated service account only the roles required to operate the runtime, and the granted roles SHALL be enumerated in `docs/platform-enablement.md` Phase 3 section.

#### Scenario: Module grants minimum roles

- **WHEN** the `iam-delegated-permissions` module is applied to a federated project
- **THEN** only the documented minimum roles SHALL be granted to the platform's service account, and any addition SHALL require a documented change

#### Scenario: Verifier flags over-privilege

- **WHEN** the verifier inspects a federated project and finds roles beyond the documented minimum
- **THEN** the verifier SHALL include a `warn` check naming the extra roles
