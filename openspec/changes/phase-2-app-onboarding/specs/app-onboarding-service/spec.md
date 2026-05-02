# Spec Delta: app-onboarding-service (ADDED)

## ADDED Requirements

### Requirement: Onboarding request lifecycle

The `app-onboarding-service` SHALL accept onboarding requests and orchestrate the end-to-end flow: policy evaluation → scaffolding → repo creation via GitHub MCP → branch protections → CI pipelines publication → asset registration.

#### Scenario: Successful onboarding from approved template

- **GIVEN** an authenticated user with `workspace:onboard-app` permission
- **AND** a template `go-microservice@1.0.0` in `lifecycle_state=approved` and `trust_level=T3`
- **WHEN** the user submits `POST /v1/onboarding` with valid parameters
- **THEN** the service MUST evaluate onboarding policies and approvals
- **AND** invoke the scaffolder to render the template with parameters
- **AND** invoke the GitHub MCP to create the repo with CODEOWNERS and PR templates
- **AND** apply branch protections according to criticality
- **AND** publish baseline CI pipelines
- **AND** register the new asset of `type=application` in the Registry with `lifecycle_state=proposed`
- **AND** emit `app.onboarding.requested.v1` and `app.onboarding.completed.v1`
- **AND** record full audit entries

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

### Requirement: Live status and streaming

The service SHALL expose `GET /v1/onboarding/{id}` with current state and a Server-Sent Events stream for live updates including each stage transition.

#### Scenario: Live state for in-progress onboarding

- **GIVEN** an onboarding request `id=req-123` in progress
- **WHEN** a client opens the SSE stream
- **THEN** the service MUST emit events `stage.started`, `stage.completed`, `stage.failed` for each stage
- **AND** include duration and structured payload for each event

### Requirement: Idempotency

Onboarding requests SHALL be idempotent based on `(workspace_id, repo_name)`; a repeated request MUST return the existing request id and current state without duplicating side effects.

#### Scenario: Duplicate request returns existing onboarding

- **GIVEN** an existing onboarding for `(workspace=ws-1, repo_name=svc-foo)` in state `completed`
- **WHEN** a duplicate request arrives
- **THEN** the service MUST return `200` with the original `request_id` and `status=completed`
- **AND** SHALL NOT create a second repo or duplicate audit entries

### Requirement: Audit and events

Every onboarding action SHALL emit immutable audit entries and CloudEvents `app.onboarding.*`, `repo.created.v1`, `branch_protection.applied.v1`, with `correlation_id` linking all stages.

#### Scenario: Full audit chain

- **GIVEN** a successful onboarding
- **WHEN** the auditor queries by `correlation_id`
- **THEN** the auditor MUST see entries for: request received, policy evaluated, scaffold rendered, repo created, branch protection applied, pipelines published, asset registered, completion
