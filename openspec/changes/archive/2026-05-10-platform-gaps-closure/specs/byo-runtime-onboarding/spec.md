## ADDED Requirements

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
