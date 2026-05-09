# deployment-history Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Immutable revision history

Every deployment MUST persist a `revision_id` capturing `(asset_id, env, image_digest, manifest_hash, timestamp, actor, openspec_ids, pr_sha)`; revisions are immutable.

#### Scenario: Revision recorded for every deploy

- **GIVEN** a successful deploy of `app-foo:abc123` to `env=stage`
- **WHEN** the deploy completes
- **THEN** a row MUST be persisted in `deployment` with a unique `revision_id`
- **AND** `image_digest` and `manifest_hash` MUST be present
- **AND** the row MUST be immutable (DB triggers reject UPDATE/DELETE)

### Requirement: One-click rollback

The Portal SHALL surface a "Rollback to previous" action on each deployment, executing `POST /v1/deployments/{id}/rollback`.

#### Scenario: Rollback restores previous revision

- **GIVEN** a deployment `dep-9` (revision r9) preceded by `dep-8` (revision r8) in `env=stage`
- **WHEN** an operator clicks "Rollback to previous" on `dep-9`
- **THEN** the orchestrator MUST re-apply revision r8 with full pipeline (policy, verify, apply, verify)
- **AND** record a new deployment `dep-10` with `source_revision=r8` and `rollback_of=dep-9`
- **AND** emit `deployment.rolled_back.v1`

### Requirement: Asset-level history API

`GET /v1/assets/{id}/deployments?env=<env>` MUST return paginated deployment history with revision metadata, status, and links to OpenSpec/PR/runtime.

#### Scenario: Auditor retrieves prod history

- **GIVEN** asset `app-foo` with 50 prod deploys
- **WHEN** auditor queries `GET /v1/assets/app-foo/deployments?env=prod&limit=20`
- **THEN** the response MUST contain 20 deployments ordered by `created_at desc`
- **AND** include `revision_id`, `image_digest`, `verified_status`, `openspec_ids`, `pr_sha`, `runtime_id`, `actor`

### Requirement: Audit linkage

Each deployment record MUST be linked to the audit trail via `correlation_id`; querying audit by `correlation_id` MUST reproduce the full stage timeline.

#### Scenario: Audit reconstruction by correlation_id

- **GIVEN** a deployment with `correlation_id=corr-42`
- **WHEN** the auditor queries audit by `corr-42`
- **THEN** entries for request, policy, image-verify, apply, verify, notify MUST be present in order
