# skill-artifact-store Specification

## Purpose
Defines how Forge stores, versions, signs and synchronises skill artifacts. Covers the pluggable adapter abstraction over private enterprise backends, public-origin mirroring, provenance chain, versioning/immutability, per-Tenant isolation, and the hybrid sync model that keeps registry entries aligned with their origin over time.

## Requirements

### Requirement: Pluggable artifact-store adapter

The platform SHALL provide `pkg/artifact-store-adapter` with a Go interface (`Put`, `Get`, `Stat`, `Delete`, `Health`) and SHALL ship initial drivers for **Nexus**, **JFrog Artifactory**, **GitHub Packages (private)** and **AWS CodeArtifact**. Per-Tenant binding (`artifact_store_binding`) SHALL select the active driver. Public registries (e.g., npmjs.org) SHALL NOT be offered as storage drivers; using a public registry as the storage backend SHALL be rejected at binding time. Public registries MAY be used as read-only origin sources under the public-origin mirroring requirement below.

#### Scenario: Tenant binding selects Nexus

- **GIVEN** Tenant `t1` configured with `artifact_store_binding=nexus`
- **WHEN** a skill is published for `t1`
- **THEN** the bytes MUST be uploaded through the Nexus driver
- **AND** the registry MUST persist the resulting digest, signature and a `nexus://...` pointer

#### Scenario: Public NPM as storage backend is rejected

- **WHEN** an operator attempts to configure `artifact_store_binding=npm-public`
- **THEN** the binding MUST be rejected with reason `npm_public_not_permitted`
- **AND** the rejection MUST be audited

### Requirement: Public-origin health flag semantics

The `Health` response SHALL distinguish two independent flags:

- `is_public_origin bool` — the artifact was *sourced* from a public registry (npm, GitHub Packages public). Valid metadata; does not block the adapter.
- `is_public_storage bool` — the adapter backend is itself publicly accessible. This remains a terminal misconfiguration; the binding layer SHALL refuse to construct any adapter whose `Health` reports `is_public_storage=true`.

#### Scenario: Public-origin adapter is accepted

- **GIVEN** a tenant adapter with `is_public_origin=true, is_public_storage=false`
- **WHEN** `Health` is probed
- **THEN** the binding layer MUST accept it and record `is_public_origin=true` on all artifacts stored through that path

#### Scenario: Public-storage adapter is rejected

- **GIVEN** any adapter whose `Health` reports `is_public_storage=true`
- **WHEN** the binding layer initialises
- **THEN** it MUST refuse with `misconfiguration: public_storage_not_permitted` and emit `guardrail.trip.v1` with reason `public_storage_misconfiguration`

### Requirement: Public-origin asset mirroring

Skills sourced from public registries (npm, GitHub Packages public) SHALL be mirrored into the Tenant's private artifact store on registration. The registry row SHALL record both the `origin_ref` (e.g., `npm:my-skill@1.2.3`) and the private `stored_at` pointer. All consumer access (gateway, runner, CLI) MUST resolve the private pointer — never the public origin URL — so that every invocation flows through the Skill Gateway with governance intact.

Mirror integrity SHALL be verified by computing the sha256 of the fetched bytes and comparing against the checksum declared by the origin registry. A mismatch MUST abort the mirror and emit `publication.rejected.v1` with reason `origin_checksum_mismatch`.

The mirrored artifact enters `lifecycle_state=mirrored` (not yet active). The asset owner MUST explicitly promote it to `approved` unless `auto_promote_policy` is configured.

#### Scenario: Registering a public-origin skill mirrors it

- **GIVEN** an operator registers skill `cool-skill` from `npm:cool-skill@2.1.0`
- **THEN** Forge MUST fetch the bytes from npm, verify sha256, store via the Tenant adapter, record `origin_ref=npm:cool-skill@2.1.0` and `is_public_origin=true` on the asset row
- **AND** set `lifecycle_state=mirrored`
- **AND** emit `asset.version.mirrored.v1`

#### Scenario: Checksum mismatch aborts mirror

- **GIVEN** the bytes fetched from npm do not match the npm-declared checksum
- **WHEN** the mirror flow runs
- **THEN** Forge MUST abort, emit `publication.rejected.v1{reason=origin_checksum_mismatch}` and leave the asset in `lifecycle_state=proposed`

#### Scenario: Consumer resolves private pointer, not public URL

- **GIVEN** a mirrored-and-promoted skill with `origin_ref=npm:cool-skill@2.1.0`
- **WHEN** Alfred invokes the skill via the Skill Gateway
- **THEN** the gateway MUST resolve the private `stored_at` artifact pointer
- **AND** MUST NOT issue any outbound request to npmjs.org at invocation time

