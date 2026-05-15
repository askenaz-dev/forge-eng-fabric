# ai-asset-registry Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Supported asset types

The Registry SHALL support exactly six asset types: **MCP Server, Agent Skill, Agent, Workflow, Prompt Template, Design System**. Each asset SHALL be uniquely identified, typed, versioned (SemVer) and owned by a team.

#### Scenario: Publish an MCP Server asset

- **WHEN** a team publishes a new MCP Server with name, version, owner and metadata
- **THEN** the Registry persists the asset, assigns lifecycle state `proposed`, and emits a publication event

#### Scenario: Publish a Design System asset

- **WHEN** the design team publishes a new Design System with `name`, `version`, `owner`, `manifest.tokens`, `manifest.components`, `manifest.fonts`, `manifest.screenshots`, `manifest.use_case`
- **THEN** the Registry persists the asset with `type=design_system`, `lifecycle_state=proposed`, and emits `asset.design_system.published.v1`

#### Scenario: Reject unknown asset type

- **WHEN** any caller submits an asset with `type` not in {mcp-server, agent-skill, agent, workflow, prompt-template, design-system}
- **THEN** the Registry MUST reject with `422 unsupported_asset_type`

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

### Requirement: How-to block required on every asset

Every Asset Registry record SHALL include a `how_to` block describing install command(s) per supported client, usage snippet(s) per supported language, and environment requirements. The block SHALL validate against a JSON schema; assets failing validation SHALL be rejected at submission.

#### Scenario: Asset missing how-to is rejected

- **WHEN** an asset is submitted without a `how_to` block
- **THEN** the registry MUST return `400 missing_how_to`
- **AND** list the required sub-fields in the error

#### Scenario: How-to renders in Portal

- **GIVEN** an approved asset with a populated `how_to` block
- **WHEN** a user opens the asset detail in the Portal
- **THEN** a **How-to** tab MUST render the install command and usage snippets per client

### Requirement: Active-surface block required on every asset

Every Asset Registry record SHALL include an `active_surface` block describing where the asset is reachable at runtime: `family ∈ {mcp, a2a, skill}` plus either a gateway endpoint (`mcp-gateway` / `a2a-gateway`) or an artifact pointer (`skill-artifact-store` URI + digest + signature id). The block SHALL be validated against the active-surface schema.

#### Scenario: MCP asset publishes its gateway endpoint

- **WHEN** an internal MCP is published
- **THEN** the registry MUST set `active_surface = { family: "mcp", endpoint: "/v1/gw/mcp/<asset_id>" }`
- **AND** the endpoint MUST resolve through `mcp-gateway`

#### Scenario: Skill asset publishes its artifact pointer

- **WHEN** a skill is published
- **THEN** the registry MUST set `active_surface = { family: "skill", artifact_pointer: "<adapter>://...", digest, signature_id }`
- **AND** consumers MUST fetch through `skill-artifact-store`, never directly from the underlying backend

### Requirement: Approved lifecycle requires the catalog/how-to/gateway triad

Promotion of an asset to `lifecycle_state=approved` SHALL be rejected unless all of: minimum metadata, a populated `how_to` block, a populated `active_surface` block, and the eval-score threshold for the asset's trust level are satisfied.

#### Scenario: Promotion blocked by missing active surface

- **GIVEN** an asset with valid metadata, `how_to` populated, eval scores above threshold, but `active_surface=null`
- **WHEN** an owner attempts to promote to `approved`
- **THEN** the registry MUST refuse with `409 missing_active_surface`

### Requirement: External provenance flag for third-party MCPs and agents

The Asset Registry SHALL support a `provenance ∈ {internal, external}` flag on `mcp` and `agent` asset types. External assets SHALL carry `transport.endpoint`, a per-Tenant `credential_ref`, an optional `allowlist`, and the upstream manifest or agent-card hash captured at registration. External assets SHALL move through the same lifecycle and trust pipeline as internal assets.

#### Scenario: External MCP registered with manifest hash

- **WHEN** an operator registers an external MCP `vendor-x` with `transport.endpoint` and `credential_ref`
- **THEN** the registry MUST fetch the live tool manifest, persist its hash on the asset, and emit `com.forge.asset.external_registered.v1`

#### Scenario: External asset promotion re-verifies upstream

- **GIVEN** external MCP `vendor-x` previously registered with manifest hash `H1`
- **WHEN** an owner attempts to promote `vendor-x` from `in_review` to `approved`
- **THEN** the registry MUST fetch the live manifest, compute hash `H2`, and refuse the promotion if `H2 ≠ H1` unless the change is explicitly acknowledged in the promotion request

### Requirement: Drift detection on external assets

The Asset Registry SHALL run a daily drift cron over every `provenance=external` asset, re-fetching the upstream manifest or agent card, comparing the hash against the stored value, and emitting drift events. Drift exceeding policy thresholds SHALL move the asset to `deprecated` automatically.

