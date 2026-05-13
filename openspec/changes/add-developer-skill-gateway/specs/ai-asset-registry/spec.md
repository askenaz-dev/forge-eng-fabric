## ADDED Requirements

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
