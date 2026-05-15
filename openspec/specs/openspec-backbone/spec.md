# openspec-backbone Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: OpenSpec is the living contract for relevant changes
Every relevant change in Forge SHALL originate from or be referenced by an **OpenSpec** document. Relevance SHALL be defined by Workspace policy (e.g., user-facing changes, architecture changes, security-impacting changes, deployments to staging/prod).

#### Scenario: Production deploy requires linked OpenSpec
- **WHEN** Alfred or a user triggers a deploy to a production environment
- **THEN** the platform requires a linked, current OpenSpec; otherwise the action is blocked with a clear message

### Requirement: OpenSpec editable by Alfred and authorized humans
OpenSpecs SHALL be editable by Alfred (acting on behalf of authorized principals) and by humans with appropriate Workspace permissions. Edits SHALL be versioned and attributed.

#### Scenario: Concurrent edit produces auditable resolution
- **WHEN** a human edits an OpenSpec while Alfred has a pending suggestion
- **THEN** both contributions are recorded, conflict is resolved per configured strategy, and actors/policies are audited

### Requirement: Minimum OpenSpec model

An OpenSpec SHALL include at minimum: `openspec_id`, `app_id`, `workspace_id`, `title`, `business_intent`, `problem_statement`, `stakeholders`, `success_metrics`, `requirements (functional + non_functional)`, `constraints`, `autonomy_policy`, `linked_artifacts (jira/github/confluence/figma/ci_cd/deployments)`, `decision_log`, and `audit (created_by, updated_by, version)`. `app_id` SHALL be a foreign key to the `application` table and SHALL be NOT NULL once the rollout migration (M5) completes. `workspace_id` SHALL be derived from `app_id` and SHALL match the parent App's workspace.

#### Scenario: OpenSpec missing required fields is rejected

- **WHEN** an OpenSpec is created without `business_intent`, `requirements.functional` or `app_id`
- **THEN** the platform rejects the operation with a validation error listing missing fields

#### Scenario: Workspace mismatch rejected

- **WHEN** an OpenSpec payload carries `app_id=app-1` and `workspace_id=ws-2` while `app-1.workspace_id=ws-1`
- **THEN** the platform MUST reject the persistence with `422 app_workspace_mismatch`
- **AND** the error MUST point at the conflicting fields

### Requirement: Bidirectional traceability with external systems
OpenSpec SHALL maintain bidirectional links with GitHub (issues, PRs, commits), Jira (epics, stories, tasks), Confluence (pages), Figma (when used), CI/CD pipelines and deployments. Links SHALL be navigable from the Portal in both directions.

#### Scenario: Navigate from PR to OpenSpec and back
- **WHEN** a user opens a PR connected to an OpenSpec
- **THEN** the Portal displays the OpenSpec from the PR view and lists the PR among the OpenSpec's linked artifacts

### Requirement: Decision log
Each OpenSpec SHALL maintain a `decision_log` with `id`, `actor`, `decision`, `timestamp` and `rationale`. Decisions made by Alfred SHALL include the policy evaluated and `correlation_id`.

#### Scenario: Alfred records a decision in the OpenSpec
- **WHEN** Alfred makes a relevant decision (e.g., choosing a runtime)
- **THEN** the decision is appended to `decision_log` with rationale and policy reference### Requirement: Autonomy policy embedded
Each OpenSpec SHALL include an `autonomy_policy` block declaring `default_mode` (autonomous/manual) and `approvals_required` per action class. The platform SHALL enforce this policy when Alfred operates against the OpenSpec scope.

#### Scenario: OpenSpec policy overrides Workspace default
- **WHEN** an OpenSpec declares `approvals_required` for `deploy:staging` while the Workspace default is autonomous
- **THEN** the platform requires approval for staging deploys related to that OpenSpec

### Requirement: OpenSpec is the living contract
Every relevant change in Forge SHALL originate from or be referenced by an **OpenSpec**. Relevance is defined by Workspace policy.

#### Scenario: Production-relevant action without OpenSpec is blocked
- **WHEN** Alfred or a user triggers a production-relevant action without a linked OpenSpec
- **THEN** the platform blocks the action with a clear error and offers to create or link an OpenSpec

### Requirement: Bidirectional traceability
OpenSpec SHALL maintain bidirectional links with GitHub (issues, PRs, commits), Jira (epics, stories, tasks), Confluence (pages), Figma (when used), CI/CD pipelines and deployments. Links SHALL be navigable in both directions from the Portal.

