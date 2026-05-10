# byo-runtime-onboarding Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Register BYO runtime with encrypted credentials

A Workspace MAY register a Bring-Your-Own runtime providing kubeconfig or SA key; credentials MUST be encrypted at rest using KMS scoped per Tenant and never logged in plain text.

#### Scenario: Register BYO GKE cluster

- **GIVEN** an authenticated Workspace owner
- **WHEN** they call `POST /v1/runtimes` with `{type: gke, mode: byo, kubeconfig: <encrypted>}`
- **THEN** the runtime registry MUST encrypt the kubeconfig with KMS
- **AND** persist only the ciphertext + KMS key reference
- **AND** emit `runtime.registered.v1` with `mode=byo, encrypted=true`

#### Scenario: Reject plaintext credential storage

- **GIVEN** a misconfigured registry attempting to persist plaintext
- **WHEN** the persistence layer is invoked
- **THEN** the operation MUST refuse with `500 encryption_required`
- **AND** emit `security.violation.v1`

### Requirement: Scoped service account

The BYO runtime credential MUST be a service account scoped to a single namespace (or project for Cloud Run) with minimum required RBAC; preflight rejects credentials with cluster-admin scope.

#### Scenario: Reject cluster-admin credentials

- **GIVEN** a kubeconfig granting `cluster-admin`
- **WHEN** preflight runs
- **THEN** the result MUST be `failed` with reason `excessive_privilege`

### Requirement: Tenancy boundary

A BYO runtime registered in Workspace `ws-1` MUST NOT be usable for deploys originating from Workspace `ws-2` unless explicitly shared at Tenant level.

#### Scenario: Cross-Workspace deploy denied

- **GIVEN** a runtime owned by `ws-1`
- **WHEN** an actor in `ws-2` attempts deployment to it
- **THEN** the orchestrator MUST refuse with `403 cross_workspace_denied`
- **AND** emit `guardrail.trip.v1`

### Requirement: Credential rotation and revocation

Workspace owners SHALL rotate or revoke BYO credentials at any time; revocation immediately disables further deploys to that runtime.

#### Scenario: Revoke credential blocks subsequent deploys

- **GIVEN** runtime `rt-1` is in use
- **WHEN** the owner calls `POST /v1/runtimes/rt-1/revoke`
- **THEN** subsequent `Apply` MUST fail with `403 runtime_revoked`
- **AND** in-flight deploys MUST complete or fail with `revoked_during_apply`

### Requirement: Verification API and tooling for BYO runtimes

The `runtime-registry` service SHALL expose the same `POST /v1/runtimes/{id}/verify` surface for BYO runtimes as for Provisioned runtimes, applying check sets appropriate to BYO mode (connectivity, credential validity, scoped service-account verification, image-pull, observability collector reachability). The Makefile target `verify-runtime` SHALL operate identically against BYO and Provisioned runtimes.

#### Scenario: Verifier runs against a BYO GKE runtime

- **WHEN** an operator runs `make verify-runtime WORKSPACE=<id> RUNTIME=<byo_gke_id>`
- **THEN** the target SHALL invoke the verification API and SHALL include checks for: kubeconfig connectivity, scoped service-account capabilities, ingress/egress to the platform's required endpoints, and observability collector reachability

#### Scenario: Verifier runs against a BYO Cloud Run runtime

- **WHEN** an operator runs `make verify-runtime WORKSPACE=<id> RUNTIME=<byo_cloudrun_id>`
- **THEN** the verifier SHALL run the Cloud Run-specific check set (project access, region availability, image-pull, IAM bindings)

#### Scenario: Verifier runs against a Minikube runtime

- **WHEN** an operator runs `make verify-runtime WORKSPACE=<id> RUNTIME=<minikube_id>`
- **THEN** the verifier SHALL run the local-runtime check set (cluster reachable, CRDs available, in-cluster observability stub reachable)

### Requirement: Verifier evidence persisted with runtime record

Each verification run SHALL persist its report as evidence on the runtime record with timestamp and caller principal, and the most recent successful run SHALL be visible from the Portal's runtime detail page.

#### Scenario: Latest verification visible in Portal

- **WHEN** an operator opens a runtime in the Portal
- **THEN** the page SHALL display the timestamp and summary status of the most recent verification, with a link to the full report

#### Scenario: Failed verification surfaces remediation

- **WHEN** any verification check fails
- **THEN** the report SHALL include actionable remediation guidance for that check, and the failure SHALL be visible in the Portal until a subsequent successful verification supersedes it
