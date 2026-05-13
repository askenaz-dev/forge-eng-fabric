## ADDED Requirements

### Requirement: Pluggable artifact-store adapter

The platform SHALL provide `pkg/artifact-store-adapter` with a Go interface (`Put`, `Get`, `Stat`, `Delete`, `Health`) and SHALL ship initial drivers for **Nexus**, **JFrog Artifactory**, **GitHub Packages (private)** and **AWS CodeArtifact**. Per-Tenant binding (`artifact_store_binding`) SHALL select the active driver. Public NPM (npmjs.org) SHALL NOT be an offered driver and SHALL be rejected if configured.

#### Scenario: Tenant binding selects Nexus

- **GIVEN** Tenant `t1` configured with `artifact_store_binding=nexus`
- **WHEN** a skill is published for `t1`
- **THEN** the bytes MUST be uploaded through the Nexus driver
- **AND** the registry MUST persist the resulting digest, signature and a `nexus://...` pointer

#### Scenario: Public NPM backend is rejected

- **WHEN** an operator attempts to configure `artifact_store_binding=npm-public`
- **THEN** the binding MUST be rejected with reason `npm_public_not_permitted`
- **AND** the rejection MUST be audited

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
