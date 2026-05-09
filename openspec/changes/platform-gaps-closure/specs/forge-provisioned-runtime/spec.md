## ADDED Requirements

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
