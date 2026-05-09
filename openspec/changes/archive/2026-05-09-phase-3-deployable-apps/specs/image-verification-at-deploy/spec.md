# Spec Delta: image-verification-at-deploy (ADDED)

## ADDED Requirements

### Requirement: Cosign signature verification

Before `Apply`, the orchestrator MUST verify the Cosign signature of the target image against the expected OIDC identity (GitHub Actions workflow on `main` of the source repo).

#### Scenario: Valid signature accepted

- **GIVEN** image `app-foo:abc123` signed via Cosign keyless from `github.com/org/app-foo` on `main`
- **WHEN** verification runs
- **THEN** Cosign MUST succeed
- **AND** emit `deployment.image_verified.v1{outcome=success, identity=...}`

#### Scenario: Invalid signature blocks deploy

- **GIVEN** an image whose signature does not match the expected identity
- **WHEN** verification runs
- **THEN** the deploy MUST be blocked
- **AND** emit `deployment.image_verified.v1{outcome=failed, reason=identity_mismatch}`

### Requirement: SLSA attestation verification

In addition to signature, a SLSA provenance attestation MUST be verified, and `image_digest` MUST match the value registered in the Asset Registry.

#### Scenario: Mismatch between attested digest and registry

- **GIVEN** registry asset records `image_digest=sha256:111`
- **AND** the image being deployed has digest `sha256:222`
- **WHEN** verification runs
- **THEN** verification MUST fail with `digest_mismatch`
- **AND** the deploy MUST be blocked

### Requirement: Rekor lookup

Signature transparency MUST be verified via Rekor (private for `data_classification ∈ {confidential, restricted}`, public otherwise); failure to find a Rekor entry MUST block deploy.

#### Scenario: Missing Rekor entry blocks deploy

- **GIVEN** an image whose signature lacks a Rekor entry
- **WHEN** verification runs
- **THEN** the deploy MUST be blocked with `rekor_entry_missing`

### Requirement: Override with strict TTL

Bypass of signature verification SHALL require an approved override `allow-unsigned-image` with TTL ≤ 1h, granted by `security-approver`, fully audited.

#### Scenario: Override allows one-time unsigned deploy

- **GIVEN** an approved override `allow-unsigned-image` with TTL=30m for `deployment_id=dep-9`
- **WHEN** the deploy runs within TTL
- **THEN** verification MUST be skipped for that deploy only
- **AND** emit `policy.override.consumed.v1`
- **AND** subsequent deploys MUST require new approval
