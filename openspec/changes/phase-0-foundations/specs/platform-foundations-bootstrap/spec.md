## ADDED Requirements

### Requirement: Tenant, Business Unit and Workspace hierarchy
The platform SHALL implement a three-level hierarchy: **Tenant → Business Unit → Workspace**. A Workspace is the primary unit of work and SHALL be able to associate (zero or many of each): repositories, environments, AI assets and audit records. Cross-Tenant data SHALL be isolated.

#### Scenario: Bootstrap a new Tenant with BU and Workspace
- **WHEN** an authorized administrator creates a Tenant with at least one Business Unit and one Workspace
- **THEN** the records are persisted in PostgreSQL, isolated from other Tenants, and exposed via the Control Plane API and the Portal

#### Scenario: Workspace cannot exist without a Business Unit
- **WHEN** a client tries to create a Workspace without a valid Business Unit reference
- **THEN** the Control Plane API returns 400 with a validation error and persists nothing

### Requirement: Workspace requires at least one owner
Each Workspace SHALL have at least one owner. Removing the last owner SHALL be rejected unless ownership is transferred in the same operation.

#### Scenario: Cannot remove the last owner
- **WHEN** an operation would leave a Workspace without owners
- **THEN** the Control Plane API rejects the operation with 409 and emits no audit event for the removal

### Requirement: Authentication via Keycloak
All Control Plane and Portal access SHALL require a valid token issued by Keycloak (federated to the corporate IdP via OIDC/SAML). Anonymous access SHALL be rejected.

#### Scenario: Unauthenticated request is rejected
- **WHEN** any API call lacks a valid Keycloak-issued token
- **THEN** the platform returns 401 and emits an audit event for the failed authentication attempt

### Requirement: Authorization via OpenFGA (ReBAC)
All resource access SHALL be authorized via **OpenFGA** with a model covering `tenant`, `business_unit`, `workspace`, `asset`, `repo`, `environment`, `deployment` and relations `parent`, `owner`, `member`, `viewer`. No identity SHALL have global cross-Workspace permissions by default.

#### Scenario: User without relation is denied
- **WHEN** an authenticated user requests a Workspace they have no OpenFGA relation to
- **THEN** the platform returns 403 and emits an audit event with the policy decision

#### Scenario: Owner of Workspace A cannot read Workspace B
- **WHEN** an owner of Workspace A requests resources of Workspace B in another BU/Tenant
- **THEN** the platform returns 403 and emits an audit event

### Requirement: Append-only audit trail with tamper-evidence
Every state-changing action on Tenants, BUs, Workspaces, assets, repos, deployments and permission grants/revocations SHALL produce an append-only audit event. The audit table SHALL reject UPDATE and DELETE at the database level. Each event SHALL include `prev_hash` chained per Tenant for tamper-evidence and SHALL be replicated to Kafka topic `audit.events.v1`.

#### Scenario: Audit event is emitted on Workspace creation
- **WHEN** a Workspace is created
- **THEN** an event is persisted with `actor`, `action`, `target`, `timestamp`, `correlation_id` and `prev_hash`, and is published to `audit.events.v1`

#### Scenario: Audit row cannot be modified
- **WHEN** any client (including DB superuser via the application path) attempts UPDATE or DELETE on `audit_event`
- **THEN** the operation is rejected by the trigger/policy and the attempt itself is audited

### Requirement: Event backbone with Kafka and CloudEvents
The platform SHALL operate **Apache Kafka** as the asynchronous event backbone from day one. All platform events SHALL conform to **CloudEvents v1.0** and SHALL include extensions `forgetenantid`, `forgeworkspaceid` (when applicable), `forgeactor` and `forgecorrelationid`. Topic names SHALL follow `<domain>.<event>.v<n>` (e.g., `workspace.created.v1`, `audit.events.v1`).

#### Scenario: Published event conforms to CloudEvents
- **WHEN** any service publishes a platform event
- **THEN** the payload validates against the CloudEvents schema and includes the required Forge extensions

### Requirement: PostgreSQL and Redis
Relational data (tenancy, audit, assets, OpenSpecs metadata) SHALL be persisted in **PostgreSQL** with backups and PITR enabled. Cache, sessions and short-lived state SHALL use **Redis**.

#### Scenario: Backup and PITR are enabled
- **WHEN** the bootstrap is complete in any environment
- **THEN** PostgreSQL has automated backups enabled and PITR can restore to any point within the configured retention window

### Requirement: Milvus available for future RAG
**Milvus** SHALL be deployed and reachable from the Agentic Plane network. Ingestion and retrieval SHALL be validated with a synthetic dataset before declaring the bootstrap complete. Production RAG ingestion is part of Fase 1.

#### Scenario: Milvus health and round-trip validated
- **WHEN** the platform smoke test runs
- **THEN** the test ingests N synthetic vectors, retrieves the expected nearest neighbors, and reports success

### Requirement: Custom Portal bootstrap
The platform SHALL provide a Custom Portal (Next.js + React + Tailwind/shadcn) that supports Keycloak login, lists the user's Workspaces (filtered by OpenFGA relations) and exposes empty placeholders for the modules introduced in later phases. The Portal SHALL NOT be based on Backstage.

#### Scenario: Authenticated user lists their Workspaces
- **WHEN** a user logs in via Keycloak and opens the Portal home
- **THEN** the Portal lists every Workspace where the user has an OpenFGA relation, scoped by Tenant

### Requirement: Base observability stack
Every Forge service SHALL emit logs, metrics and traces using **OpenTelemetry** with a collector forwarding to **Prometheus**, **Grafana**, **Loki** and **Tempo**. Each service SHALL expose `/healthz` and SLO-relevant metrics. Logs and traces SHALL include `correlation_id`.

#### Scenario: Service exposes /healthz and metrics with correlation
- **WHEN** a Forge service is running
- **THEN** `/healthz` returns 200 when ready, Prometheus metrics are scrapable, and traces include the inbound `correlation_id`

### Requirement: GitHub App for SCM integration
The platform SHALL register a Forge GitHub App with minimum required scopes. The Portal SHALL provide a "Connect GitHub" UI that completes the App installation flow and enables listing the user's repositories.

#### Scenario: User connects GitHub and lists repos
- **WHEN** a user completes the GitHub App installation flow from the Portal
- **THEN** the platform stores the installation, lists the accessible repositories, and emits an audit event

### Requirement: Retention policies for audit, telemetry and RAG data
The platform SHALL define and enforce retention policies for audit events, telemetry (logs/metrics/traces, AI traces) and RAG-ingested data, parameterized by data classification (public, internal, confidential, restricted).

#### Scenario: Retention policy is published and applied
- **WHEN** the bootstrap is complete
- **THEN** retention policies are documented in `docs/policies/retention.md` and enforced by storage TTLs / archival jobs
