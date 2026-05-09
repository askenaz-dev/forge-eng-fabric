# ai-asset-registry-minimal Specification

## Purpose
TBD - created by archiving change phase-0-foundations. Update Purpose after archive.
## Requirements
### Requirement: Minimal asset model
The Registry SHALL persist assets with at least the following fields: `id`, `name`, `type` ∈ {`mcp`, `skill`, `agent`, `workflow`, `prompt_template`}, `version` (SemVer), `owner_team`, `description`, `inputs_schema`, `outputs_schema`, `lifecycle_state` (defaulting to `proposed` in Fase 0), `workspace_id`, `visibility` ∈ {`workspace`, `tenant`}, `created_at`, `updated_at`.

#### Scenario: Publish a minimal asset
- **WHEN** an authorized publisher submits a valid asset payload
- **THEN** the Registry persists it with `lifecycle_state = proposed`, emits `asset.created.v1` and is discoverable via the Registry API

#### Scenario: Asset missing required fields is rejected
- **WHEN** a submission omits `inputs_schema` or `outputs_schema`
- **THEN** the API returns 400 with a validation error listing missing fields

### Requirement: SemVer enforcement and version immutability
Asset versions SHALL follow SemVer. A given `(asset_id, version)` SHALL be immutable once persisted; further changes SHALL produce a new version.

#### Scenario: Republishing same version with different payload is rejected
- **WHEN** an attempt is made to republish version `1.0.0` with different content
- **THEN** the API returns 409 and instructs the publisher to bump the version

### Requirement: Discovery API
The Registry SHALL expose discovery endpoints to list and filter assets by `type`, `owner_team`, `workspace_id`, `visibility` and `lifecycle_state`, scoped by the caller's OpenFGA relations.

#### Scenario: Discovery is scoped by OpenFGA
- **WHEN** a user queries assets
- **THEN** the response only includes assets the user is allowed to see according to OpenFGA and the asset's `visibility`

### Requirement: Audit on lifecycle/state mutations
Every create/update on assets SHALL produce an audit event including actor, before/after diff (with sensitive fields redacted) and `correlation_id`.

#### Scenario: Update of asset description emits audit
- **WHEN** an owner updates the `description` of an asset
- **THEN** an audit event is emitted with the diff and is queryable in the audit store

### Requirement: Lifecycle transitions are deferred to Fase 1
The Registry SHALL implement the **full asset lifecycle**: `proposed → in_review → approved → deprecated → retired`. Transitions SHALL be auditable. Only `approved` assets SHALL be invocable in production-relevant flows. Promotion to `approved` SHALL require eval scores meeting the threshold defined for the asset's trust level. Promotion of T4 assets SHALL require DevOps/SRE review; promotion of T5 (Critical/Core) assets SHALL require explicit SDLC Team approval. Deprecated assets SHALL remain discoverable with a deprecation banner and recommended replacement.

#### Scenario: Promotion blocked by failing evals
- **WHEN** an owner tries to promote an asset whose eval scores are below the threshold for its trust level
- **THEN** the promotion is rejected and the failing dimensions are returned

#### Scenario: Promotion of T5 asset requires SDLC Team
- **WHEN** an asset at trust level T5 is moved toward `approved`
- **THEN** the transition is held until the SDLC Team approves explicitly, the approval is audited, and only then the asset becomes `approved`

#### Scenario: Production flow rejects non-approved asset
- **WHEN** Alfred attempts to invoke an asset whose `lifecycle_state` is not `approved` in a production-relevant flow
- **THEN** the platform rejects the invocation and audits the attempt

#### Scenario: Deprecated asset is discoverable with warning
- **WHEN** a Workspace lists assets and includes a deprecated one
- **THEN** the asset is shown with a deprecation banner, a pointer to the recommended replacement, and a notice discouraging new adoption### Requirement: Trust levels T0–T5 enforced
The Registry SHALL classify each asset with a trust level: **T0 Experimental, T1 Read-only, T2 Internal Write, T3 SDLC Write, T4 Infra/Deploy, T5 Critical/Core**. Trust level SHALL drive review depth, eval thresholds, allowed environments and required approvers.

#### Scenario: Trust-level change requires re-approval
- **WHEN** an owner increases the trust level of an asset
- **THEN** the asset returns to `in_review` and must be re-approved according to the new level's requirements

### Requirement: Eval scores attached to assets
Approved assets SHALL carry `eval_scores` covering quality, safety, cost and latency from the eval harness. Scores SHALL be visible in the Asset detail view.

#### Scenario: Eval scores visible in detail view
- **WHEN** a user opens an approved asset's detail view
- **THEN** the latest eval scores per dimension and trend over versions are displayed

### Requirement: Application asset deployment sub-resource

The `application` asset type SHALL be extended with a sub-resource `deployment` representing release history per environment; entries MUST be immutable.

#### Scenario: Deployment recorded on asset

- **GIVEN** asset `application/app-foo`
- **WHEN** a deploy completes successfully to `env=stage`
- **THEN** a `deployment` entry MUST appear in the asset linking `revision_id`, `image_digest`, `runtime_id`, `env`, `verified_status`, `openspec_ids`, `pr_sha`, `created_at`, `actor`
- **AND** subsequent UPDATE/DELETE on the entry MUST be rejected by DB triggers

### Requirement: Deployment history query

`GET /v1/assets/{id}/deployments?env=<env>&limit=&cursor=` MUST return paginated history ordered by `created_at desc`.

#### Scenario: Paginate history for an env

- **GIVEN** asset with 50 prod deployments
- **WHEN** querying with `limit=20`
- **THEN** the response MUST return 20 entries plus a `next_cursor`
- **AND** subsequent cursor request MUST return the next page

### Requirement: Verified status flags exposed on asset

The asset response MUST expose `image.signature_verified` and `image.attestation_verified` flags for the latest deploy per env.

#### Scenario: Auditor checks signature status

- **GIVEN** asset `application/app-foo` with a verified deploy in `env=prod`
- **WHEN** the asset is queried
- **THEN** `deployments.prod.latest.image.signature_verified` MUST be `true`
- **AND** `deployments.prod.latest.image.attestation_verified` MUST be `true`

### Requirement: Workflow asset type

The Registry SHALL support asset `type=workflow` with sub-resources `version`, `eval_run`, `installation`.

#### Scenario: Workflow asset registered with versions

- **GIVEN** a published workflow `wf-1`
- **WHEN** queried via `GET /v1/assets/wf-1`
- **THEN** the response MUST list versions, latest eval runs, and installations across Workspaces
- **AND** include lifecycle state per version

#### Scenario: Eval-dataset asset type

- **GIVEN** a registered eval dataset `ds-7`
- **WHEN** queried
- **THEN** the response MUST include version history and trust level
- **AND** the dataset MUST be referenced by workflow eval runs that consumed it
