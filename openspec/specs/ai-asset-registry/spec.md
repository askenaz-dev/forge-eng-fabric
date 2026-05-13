# ai-asset-registry Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Supported asset types
The Registry SHALL support exactly five asset types: **MCP Server, Agent Skill, Agent, Workflow, Prompt Template**. Each asset SHALL be uniquely identified, typed, versioned (SemVer) and owned by a team.

#### Scenario: Publish an MCP Server asset
- **WHEN** a team publishes a new MCP Server with name, version, owner and metadata
- **THEN** the Registry persists the asset, assigns lifecycle state `proposed`, and emits a publication event

### Requirement: Required metadata
Every asset SHALL include at minimum: `id`, `name`, `type`, `version`, `owner_team`, `description`, `inputs_schema`, `outputs_schema`, `required_permissions`, `data_sensitivity`, `cost_class`, `eval_scores`, `trust_level`, `sla`, `runbook_url`, `openspec_link` (when applicable), `lifecycle_state`, `examples`, `audit_policy`.

#### Scenario: Asset missing required metadata is rejected
- **WHEN** an asset is submitted without `inputs_schema` or `outputs_schema`
- **THEN** the Registry rejects the submission with a validation error listing missing fields

### Requirement: Lifecycle states and transitions
Each asset SHALL progress through the lifecycle: **proposed → in_review → approved → deprecated → retired**. Transitions SHALL be auditable. Only `approved` assets SHALL be invocable in production environments.

#### Scenario: Only approved assets are invocable in prod
- **WHEN** Alfred attempts to invoke an asset in lifecycle state `proposed` or `in_review` in a production environment
- **THEN** the platform rejects the invocation and audits the attempt

#### Scenario: Deprecation preserves discoverability with warning
- **WHEN** an asset is moved to `deprecated`
- **THEN** the asset remains discoverable with a deprecation banner and recommended replacement, but new adoptions are discouraged

### Requirement: SemVer versioning and immutability
Asset versions SHALL follow SemVer. Once published, a specific version SHALL be immutable; changes SHALL produce a new version. Breaking changes SHALL increment the major version.

#### Scenario: Republishing same version is rejected
- **WHEN** a team attempts to republish version `1.2.3` of an existing asset with different content
- **THEN** the Registry rejects the operation and instructs the publisher to bump the version

### Requirement: Trust levels T0–T5
The Registry SHALL classify each asset with a trust level: **T0 Experimental, T1 Read-only, T2 Internal Write, T3 SDLC Write, T4 Infra/Deploy, T5 Critical/Core**. Higher trust levels SHALL require additional reviews and tighter policies.

#### Scenario: T5 critical asset requires SDLC Team approval
- **WHEN** an asset is submitted or moved into trust level T5
- **THEN** the lifecycle transition to `approved` requires explicit SDLC Team approval recorded in audit

### Requirement: Eval scores attached to assets
Approved assets SHALL carry `eval_scores` from the platform eval harness covering quality, safety, cost and latency dimensions. Approval SHALL require minimum scores defined by policy per trust level.

#### Scenario: Asset below eval threshold cannot be approved
- **WHEN** an asset's eval scores are below the configured threshold for its trust level
- **THEN** the lifecycle transition to `approved` is rejected and the publisher receives the failing dimensions

### Requirement: Tenant-scoped sharing with ownership preserved
Assets SHALL be shareable inside the same Tenant according to visibility settings (private to Workspace or shared with Tenant) while preserving the publishing Workspace as owner.

#### Scenario: Asset shared across Workspaces of the same Tenant
- **WHEN** Workspace B searches for an asset published with Tenant visibility by Workspace A in the same Tenant
- **THEN** the asset is discoverable and invocable by Workspace B subject to its own policies, with ownership remaining in Workspace A

### Requirement: Distribution metadata block

Every asset record SHALL carry a `distribution` block with `gateway_published: boolean`, `gateway_channel: stable|beta`, `package_digest: string|null` (sha256 of the latest packaged bundle, for type `skill`/`agent`), `package_signed_at: timestamp|null` and `deprecation_pointer: asset_ref|null`. The block SHALL be returned by every `GET` on the asset and SHALL be initialised to `gateway_published=false`, `gateway_channel=stable`.

#### Scenario: Distribution defaults

- **WHEN** a new asset is created via `POST /v1/workspaces/{id}/assets`
- **THEN** the response includes `distribution.gateway_published=false` and `distribution.package_digest=null`

#### Scenario: Distribution surfaces on list

- **WHEN** any caller lists assets in a workspace
- **THEN** each returned item includes the full `distribution` block

### Requirement: Gateway-publication eligibility

An asset SHALL only be `distribution.gateway_published=true` when ALL of the following hold: `lifecycle_state=approved`, `trust_level>=T1`, a non-null `package_digest` (for `skill`/`agent`) or a declared `remote_transport` (for `mcp`), and a current signature with a passing supply-chain attestation. Transitions that would violate any condition SHALL be rejected with `409 distribution_invariant_violated`.

#### Scenario: Cannot publish a proposed asset

- **GIVEN** an asset in `lifecycle_state=proposed`
- **WHEN** a publisher calls the publication hook
- **THEN** the registry responds `409 distribution_invariant_violated`

#### Scenario: Lifecycle regression unpublishes

- **GIVEN** a gateway-published, approved asset
- **WHEN** the asset is transitioned to `deprecated`
- **THEN** `distribution.gateway_published` MUST flip to `false` automatically
- **AND** `distribution.deprecation_pointer` MUST be set if a replacement is recorded
- **AND** an `assets.gateway_unpublished.v1` event MUST be emitted

### Requirement: Publication lifecycle hook

The registry SHALL expose `POST /v1/assets/{id}/versions/{v}/lifecycle-hooks/gateway-publish` accepting `{ channel, package_digest, signature_id, attestation_id }` for skill/agent assets and `{ channel, remote_transport }` for MCPs. The hook SHALL atomically (a) verify the asset is approved and T1+, (b) verify the signature + attestation chain, (c) write `asset_package` (for skill/agent) and (d) set `distribution.gateway_published=true`.

#### Scenario: Publish an approved skill

- **GIVEN** an approved T2 skill with a valid signed bundle
- **WHEN** the publish hook is called
- **THEN** `asset_package` is written with the supplied digest
- **AND** `distribution.gateway_published=true`
- **AND** `com.forge.asset.gateway_published.v1` is emitted

#### Scenario: Signature mismatch rejected

- **WHEN** the supplied signature does not verify against the package digest
- **THEN** the hook responds `400 signature_invalid`
- **AND** no state change occurs

### Requirement: Gateway-published event

The registry SHALL emit `com.forge.asset.gateway_published.v1` whenever `distribution.gateway_published` transitions from `false` to `true`, and `com.forge.asset.gateway_unpublished.v1` for the inverse. Events SHALL include `asset_id`, `version`, `tenant_id`, `channel`, `package_digest|remote_transport`, `actor` and `correlation_id`, conforming to the CloudEvents envelope used by the registry.

#### Scenario: Event is consumable by observability

- **WHEN** an asset is gateway-published
- **THEN** the asset-observability service receives `com.forge.asset.gateway_published.v1` within 5 seconds
- **AND** the event passes the platform's CloudEvents schema validation