#### Scenario: Navigate from PR to OpenSpec and back
- **WHEN** a user opens a PR connected to an OpenSpec
- **THEN** the Portal displays the linked OpenSpec from the PR view, and the OpenSpec lists the PR among `linked_artifacts`

### Requirement: Embedded autonomy policy
Each OpenSpec SHALL include an `autonomy_policy` block (`default_mode`, `approvals_required`). Policy enforcement SHALL apply when Alfred operates against the OpenSpec scope, overriding Workspace defaults when more restrictive.

#### Scenario: OpenSpec policy overrides Workspace default
- **WHEN** the OpenSpec requires approval for `deploy:staging` while the Workspace default is autonomous
- **THEN** the platform requires approval for staging deploys related to that OpenSpec

### Requirement: Decision log extended with Jira/Confluence/test/SLO entries

The OpenSpec `decision_log` SHALL accept entry types `jira_link`, `confluence_link`, `test_run_link`, `slo_link`, `incident_link`, `cost_record_link` in addition to the existing types.

#### Scenario: Jira link recorded on issue creation

- **GIVEN** an OpenSpec `spec-7` linked to initiative `init-1`
- **WHEN** Alfred creates Jira epic `ENG-100` referencing the OpenSpec
- **THEN** `spec-7.decision_log` MUST receive an entry `{type: jira_link, key: ENG-100, url: ..., created_by: alfred, at: ...}`
- **AND** the OpenSpec version MUST be bumped if mutability rules require

### Requirement: Linked artifacts namespaces

The `linked_artifacts` field SHALL support namespaces `jira:`, `confluence:`, `test:`, `slo:`, `incident:`, `cost:` in addition to existing namespaces.

#### Scenario: Linked artifacts queryable

- **GIVEN** `spec-7` with linked artifacts including `jira:ENG-100`, `slo:slo-12`, `confluence:DESIGN-42`
- **WHEN** the OpenSpec is fetched
- **THEN** `linked_artifacts` MUST list all entries with their namespace and external id
- **AND** the Portal viewer MUST render tabs grouped by namespace

### Requirement: Accept evolution-loop proposals

The backbone SHALL accept change proposals carrying marker `source=autonomous-loop`; such proposals MUST land in the Evolution Inbox and require human approval before being converted to a normal change.

#### Scenario: Autonomous-loop proposal queued for human review

- **GIVEN** an evolution proposal generated by Alfred
- **WHEN** submitted
- **THEN** it MUST be persisted with `source=autonomous-loop` and visible in the Evolution Inbox
- **AND** MUST NOT be applied directly to specs

#### Scenario: Approval converts proposal into a normal change

- **GIVEN** an inbox approver accepting `prop-7`
- **WHEN** they confirm
- **THEN** a normal OpenSpec change MUST be created carrying the proposal contents
- **AND** the proposal record MUST be marked `adopted_as=<change_id>`
- **AND** the new change MUST follow the standard lifecycle

### Requirement: Progressive draft state for OpenSpecs

The OpenSpec backbone SHALL support a `status` lifecycle of `drafting → validating → committed → abandoned`. Drafts SHALL share the same `openspec_id` namespace as committed OpenSpecs and SHALL be persisted across sessions, but SHALL NOT satisfy production-relevance gates until they reach `committed`.

#### Scenario: Draft persists across sessions

- **WHEN** a user starts an intent dialogue, leaves, and returns later
- **THEN** the draft SHALL be retrievable by the same `draft_id`/`openspec_id` and resumable from the last persisted state

#### Scenario: Draft cannot satisfy production-relevance gates

- **WHEN** a deployment to production references an OpenSpec in `drafting` state
- **THEN** the platform SHALL block the deployment with an error stating that only committed OpenSpecs satisfy production gates

#### Scenario: Atomic commit transitions draft to committed

- **WHEN** the wizard or any caller invokes the commit API on a complete draft
- **THEN** the OpenSpec SHALL transition from `drafting` to `committed` in a single atomic operation; partial commits SHALL NOT be observable

### Requirement: Per-field completeness reporting

The OpenSpec backbone SHALL expose a completeness API that returns, for any draft, which required and recommended fields are complete, partial, or empty, sufficient for the wizard to choose the next question.

#### Scenario: Completeness reflects current draft

