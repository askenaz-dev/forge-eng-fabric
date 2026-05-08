# CI Baseline

Every Forge-created repository starts with mandatory checks enforced through branch protection.

## Required Stages

- `forge/lint`
- `forge/test-with-coverage`
- `forge/sast`
- `forge/sca`
- `forge/sbom`
- `forge/container-scan`
- `forge/cosign-sign-attest`
- `forge/openspec-link`

## Gates

Thresholds are configured by criticality in `services/policy-engine/pipeline_rules/onboarding_thresholds.yaml`.

- Coverage minimum is stricter for high and critical apps.
- SAST and SCA findings at or above the configured severity block merge.
- OpenSpec links are required for `criticality>=medium`.
- Overrides require an approved policy override with TTL <= 24 hours.

## Supply Chain

Main branch image publication must include:

- SBOM in SPDX and CycloneDX formats.
- Cosign keyless signature through GitHub OIDC.
- SLSA provenance attestation.
- Private Rekor routing for confidential or restricted repositories.

The Registry lifecycle hook only moves an application asset from `proposed` to `in_review` after a green pipeline, SBOM publication, verified signature and verified attestation.
