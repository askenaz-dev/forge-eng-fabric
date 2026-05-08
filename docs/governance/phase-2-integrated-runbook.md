# Phase 2 Integrated Runbook

Use this runbook to execute the pilot validation for app onboarding.

## Prerequisites

- GitHub App `Forge` installed in the pilot organization.
- `FORGE_TEMPLATES_DIR` points at `forge-templates/templates`.
- `FORGE_GITHUB_MCP_URL` points at the GitHub MCP write-mode service.
- `FORGE_REGISTRY_URL` and `FORGE_REGISTRY_TOKEN` are configured for app-onboarding.
- Prometheus and Grafana are running from `deploy/compose/docker-compose.yaml`.

## Pilot Onboardings

Run one onboarding per category:

- `go-microservice`
- `nextjs-frontend`
- `iac-terraform-module`

For each onboarding, record:

- request ID
- correlation ID
- repository URL
- bootstrap PR URL
- Registry asset ID
- image digest and SBOM URI when applicable
- OpenSpec ID linked from the PR

## Verification

- Audit includes request, policy, scaffold, repo creation, branch protection, pipeline publication, asset registration and completion entries.
- Events include `app.onboarding.requested.v1`, `app.onboarding.completed.v1`, `repo.created.v1`, `branch_protection.applied.v1`, `pipeline.gate.evaluated.v1`, `image.signed.v1`, `sbom.published.v1` and `pr.linked-to-openspec.v1` where applicable.
- Registry asset is `type=application` with complete metadata.
- PR includes `OpenSpec: <id>` and `forge/openspec-link` passes.
- Image signature and attestation verify with Cosign.
- Grafana `Phase 2 App Onboarding` dashboard shows onboarding duration and SLO rates.

Store evidence under `docs/governance/evidence/phase-2/`.
