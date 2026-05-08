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
In Fase 0 the Registry SHALL accept assets in `lifecycle_state = proposed` and SHALL NOT enforce transitions to `in_review`, `approved`, `deprecated` or `retired`. The full lifecycle and trust-level enforcement SHALL be introduced by `phase-1-agentic-core`.

#### Scenario: Attempt to set state beyond `proposed` is rejected
- **WHEN** a client tries to set `lifecycle_state` to anything other than `proposed` in Fase 0
- **THEN** the API returns 409 with a message pointing to Fase 1 for full lifecycle support

