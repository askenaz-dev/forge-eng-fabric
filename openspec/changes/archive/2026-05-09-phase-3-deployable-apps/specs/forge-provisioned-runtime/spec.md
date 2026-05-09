# Spec Delta: forge-provisioned-runtime (ADDED)

## ADDED Requirements

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
