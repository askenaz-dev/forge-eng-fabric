## MODIFIED Requirements

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

### Requirement: Idempotency

Onboarding requests SHALL be idempotent based on `(workspace_id, app_id, repo_name)`; a repeated request MUST return the existing request id and current state without duplicating side effects.

#### Scenario: Duplicate request returns existing onboarding

- **GIVEN** an existing onboarding for `(workspace=ws-1, app=app-1, repo_name=svc-foo)` in state `completed`
- **WHEN** a duplicate request arrives
- **THEN** the service MUST return `200` with the original `request_id` and `status=completed`
- **AND** SHALL NOT create a second repo or duplicate audit entries
