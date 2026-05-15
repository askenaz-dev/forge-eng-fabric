# app-onboarding-service Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Onboarding request lifecycle

The `app-onboarding-service` SHALL accept onboarding requests **scoped to an App** and orchestrate the end-to-end flow: policy evaluation → scaffolding → repo creation via GitHub MCP → branch protections → CI pipelines publication → asset registration. Every onboarding request SHALL carry either `app_id` (target an existing App) or `app_proposal` (create a new App atomically with the repository).

#### Scenario: Successful onboarding from approved template against existing App

- **GIVEN** an authenticated user with `workspace:onboard-app` permission and `app#editor` on `app-1`
- **AND** a template `go-microservice@1.0.0` in `lifecycle_state=approved` and `trust_level=T3`
- **WHEN** the user submits `POST /v1/onboarding` with valid parameters and `app_id=app-1`
- **THEN** the service MUST evaluate onboarding policies and approvals
- **AND** invoke the scaffolder to render the template with parameters
- **AND** invoke the GitHub MCP to create the repo with CODEOWNERS and PR templates
- **AND** apply branch protections according to criticality
- **AND** publish baseline CI pipelines
- **AND** register the new asset of `type=application` in the Registry with `lifecycle_state=proposed` and `app_id=app-1`
- **AND** link the new repo to `app-1.repo_links[]`
- **AND** emit `app.onboarding.requested.v1` and `app.onboarding.completed.v1` with `app_id=app-1`
- **AND** record full audit entries

#### Scenario: Onboarding with inline App creation

- **GIVEN** an authenticated user with `workspace:create-app` permission
- **WHEN** the user submits `POST /v1/onboarding` with `app_proposal={name, slug, description, owners}` and no `app_id`
- **THEN** the service MUST first create the App via the App CRUD API as a single audited operation
- **AND** then proceed with onboarding using the newly created `app_id`
- **AND** the audit record MUST link the App creation and the onboarding under the same `correlation_id`

#### Scenario: Reject onboarding with non-approved template

- **GIVEN** a template with `lifecycle_state=in_review` or `trust_level<T3`
- **WHEN** an onboarding request references that template
- **THEN** the service MUST reject the request with `403 template_not_approved`
- **AND** emit `app.onboarding.rejected.v1` with reason

#### Scenario: Reject onboarding without policy approval

- **GIVEN** a Workspace policy that requires approval for repo creation
- **WHEN** the request lacks an approved `approval_request`
- **THEN** the service MUST mark the request as `pending_approval`
- **AND** create an entry in the Approvals Inbox
- **AND** SHALL NOT proceed until approval is granted

#### Scenario: Reject onboarding without App scope

- **WHEN** an onboarding request omits both `app_id` and `app_proposal`
- **THEN** the service MUST reject the request with `422 missing_app_scope`

### Requirement: Live status and streaming

The service SHALL expose `GET /v1/onboarding/{id}` with current state and a Server-Sent Events stream for live updates including each stage transition.

#### Scenario: Live state for in-progress onboarding

- **GIVEN** an onboarding request `id=req-123` in progress
- **WHEN** a client opens the SSE stream
- **THEN** the service MUST emit events `stage.started`, `stage.completed`, `stage.failed` for each stage
- **AND** include duration and structured payload for each event

### Requirement: Idempotency

Onboarding requests SHALL be idempotent based on `(workspace_id, app_id, repo_name)`; a repeated request MUST return the existing request id and current state without duplicating side effects.

#### Scenario: Duplicate request returns existing onboarding

- **GIVEN** an existing onboarding for `(workspace=ws-1, app=app-1, repo_name=svc-foo)` in state `completed`
- **WHEN** a duplicate request arrives
- **THEN** the service MUST return `200` with the original `request_id` and `status=completed`
- **AND** SHALL NOT create a second repo or duplicate audit entries

### Requirement: Audit and events

Every onboarding action SHALL emit immutable audit entries and CloudEvents `app.onboarding.*`, `repo.created.v1`, `branch_protection.applied.v1`, with `correlation_id` linking all stages.

#### Scenario: Full audit chain

- **GIVEN** a successful onboarding
- **WHEN** the auditor queries by `correlation_id`
- **THEN** the auditor MUST see entries for: request received, policy evaluated, scaffold rendered, repo created, branch protection applied, pipelines published, asset registered, completion

### Requirement: Optional runtime defaults at onboarding

The onboarding service SHALL support optional runtime defaults and, when requested, MUST provision a default runtime (typically `dev`) by invoking `POST /v1/runtimes/provision` (Provisioned mode) or accept a pre-existing BYO runtime id.

#### Scenario: Onboarding with provisioned dev runtime

- **GIVEN** a Workspace requesting onboarding with `provision_dev_runtime=true`
- **WHEN** onboarding completes the repo creation phase
- **THEN** the service MUST trigger runtime provisioning for `env=dev`
- **AND** wait for `runtime.provisioned.v1`
- **AND** emit `app.onboarding.runtime_provisioned.v1`

#### Scenario: Onboarding with BYO runtime reference

- **GIVEN** a Workspace passing `byo_runtime_id=rt-7`
- **WHEN** onboarding processes runtime defaults
- **THEN** the service MUST validate that `rt-7` belongs to the same Workspace
- **AND** record the linkage on the application asset
- **AND** SHALL NOT provision new infrastructure

### Requirement: Reject onboarding referencing inaccessible runtime

If a referenced runtime is from another Workspace or is revoked, onboarding MUST fail at the runtime-defaults stage.

#### Scenario: Cross-Workspace runtime reference rejected

- **GIVEN** an onboarding in `ws-1` referencing a runtime in `ws-2`
- **WHEN** runtime defaults are evaluated
- **THEN** onboarding MUST fail with `403 cross_workspace_runtime`
- **AND** the repo creation MUST NOT be reverted (already committed) but the application asset MUST stay in `lifecycle_state=proposed` with annotation
