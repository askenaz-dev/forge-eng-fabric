## ADDED Requirements

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