- **WHEN** the wizard calls `GET /v1/openspecs/{id}/completeness`
- **THEN** the response SHALL list each section (`business_intent`, `problem_statement`, `requirements.functional`, `requirements.non_functional`, `constraints`, `autonomy_policy`, `stakeholders`, `success_metrics`) with status `complete | partial | empty` and field-level detail

#### Scenario: Completeness updates after a turn

- **WHEN** an answer adds or modifies fields on a draft
- **THEN** the next call to the completeness API SHALL reflect the changes within the same logical transaction

### Requirement: Draft expiry and cleanup

Drafts in `drafting` state SHALL expire after 14 days of inactivity. Expiry SHALL transition the draft to `abandoned` and SHALL NOT delete the audit trail.

#### Scenario: Inactive draft expires

- **WHEN** a draft has had no activity for 14 days
- **THEN** the next cleanup run SHALL transition it to `abandoned` and emit an `intent.draft.abandoned.v1` event

#### Scenario: Audit trail preserved on expiry

- **WHEN** a draft transitions to `abandoned`
- **THEN** all dialogue turns, field changes, and policy decisions SHALL remain in audit storage subject to the data retention policy

### Requirement: Orphan-spec migration (hard delete with audit)

The backbone SHALL ship a migration job that classifies every existing OpenSpec into one of three buckets: `retain_with_target_app` (a clear App can be inferred), `retain_unassigned` (no clear App but the spec is still actively referenced) and `orphan` (a deletion candidate). Orphans SHALL be hard-deleted only after (a) a dry-run CSV is produced for the parent Workspace owner, (b) the Workspace owner explicitly confirms the deletion list, and (c) the full record contents are copied to the immutable audit retention bucket. Each deletion SHALL emit `spec.purged.v1`.

#### Scenario: Migration produces dry-run report

- **WHEN** the migration job runs in `mode=dry-run` for `ws-1`
- **THEN** the job MUST produce `migration-dry-run-{ws-1}-{timestamp}.csv` listing every spec with columns `{spec_id, classification, last_activity, evidence}`
- **AND** the job MUST NOT mutate any spec

#### Scenario: Hard delete requires owner confirmation

- **GIVEN** a dry-run report for `ws-1` with `K` orphans
- **WHEN** the migration job is invoked in `mode=execute` without a recorded owner confirmation for that report
- **THEN** the job MUST refuse to delete and exit with `412 missing_owner_confirmation`

#### Scenario: Spec.purged event carries full body

- **WHEN** the migration job hard-deletes an orphan
- **THEN** exactly one `spec.purged.v1` event MUST be emitted carrying the full prior spec body, the classification evidence, and the `correlation_id` linking back to the audit copy

### Requirement: intent.committed.v1 carries app_id

The `intent.committed.v1` event SHALL include `app_id` in its payload. Subscribers (SDLC orchestrator, observability, audit) SHALL be able to read `app_id` from the event without an extra lookup.

#### Scenario: Event payload includes app_id

- **WHEN** the Intent Capture Wizard commits a draft for `app_id=app-1`
- **THEN** the emitted `intent.committed.v1` event MUST contain `app_id=app-1` alongside `openspec_id` and `workspace_id`

### Requirement: Spec re-parent API

The backbone SHALL expose `POST /v1/specs/{id}:reparent` accepting `{target_app_id, reason}`. The caller MUST have `app#editor` on both the source and the target App. The operation SHALL be atomic and SHALL emit `spec.reparented.v1`.

#### Scenario: Re-parent succeeds with editor on both apps

- **GIVEN** an OpenSpec `spec-7` with `app_id=app-1` and a caller with `app#editor` on both `app-1` and `app-2`
- **WHEN** the caller calls `POST /v1/specs/spec-7:reparent` with `{target_app_id: app-2, reason: "consolidating product surface"}`
- **THEN** the backbone MUST atomically update `spec-7.app_id=app-2`
- **AND** emit `spec.reparented.v1` with `{spec_id: spec-7, from_app_id: app-1, to_app_id: app-2, reason: "consolidating product surface"}`

#### Scenario: Re-parent refused without editor on target

- **GIVEN** a caller with `app#editor` on `app-1` but only `app#viewer` on `app-2`
- **WHEN** the caller calls re-parent from `app-1` to `app-2`
- **THEN** the backbone MUST reject the request with `403 missing_app_editor_on_target`
