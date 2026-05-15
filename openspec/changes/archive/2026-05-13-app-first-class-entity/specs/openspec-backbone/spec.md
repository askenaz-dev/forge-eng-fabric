## MODIFIED Requirements

### Requirement: Minimum OpenSpec model

An OpenSpec SHALL include at minimum: `openspec_id`, `app_id`, `workspace_id`, `title`, `business_intent`, `problem_statement`, `stakeholders`, `success_metrics`, `requirements (functional + non_functional)`, `constraints`, `autonomy_policy`, `linked_artifacts (jira/github/confluence/figma/ci_cd/deployments)`, `decision_log`, and `audit (created_by, updated_by, version)`. `app_id` SHALL be a foreign key to the `application` table and SHALL be NOT NULL once the rollout migration (M5) completes. `workspace_id` SHALL be derived from `app_id` and SHALL match the parent App's workspace.

#### Scenario: OpenSpec missing required fields is rejected

- **WHEN** an OpenSpec is created without `business_intent`, `requirements.functional` or `app_id`
- **THEN** the platform rejects the operation with a validation error listing missing fields

#### Scenario: Workspace mismatch rejected

- **WHEN** an OpenSpec payload carries `app_id=app-1` and `workspace_id=ws-2` while `app-1.workspace_id=ws-1`
- **THEN** the platform MUST reject the persistence with `422 app_workspace_mismatch`
- **AND** the error MUST point at the conflicting fields

## ADDED Requirements

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
