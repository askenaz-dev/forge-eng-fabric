# workspace-management Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Workspace lifecycle
The platform SHALL support the full lifecycle of a Workspace: **create, configure, update, archive, delete**. Each Workspace SHALL have at least one owner, belong to exactly one Business Unit and expose a unique identifier within its Tenant.

#### Scenario: Create a Workspace with an owner
- **WHEN** an authorized user creates a Workspace specifying name, BU and at least one owner
- **THEN** the Workspace is persisted, the owner gets the `owner` OpenFGA relation, and a creation audit event is emitted

#### Scenario: Archive a Workspace preserves audit and OpenSpec links
- **WHEN** an owner archives a Workspace
- **THEN** the Workspace becomes read-only, no new actions are accepted, and OpenSpecs/audit history remain accessible

### Requirement: Workspace contents
A Workspace SHALL be able to associate (zero or many of each): repositories, OpenSpecs, environments, workflows, AI assets (used or published), pipelines, deployments, policies, approval rules and metrics dashboards.

#### Scenario: Associate a GitHub repository to a Workspace
- **WHEN** an owner connects a GitHub repository to the Workspace
- **THEN** the repository is listed under the Workspace, audit is emitted and the repo inherits Workspace policies

### Requirement: Configurable autonomy and approval policies
Each Workspace SHALL allow configuring autonomy and approval policies by **action type, environment, criticality, role and responsible person**. Defaults SHALL follow "Autonomy by Default, Policy by Exception": Alfred operates autonomously unless a policy explicitly requires HITL.

#### Scenario: Autonomous deploy to dev allowed
- **WHEN** the Workspace policy marks `deploy:dev` as autonomous and Alfred triggers a deploy to dev
- **THEN** the deploy executes without human approval and the action is fully audited

#### Scenario: HITL required for production deploy
- **WHEN** the Workspace policy marks `deploy:prod` as `requires_approval` and Alfred attempts a prod deploy
- **THEN** Alfred creates an approval request, blocks the action, and proceeds only after approval by the configured role

### Requirement: Workspace owners and roles
Each Workspace SHALL support multiple owners (technical, functional, operational) and role-based memberships. Removing the last owner SHALL be rejected unless transferred.

#### Scenario: Cannot remove the last owner
- **WHEN** a user attempts to remove the only remaining owner of a Workspace
- **THEN** the operation is rejected and a clear error is returned

### Requirement: Workspace metrics
Each Workspace SHALL expose adoption, velocity (intent → PR/deploy), reuse, quality, security, cost and reliability metrics scoped to its boundary.

#### Scenario: Workspace dashboard shows reuse rate
- **WHEN** an owner opens the Workspace dashboard
- **THEN** the dashboard displays the count and percentage of assets reused from the Registry within that Workspace

### Requirement: Tenant-wide asset visibility with Workspace ownership
Assets published from a Workspace SHALL be visible inside the same Tenant according to their visibility setting (private to Workspace or shared with Tenant) and SHALL preserve their owning Workspace.

#### Scenario: Shared asset is discoverable across the Tenant
- **WHEN** a Workspace publishes an asset with Tenant visibility
- **THEN** other Workspaces of the same Tenant can discover and request it in the Registry, while ownership remains with the publishing Workspace

