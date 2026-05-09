# supply-chain-attestations Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: SBOM generation and publication

For every container image built on `main`, the pipeline MUST generate an SBOM in SPDX and CycloneDX formats using Syft and publish it to the artifact store with metadata linked to the image digest.

#### Scenario: SBOM published per image

- **GIVEN** a successful build of `app-foo:abc123`
- **WHEN** the SBOM stage runs
- **THEN** SPDX and CycloneDX SBOMs MUST be produced
- **AND** stored in `sbom_record` with `image_digest=sha256:...`
- **AND** emit `sbom.published.v1`

### Requirement: Keyless image signing with Cosign

Every image published from `main` MUST be signed with Cosign keyless using the GitHub OIDC identity of the workflow.

#### Scenario: Image signed and verifiable

- **GIVEN** a build of `app-foo:abc123`
- **WHEN** the sign stage runs
- **THEN** Cosign MUST sign the image using OIDC token
- **AND** record the signature in `image_signature`
- **AND** emit `image.signed.v1` with signer identity, issuer, and Rekor entry id
- **AND** the signature MUST be verifiable with `cosign verify --certificate-identity=...`

#### Scenario: Reject publish without signature

- **GIVEN** a build whose sign stage failed
- **WHEN** the publish stage runs
- **THEN** publish MUST be blocked
- **AND** emit `image.publish.denied.v1`

### Requirement: SLSA provenance attestation

Every signed image MUST also carry a Cosign attestation with SLSA provenance (build environment, source repo, commit SHA, builder).

#### Scenario: Provenance verifiable

- **GIVEN** a signed image `app-foo:abc123`
- **WHEN** an auditor verifies the attestation
- **THEN** Cosign MUST return the provenance with source repo, commit SHA, and builder identity matching the GitHub Actions workflow

### Requirement: Rekor transparency log routing

Signatures and attestations for repos with `data_classification ∈ {confidential, restricted}` MUST be published to the **private internal Rekor**; for `internal/public` MAY use the public Rekor.

#### Scenario: Confidential repo routes to private Rekor

- **GIVEN** a repo with `data_classification=confidential`
- **WHEN** the sign stage runs
- **THEN** the Rekor entry MUST be created in the private Rekor instance
- **AND** the entry id MUST be recorded in `image_signature.rekor_url`

### Requirement: Verification at deploy time

In Phase 3 deployment will verify signature and attestation; in Phase 2 the contract MUST be in place: every image record SHALL include `signature_verified` and `attestation_verified` flags exposed in the Registry asset metadata.

#### Scenario: Asset exposes verification flags

- **GIVEN** an asset `application/app-foo` with image `app-foo:abc123` registered
- **WHEN** the asset is queried
- **THEN** the response MUST include `image.signature_verified=true` and `image.attestation_verified=true`
