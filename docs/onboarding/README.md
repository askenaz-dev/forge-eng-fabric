# App Onboarding

Phase 2 app onboarding is the Forge golden path for creating governed repositories from approved templates.

## Flow

1. Open Portal `New App`.
2. Select an approved template with `trust_level >= T3`.
3. Enter Workspace, repository, owner, criticality, data classification and runtime parameters.
4. Review the preview: CODEOWNERS, branch protection, required checks, image repository and Registry asset metadata.
5. Confirm onboarding.

The app onboarding service then runs these stages:

1. Policy evaluation.
2. Template resolution and scaffolding.
3. GitHub repository creation through the GitHub MCP.
4. CODEOWNERS, PR template, branch protection and required check setup.
5. Registry asset creation as `type=application` with `lifecycle_state=proposed`.

## API

- `POST /v1/onboarding` creates or returns an idempotent onboarding request for `(workspace_id, repo_name)`.
- `GET /v1/onboarding?workspace_id=<id>` lists onboarding history.
- `GET /v1/onboarding/{id}` returns current status.
- `GET /v1/onboarding/{id}/timeline` returns stage events.
- `GET /v1/onboarding/{id}/events` streams Server-Sent Events.
- `GET /v1/templates` lists approved templates.
- `GET /v1/pipeline-gates` lists PR gate results by Workspace, repo and PR.

## Registry Lifecycle

The onboarding service registers applications as proposed assets. Registry lifecycle hooks then move assets through:

- `proposed -> in_review` after the first green pipeline, SBOM publication, and verified image signature plus attestation.
- `in_review -> approved` after Workspace owner approval.

## Pilot Evidence

Pilot onboarding evidence should be stored under `docs/governance/evidence/phase-2/` with request IDs, repository URLs, PR URLs, Registry asset IDs, image digests, SBOM URIs and sign-off records.
