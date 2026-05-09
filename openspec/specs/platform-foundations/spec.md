# platform-foundations Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Tenant, Business Unit and Workspace hierarchy
The platform SHALL implement a three-level organizational hierarchy: **Tenant → Business Unit → Workspace**. A Tenant represents the entire organization. A Business Unit groups Workspaces inside a Tenant. A Workspace is the primary unit of work and SHALL be able to contain repositories, OpenSpecs, environments, workflows, AI assets, pipelines, deployments, owners, policies, approvals and observability artifacts.

#### Scenario: Bootstrap a new Tenant with Business Units and Workspaces
- **WHEN** an authorized administrator provisions a new Tenant with at least one Business Unit and one Workspace
- **THEN** the platform persists the Tenant/BU/Workspace records, isolates their data from other Tenants, and exposes them through the Portal and APIs

#### Scenario: Workspace cannot exist outside a Business Unit
- **WHEN** a client tries to create a Workspace without referencing a valid Business Unit inside an existing Tenant
- **THEN** the platform rejects the request with a validation error and does not persist the Workspace

### Requirement: Identity, authentication and authorization
The platform SHALL use **Keycloak** for authentication (OIDC/SAML, federated with the corporate IdP) and **OpenFGA** for authorization (ReBAC/Zanzibar-style). All API calls SHALL be authenticated; all resource access SHALL be authorized via OpenFGA tuples. No identity SHALL have global cross-Workspace permissions by default.

#### Scenario: Authenticated user accesses a Workspace they own
- **WHEN** a user with a valid Keycloak token requests resources of a Workspace where OpenFGA grants them an owner relation
- **THEN** the platform serves the resources

#### Scenario: User without OpenFGA relation is rejected
- **WHEN** a user with a valid Keycloak token requests resources of a Workspace where they have no OpenFGA relation
- **THEN** the platform returns 403 Forbidden and emits an audit event

### Requirement: Immutable audit trail for platform actions
The platform SHALL emit an audit event for every state-changing action (create/update/delete on Tenants, BUs, Workspaces, assets, OpenSpecs, repos, deployments, approvals, permission grants/revocations, agent tool calls). Audit events SHALL be append-only, persisted, queryable by Workspace and timestamp, and tamper-evident.

#### Scenario: Audit event emitted on Workspace creation
- **WHEN** a Workspace is created
- **THEN** an audit event is published to Kafka with actor, action, target, timestamp and correlation ID, and is persisted in the audit store

#### Scenario: Audit trail cannot be modified
- **WHEN** any actor attempts to delete or update a persisted audit event
- **THEN** the platform rejects the operation regardless of role

### Requirement: Event backbone with CloudEvents
The platform SHALL use **Apache Kafka** as the asynchronous event backbone from day one. All platform events SHALL conform to the **CloudEvents** specification and SHALL include `tenant_id`, `workspace_id` (when applicable), `actor`, `correlation_id` and `event_type`.

#### Scenario: Platform event conforms to CloudEvents
- **WHEN** any plane publishes an event
- **THEN** the payload validates against the CloudEvents schema and includes the required Forge attributes

### Requirement: Persistence and caching
The platform SHALL use **PostgreSQL** (Cloud SQL) for relational data and **Redis** (Memorystore) for caching/session/short-lived state. Vector data for Alfred's knowledge base SHALL use **Milvus**.

#### Scenario: Relational reads/writes go to PostgreSQL
- **WHEN** the platform stores Workspace, asset, OpenSpec or audit metadata
- **THEN** the data is persisted in PostgreSQL with backups and point-in-time recovery enabled

### Requirement: Base observability stack
The platform SHALL emit metrics, logs and traces using **OpenTelemetry**, with **Prometheus**, **Grafana**, **Loki** and **Tempo** as the base stack, and **Cloud Logging** for managed runtimes. Every service SHALL expose health endpoints and SLO-relevant metrics.

#### Scenario: Service exposes /healthz and metrics
- **WHEN** a Forge service is deployed
- **THEN** it exposes `/healthz` and Prometheus metrics, and emits OpenTelemetry traces correlated by `correlation_id`

### Requirement: Custom Agentic SDLC Portal bootstrap
The platform SHALL provide a Custom Portal (Next.js + React + Tailwind/shadcn) that exposes Workspaces, Alfred Console, Asset Registry, OpenSpecs, Repositories, Environments, Deployments, Workflows, Approvals Inbox, Observability and Admin & Governance modules. The Portal SHALL NOT be based on Backstage.

#### Scenario: Portal lists user's Workspaces after login
- **WHEN** an authenticated user opens the Portal
- **THEN** the Portal lists every Workspace where the user has an OpenFGA relation, scoped by Tenant

