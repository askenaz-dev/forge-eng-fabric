# runtime-connectors Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Common Deployer interface

All runtime connectors SHALL implement the `Deployer` interface with operations: `Preflight`, `Render`, `Apply`, `Verify`, `Rollback`. Each connector MUST declare its supported capabilities (canary, blue/green, secrets-csi).

#### Scenario: Connector advertises capabilities

- **GIVEN** the GKE connector is registered
- **WHEN** the orchestrator queries `connector.Capabilities()`
- **THEN** it MUST return at least `{supports_canary: true, supports_blue_green: true, supports_secrets_csi: true}`

#### Scenario: Cloud Run connector advertises traffic splitting

- **GIVEN** the Cloud Run connector
- **WHEN** capabilities are queried
- **THEN** it MUST return `{supports_traffic_splitting: true, supports_canary: true (via revisions), supports_blue_green: true}`

### Requirement: Initial connectors

Phase 3 MUST ship connectors for: GKE (Standard and Autopilot), Cloud Run (managed), Minikube/kind (dev local).

#### Scenario: Deploy to GKE Autopilot via connector

- **GIVEN** a runtime registered as `type=gke, mode=provisioned, gke_mode=autopilot`
- **WHEN** the orchestrator invokes `Apply`
- **THEN** the GKE connector MUST render Helm chart, apply via `helm upgrade --install`, and wait for rollout completion
- **AND** emit `deployment.applied.v1`

#### Scenario: Deploy to Cloud Run with traffic split

- **GIVEN** a runtime registered as `type=cloudrun`
- **WHEN** the orchestrator invokes `Apply` with `strategy=canary{traffic=10%}`
- **THEN** the Cloud Run connector MUST deploy a new revision
- **AND** route 10% traffic to the new revision while retaining 90% on the previous

### Requirement: Preflight checks

Before accepting a runtime, the connector MUST perform preflight checks: connectivity, RBAC minimums, namespace creation rights, registry pull access, Workload Identity binding (if applicable).

#### Scenario: Preflight rejects insufficient RBAC

- **GIVEN** a BYO kubeconfig whose SA lacks `create namespace` permission
- **WHEN** preflight runs
- **THEN** the result MUST be `failed` with reason `rbac_insufficient`
- **AND** emit `runtime.preflight.v1` with details