#### Scenario: Drift moves asset to deprecated

- **WHEN** the daily cron detects drift on `vendor-x` beyond the policy threshold
- **THEN** the registry MUST set `vendor-x` to `lifecycle_state=deprecated`
- **AND** emit `com.forge.asset.external_drift_deprecated.v1`
- **AND** notify the owning team via the configured channel

### Requirement: Distribution metadata coexists with active surface

The `distribution` block introduced by `add-developer-skill-gateway` SHALL coexist with `active_surface`; `distribution` describes external developer-client publication (Agent Skills bundle, channel, package digest), while `active_surface` describes runtime invocation seam. Both MAY be present and SHALL NOT contradict.

#### Scenario: Both fields populated on a gateway-published skill

- **GIVEN** an approved skill that is both gateway-published for external IDEs and exposed through `skill-artifact-store` internally
- **WHEN** the asset is read
- **THEN** the response MUST include a `distribution` block (gateway-published state, channel, digest) and an `active_surface` block (`family=skill`, artifact pointer, digest, signature)
- **AND** the digests in the two blocks MUST be equal

### Requirement: Assets carry their producing App scope

Every asset record produced by an App SHALL include `app_id` in addition to `workspace_id`. The Registry SHALL accept `app_id` on `POST /v1/workspaces/{ws}/assets`, on the lifecycle hooks and on every PATCH. Asset lists SHALL be filterable by `app_id`. When an asset is published from outside an App context (e.g., platform-level shared skills), `app_id` MAY be null and the asset MUST be flagged `scope=workspace`.

#### Scenario: Assets created with app scope

- **GIVEN** an authenticated user with `app#editor` on `app-1`
- **WHEN** the user calls `POST /v1/workspaces/ws-1/assets` with `app_id=app-1, type=agent-skill, ...`
- **THEN** the Registry MUST persist the asset with `app_id=app-1, workspace_id=ws-1`
- **AND** include `app_id` on the publication event payload

#### Scenario: Filter assets by App

- **WHEN** a caller calls `GET /v1/workspaces/ws-1/assets?app_id=app-1`
- **THEN** the response MUST list only assets whose `app_id=app-1`
- **AND** the response MUST NOT include workspace-scoped (null `app_id`) assets unless an explicit `include_workspace_scope=true` is passed

#### Scenario: Reject app_id from another workspace

- **WHEN** a caller calls `POST /v1/workspaces/ws-1/assets` with `app_id=app-2` where `app-2.workspace_id=ws-2`
- **THEN** the Registry MUST reject the request with `403 cross_workspace_app_reference`

### Requirement: App archival cascades to asset visibility

When an App is archived, the Registry SHALL mark every asset with `app_id=<archived>` as `discoverable=false` for new consumers, while preserving invocability for existing dependents. When the App is restored, discoverability SHALL be restored.

#### Scenario: Archive cascade

- **GIVEN** an App `app-1` with 3 approved assets
- **WHEN** an owner calls `POST /v1/apps/app-1:archive`
- **THEN** the Registry MUST set `discoverable=false` on the 3 assets within the same transaction (or compensate on failure)
- **AND** emit `asset.discoverability.changed.v1` for each

### Requirement: Design System asset metadata

A Design System asset record SHALL include `manifest.tokens` (HTTPS URL to the token CSS sheet, sha256-pinned), `manifest.components` (HTTPS URL to the component pack archive, sha256-pinned), `manifest.fonts` (array of font preload entries `{family, weights, italic, source}`), `manifest.screenshots` (`{light: url, dark: url}` with both URLs sha256-pinned), `manifest.use_case` (a string of at most 240 characters describing the intended look-and-feel). Submissions missing any of these MUST be rejected with `422 missing_design_system_manifest_field`.

#### Scenario: Manifest missing screenshots is rejected

- **WHEN** a publisher submits a Design System asset without `manifest.screenshots.dark`
- **THEN** the Registry MUST reject with `422 missing_design_system_manifest_field` listing `manifest.screenshots.dark`

#### Scenario: Tokens URL must be sha256-pinned

- **WHEN** a publisher submits `manifest.tokens=https://example/tokens.css` without an explicit `manifest.tokens_sha256`
- **THEN** the Registry MUST reject with `422 missing_token_digest`

### Requirement: Design System eval scores

Approval of a Design System asset SHALL require `eval_scores.accessibility >= 0.9` (Axe-derived) and `eval_scores.brand_fidelity >= 0.8` (rubric-driven). Built-in templates SHALL ship with these scores attached at publication.

#### Scenario: Approve fails under accessibility threshold

- **GIVEN** a Design System submission with `eval_scores.accessibility=0.85`
- **WHEN** the transition to `approved` is requested
- **THEN** the Registry MUST refuse with `409 eval_below_threshold` listing the failing dimension