### Requirement: Public-origin promote flow

When a new version of a public-origin asset is mirrored (either on registration or via the Sync Worker), it enters `lifecycle_state=mirrored`. The asset owner SHALL receive a notification and MAY confirm promotion. If the owner has configured `auto_promote_policy` on the asset, minor and patch version bumps (semver) SHALL be promoted automatically without manual confirmation; major version bumps SHALL always require manual confirmation regardless of the policy.

#### Scenario: Owner manually promotes a mirrored version

- **GIVEN** `cool-skill@2.2.0` in `lifecycle_state=mirrored`
- **WHEN** the asset owner confirms promotion in the Portal
- **THEN** the asset MUST transition to `lifecycle_state=approved` and emit `asset.version.promoted.v1`

#### Scenario: Auto-promote applies to minor bump

- **GIVEN** `auto_promote_policy=minor` on asset `cool-skill`, current approved version `2.1.0`
- **WHEN** version `2.2.0` is mirrored
- **THEN** Forge MUST automatically promote `2.2.0` to `approved` and emit `asset.version.promoted.v1` with `trigger=auto_promote`

#### Scenario: Auto-promote does not apply to major bump

- **GIVEN** `auto_promote_policy=minor` on asset `cool-skill`, current approved version `2.1.0`
- **WHEN** version `3.0.0` is mirrored
- **THEN** Forge MUST NOT auto-promote; it MUST notify the asset owner and await manual confirmation

### Requirement: Provenance chain — cosign signature + in-toto attestation

Every published skill artifact SHALL carry a cosign signature and an in-toto attestation produced by the CI pipeline, identical in shape to the chain used for image deploys per `supply-chain-attestations`. The registry SHALL reject publication if the signature or attestation is missing or fails verification against the configured trust root.

#### Scenario: Unsigned artifact rejected

- **WHEN** a publisher submits a skill artifact without a cosign signature
- **THEN** the registry MUST refuse publication with `400 missing_signature`
- **AND** emit `publication.rejected.v1` with reason `missing_signature`

#### Scenario: Tampered artifact rejected at fetch

- **GIVEN** a published skill whose stored bytes have been mutated out-of-band
- **WHEN** a consumer fetches the artifact through the adapter
- **THEN** the adapter MUST verify the digest, return `409 digest_mismatch` on mismatch
- **AND** emit `guardrail.trip.v1` with reason `artifact_digest_mismatch`

### Requirement: Versioning and immutability

Skill artifact versions SHALL follow SemVer. A `(asset_id, version)` tuple SHALL be immutable once published; any change SHALL produce a new version. Adapter drivers MUST refuse overwrite of an existing `(asset_id, version)` digest.

#### Scenario: Republish with same version rejected

- **GIVEN** skill `foo` already published at `1.2.3`
- **WHEN** a publisher attempts to publish `foo@1.2.3` with different bytes
- **THEN** the adapter MUST refuse with `409 version_immutable`
- **AND** the registry MUST instruct the publisher to bump the version

### Requirement: Per-Tenant isolation

Artifact reads and writes SHALL be Tenant-scoped at the adapter layer; a Tenant MUST NOT be able to read or list another Tenant's artifacts even when sharing the same backend instance.

#### Scenario: Cross-Tenant read denied

- **GIVEN** Tenants `t1` and `t2` both bound to the same Nexus instance with separated repositories
- **WHEN** a caller from `t1` requests an artifact owned by `t2`
- **THEN** the adapter MUST return `403 cross_tenant_read_denied`
- **AND** the attempt MUST be audited

### Requirement: Capability flags exposed via Health

Adapter drivers SHALL expose capability flags through `Health(ctx)` (e.g., `supports_retention`, `supports_signed_urls`, `supports_lifecycle_rules`); the registry SHALL gate optional features (such as retention policies on skill versions) behind those flags.

#### Scenario: Retention policy gated by capability

- **GIVEN** a backend that does not advertise `supports_retention`
- **WHEN** an operator attempts to set a retention policy on a skill
- **THEN** the registry MUST refuse with `409 backend_capability_missing`
- **AND** include the capability name in the error

### Requirement: Audit and observability

Every artifact-store operation (`put`, `get`, `delete`) SHALL produce an audit event with actor, `asset_id`, `version`, `digest` and `correlation_id`; OpenTelemetry metrics SHALL include per-Tenant `bytes_in`, `bytes_out`, `latency` and `error_rate`.

#### Scenario: Audit on put

- **WHEN** the CI pipeline publishes a new skill version
- **THEN** an audit event MUST be emitted with `event=artifact.published`, `actor`, `asset_id`, `version`, `digest`, `signature_id`, `attestation_id`, `correlation_id`
