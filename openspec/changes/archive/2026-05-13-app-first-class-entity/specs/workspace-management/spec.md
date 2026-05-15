## MODIFIED Requirements

### Requirement: Workspace contents

A Workspace SHALL be able to associate (zero or many of each): **Apps**, repositories, OpenSpecs (through their parent App), environments, workflows, AI assets (used or published), pipelines, deployments, policies, approval rules and metrics dashboards. Apps SHALL be a top-level association under the Workspace and SHALL be the canonical anchor for OpenSpecs.

#### Scenario: Associate a GitHub repository to a Workspace

- **WHEN** an owner connects a GitHub repository to the Workspace
- **THEN** the repository is listed under the Workspace, audit is emitted and the repo inherits Workspace policies

#### Scenario: Workspace lists Apps

- **WHEN** a workspace member calls `GET /v1/workspaces/{id}/apps`
- **THEN** the response MUST list every App in `lifecycle_state in {active, archived}` visible to the caller
- **AND** the `_unassigned` App MUST be included with a flag `system_managed=true`

#### Scenario: OpenSpecs surface through their App

- **WHEN** a workspace member opens the Workspace detail view in the Portal
- **THEN** the Portal MUST render specs grouped under their parent App, with the `_unassigned` group rendered last and visually distinct

## ADDED Requirements

### Requirement: Workspace bootstrap provisions the unassigned App

When a new Workspace is created, the workspace-bootstrap pipeline SHALL provision the `_unassigned` App as part of the initial workspace activation, before the Workspace becomes available to its members.

#### Scenario: New workspace ships with unassigned App

- **WHEN** an authorized user creates a new Workspace
- **THEN** within the same atomic transaction (or compensating step on failure), the `_unassigned` App MUST be created
- **AND** the Workspace creation event MUST NOT fire until the `_unassigned` App is available
