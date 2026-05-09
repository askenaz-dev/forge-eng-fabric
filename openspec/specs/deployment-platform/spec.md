# deployment-platform Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Supported runtimes
The platform SHALL support exactly three runtimes for application deployment: **GKE**, **Cloud Run** and **Minikube**. Each environment registered in a Workspace SHALL declare its runtime type.

#### Scenario: Register a GKE environment
- **WHEN** a Workspace owner registers a GKE cluster as an environment
- **THEN** the platform stores the connection metadata, validates connectivity, and lists the environment as available for deployments

#### Scenario: Register a Minikube environment for local/lab use
- **WHEN** a user registers a Minikube environment
- **THEN** the platform supports it with the same lifecycle and audit controls as managed runtimes, scoped to the Workspace

### Requirement: BYO Runtime and Provisioned by Forge modes
Each environment SHALL declare its mode: **BYO Runtime** (an existing runtime is connected) or **Provisioned by Forge** (Forge provisions/configures resources via IaC). Both modes SHALL be subject to the same policy, audit and observability controls.

#### Scenario: Provision a new GKE cluster via IaC
- **WHEN** an owner requests a Provisioned-by-Forge environment on GCP
- **THEN** Alfred or the platform applies Terraform/Config Connector/Helm to provision the resources within the authorized project, and registers the environment

### Requirement: Federated cloud projects with delegated permissions
The platform SHALL be able to operate on federated cloud projects (target projects belonging to teams/initiatives) using **explicit, scoped, auditable and revocable** delegated permissions granted to Alfred.

#### Scenario: Owner grants delegated permission on a federated project
- **WHEN** a target-project owner grants Alfred a scoped role on the federated project
- **THEN** Alfred can perform only the authorized actions on that project, every action is audited, and the grant can be revoked at any time

### Requirement: IaC as the source of truth for provisioning
Resources provisioned by Forge SHALL be defined as code (Terraform, Config Connector, Helm). Manual changes SHALL be detected and reconciled against IaC.

#### Scenario: Drift detection alerts owners
- **WHEN** drift is detected between IaC and live infrastructure
- **THEN** the platform alerts the Workspace owners and offers reconciliation options

### Requirement: Deployment policies per environment
Deployment to each environment SHALL respect the configured policy: autonomous, requires approval, or restricted to specific roles. Higher-criticality environments SHALL default to requiring approval unless explicitly configured otherwise.

#### Scenario: Deploy to dev is autonomous by default
- **WHEN** Alfred triggers a deploy to a `dev` environment under default policy
- **THEN** the deploy executes without approval and is audited

#### Scenario: Deploy to prod requires approval
- **WHEN** Alfred triggers a deploy to a `prod` environment requiring approval
- **THEN** the deploy is blocked until the configured approver grants it

### Requirement: Audited deployments with rollback
Every deployment SHALL be audited with `actor`, `environment`, `version`, `image_digest`, `correlation_id`, `policy_decisions` and `outcome`, and SHALL support rollback to a previous known-good version.

#### Scenario: Rollback after failed health checks
- **WHEN** a deployment fails post-deploy health checks under an `act-and-rollback` self-healing policy
- **THEN** the platform rolls back to the previous version automatically and emits the corresponding audit and incident events

### Requirement: Per-deployment observability
Each deployment SHALL produce links to its logs, metrics, traces and health dashboard scoped to the Workspace and visible in the Portal.

#### Scenario: Portal shows deployment with linked observability
- **WHEN** a deployment completes
- **THEN** the Portal shows the deployment record with direct links to logs (Loki/Cloud Logging), metrics (Prometheus/Grafana), traces (Tempo) and health

