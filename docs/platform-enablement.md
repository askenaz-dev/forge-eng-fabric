# Platform Enablement Guide

This is the living step-by-step guide for enabling Forge Engineering Fabric across all phases. Keep this document current as OpenSpec changes are implemented from Phase 0 through later phases.

> **New to the platform?** Start with the [Tenancy Model](concepts/tenancy-model.md) to understand how Tenants, Business Units, and Workspaces relate before working through this guide.

## How To Use This Guide

1. Start here for the full platform bootstrap path.
2. Follow phase-specific runbooks only when this guide links to them.
3. Update this guide whenever a phase adds a service, dependency, environment variable, seed step, migration, validation command or sign-off requirement.

## Source Of Truth

| Area | Path |
|---|---|
| Active OpenSpec changes | `openspec/changes/` |
| Phase 0 tasks | `openspec/changes/phase-0-foundations/tasks.md` |
| Phase 1 tasks | `openspec/changes/phase-1-agentic-core/tasks.md` |
| Local compose stack | `deploy/compose/docker-compose.yaml` |
| Database migrations | `db/migrations/` |
| Helm charts | `infra/helm/` |
| Terraform modules | `infra/terraform/` |
| Governance sign-offs | `docs/governance/` |
| Runbooks | `docs/runbooks/` |

## Global Prerequisites

| Tool | Minimum | Why |
|---|---:|---|
| Docker Desktop / Docker Engine | Current | Local dependencies and compose stack |
| Docker Compose v2 | Current | `docker compose` workflows |
| Go | 1.22 | Go services and tests |
| Python | 3.12 | Python services, skills and tests |
| uv | Current | Python package/test runner |
| Node.js | 20 | Portal and generated TypeScript clients |
| pnpm via Corepack | Current | Portal workspace package manager |
| openspec CLI | Current | Change/task status and archive workflows |
| GitHub CLI | Current | PR/release automation when needed |
| Terraform | 1.6+ | Cloud infra phases |
| Helm | 3.x | Kubernetes deployment phases |
| kubectl | Current | Cluster validation |

Verify the common local tools:

```sh
docker --version
docker compose version
go version
python --version
uv --version
node --version
corepack pnpm --version
```

## Environment Tiers

| Tier | Purpose | Notes |
|---|---|---|
| Local | Development and fast checks | Uses compose for shared dependencies; application services may run from source until compose wiring is complete. |
| Staging | Integrated validation and evidence | Required for phase exit criteria that need real IAM, OpenFGA, Langfuse/Tempo, Jira/Confluence/GitHub or approvals. |
| Production | Operated platform | Requires completed sign-off, hardened secrets, cloud infra and operational runbooks. |

## Hardware & Sizing

> Last refreshed: 2026-05-09. Owner: Platform Architecture + FinOps. Cost estimates are list-price baselines for `us-central1`, USD, refreshed alongside material resource changes.

### Local tier (developer laptop)

| Dimension | Minimum (full stack) | Recommended | Notes |
|---|---|---|---|
| RAM | 16 GiB | 32 GiB | 16 GiB requires disabling heavy components below |
| vCPU | 4 | 8 | Modern Apple Silicon, recent Intel/AMD |
| Disk free | 40 GiB | 80 GiB | Docker images, Postgres data, model caches |
| Network | Outbound HTTPS | Outbound HTTPS | LiteLLM, GitHub, Artifact Registry pulls |

#### Disable-flags to reduce footprint

If a developer cannot allocate 32 GiB, the following compose services may be disabled:

| Service | How to disable | Impact |
|---|---|---|
| `milvus` | `COMPOSE_PROFILES=base` (omit `rag`) | RAG ingest/query cannot run locally |
| `tempo` | `COMPOSE_PROFILES=base` (omit `tracing`) | No traces in Grafana; logs and metrics still work |
| `loki` | `docker compose -f deploy/compose/docker-compose.yaml stop loki` | No log aggregation; service stdout still available |
| `langfuse` | `docker compose stop langfuse langfuse-db` | No LLM observability; LiteLLM still works |
| `keycloak` | Use `DEV_AUTH_BYPASS=true` | Local auth bypassed; do not use for evidence |

Recommended starter command for a 16 GiB laptop:

```sh
COMPOSE_PROFILES=base make up
```

Cost: $0 (developer hardware).

### Staging tier (shared team environment, GKE)

Default Staging is a single GKE cluster shared across the platform team, sized to support the full reference workflow plus 1-2 reference apps.

#### Per-service Kubernetes requests/limits (Staging defaults)

These are the baseline values consumed by the umbrella chart `forge-platform` at `tier=small`. Per-environment overrides go in `infra/helm/forge-platform/values-staging.yaml`.

| Service | Replicas | CPU req | CPU limit | Mem req | Mem limit |
|---|---:|---:|---:|---:|---:|
| `alfred` | 2 | 200m | 1000m | 512Mi | 1Gi |
| `app-onboarding` | 2 | 100m | 500m | 256Mi | 512Mi |
| `approvals` | 2 | 100m | 500m | 256Mi | 512Mi |
| `asset-observability` | 2 | 100m | 500m | 256Mi | 512Mi |
| `audit` | 2 | 200m | 1000m | 512Mi | 1Gi |
| `control-plane` | 2 | 200m | 1000m | 512Mi | 1Gi |
| `deploy-orchestrator` | 2 | 200m | 1000m | 512Mi | 1Gi |
| `eval-harness-adv` | 1 | 100m | 1000m | 512Mi | 2Gi |
| `finops` | 1 | 100m | 500m | 256Mi | 512Mi |
| `finops-advisor` | 1 | 100m | 500m | 256Mi | 512Mi |
| `iac-drift` | 1 | 100m | 500m | 256Mi | 512Mi |
| `incidents-kb` | 1 | 100m | 500m | 256Mi | 512Mi |
| `marketplace` | 2 | 100m | 500m | 256Mi | 512Mi |
| `mcp` | 2 | 200m | 1000m | 512Mi | 1Gi |
| `openspec` | 2 | 200m | 1000m | 512Mi | 1Gi |
| `permissions` | 2 | 100m | 500m | 256Mi | 512Mi |
| `policy-engine` | 2 | 200m | 1000m | 512Mi | 1Gi |
| `prompt-registry` | 1 | 100m | 500m | 256Mi | 512Mi |
| `runtime-registry` | 2 | 200m | 1000m | 512Mi | 1Gi |
| `scaffolder` | 2 | 100m | 500m | 256Mi | 512Mi |
| `sdlc-orchestrator` | 2 | 200m | 1000m | 512Mi | 1Gi |
| `traceability` | 1 | 100m | 500m | 256Mi | 512Mi |
| `webhooks` | 2 | 100m | 500m | 256Mi | 512Mi |
| `workflow-registry` | 2 | 200m | 1000m | 512Mi | 1Gi |
| `workflow-runtime` | 3 | 200m | 1000m | 512Mi | 1Gi |
| `diagnosis` (worker) | 1 | 100m | 500m | 256Mi | 512Mi |
| `evolution` (worker) | 1 | 100m | 500m | 256Mi | 512Mi |
| `healing-engine` (worker) | 1 | 100m | 500m | 256Mi | 512Mi |
| `incident-detection` (worker) | 1 | 100m | 500m | 256Mi | 512Mi |
| `postmortem` (worker) | 1 | 100m | 500m | 256Mi | 512Mi |
| `rag-ingest` (worker) | 1 | 200m | 1000m | 512Mi | 2Gi |
| `rag-query` (worker) | 1 | 200m | 1000m | 512Mi | 1Gi |

#### Staging shared dependencies

| Dependency | SKU / size | Notes |
|---|---|---|
| GKE | 3 × `e2-standard-4` (4 vCPU, 16 GiB) | Autoscale to 6 nodes |
| Cloud SQL Postgres | `db-custom-2-7680` (2 vCPU, 7.5 GiB), 100 GiB SSD | Single zone, no HA in Staging |
| Memorystore Redis | 1 GiB Basic | Caches and rate-limit counters |
| Milvus | 1 × `e2-standard-4` standalone | RAG vectors only |
| Kafka | 3 brokers, `e2-standard-2`, 50 GiB each | Event bus |
| Loki | 30 GiB persistence, 14-day retention | See `data-retention.md` for classification rules |
| Tempo | 30 GiB persistence, 7-day retention | |
| Prometheus | 30 GiB persistence, 14-day retention | Mimir-compatible later |
| Artifact Registry | 1 region, 100 GiB | Multi-region not required in Staging |

**Indicative monthly cost (Staging, `us-central1`, USD, list price)**: $1,150–$1,400/mo. Volatility caveat: Egress costs vary with traffic; CI/CD pipelines pulling images from outside the region can push this higher.

### Production tier (per-BU dimensioning)

Production sizing is parameterized by BU profile. The umbrella chart exposes `tier=small | medium | large` presets that map to the rows below. Per-service requests/limits scale proportionally.

| BU profile | Apps | GKE node pool | Cloud SQL | Memorystore | Milvus | Kafka | Loki retention | Tempo retention | Indicative $/mo |
|---|---:|---|---|---|---|---|---|---|---:|
| Small (≤10 apps) | ≤10 | 3-6 × `e2-standard-4` | `db-custom-4-15360`, 200 GiB SSD HA | 5 GiB Standard HA | 1 × `e2-standard-4` | 3 × `e2-standard-2` | 30 days | 14 days | $2,000–$2,800 |
| Medium (≤50 apps) | ≤50 | 6-12 × `e2-standard-8` | `db-custom-8-30720`, 500 GiB SSD HA | 16 GiB Standard HA | 3 × `e2-standard-8` | 3 × `e2-standard-4` | 30 days | 14 days | $5,500–$8,000 |
| Large (≤200 apps) | ≤200 | 12-24 × `e2-standard-16` | `db-custom-16-61440`, 1 TiB SSD HA + read replica | 32 GiB Standard HA | 5 × `e2-standard-16` | 5 × `e2-standard-8` | 90 days | 30 days | $14,000–$22,000 |

**Assumptions**: `us-central1`, list pricing, USD. Excludes egress beyond default monthly free tier and excludes LiteLLM model usage. LiteLLM costs are passthrough to the model provider and tracked separately by FinOps.

**Last refresh**: 2026-05-09. Refresh cadence: alongside any service whose resource profile changes materially, and at least quarterly.

### Sizing-to-Helm mapping

The umbrella chart `infra/helm/forge-platform/` exposes tier presets:

```sh
helm install forge-platform infra/helm/forge-platform \
  --values infra/helm/forge-platform/values-prod.yaml \
  --set global.tier=medium
```

Tier presets resolve to per-service replica counts and `requests`/`limits` from this document. The CI check `make sizing-check` verifies the umbrella `values-*.yaml` files match these tables — see [`scripts/check-sizing.py`](../scripts/check-sizing.py) for the diff logic.

### Sizing change procedure

1. Update the row in this document.
2. Update the umbrella values in `infra/helm/forge-platform/values-{local,staging,prod}.yaml`.
3. Run `make sizing-check` locally and ensure CI passes.
4. PR is reviewed by Platform Architecture **and** FinOps owners.
5. Note prior values in the PR description so the change log is reconstructable.

## Phase 0: Foundations

Goal: enable the base platform dependencies, contracts, tenancy, IAM, audit, Registry baseline, LiteLLM gateway, Portal bootstrap and observability foundations.

### Step 0.1: Bootstrap Tools

```sh
make bootstrap
```

### Step 0.2: Start Local Foundations

```sh
make up
make ps
```

This starts the local compose stack defined in `deploy/compose/docker-compose.yaml`. Current shared dependencies include Postgres, Redis, Kafka, Keycloak, OpenFGA, Milvus, LiteLLM, OpenTelemetry Collector, Prometheus, Grafana, Loki and Tempo.

### Step 0.3: Bootstrap IAM And Authorization

Use the runbooks and compose seed files:

| Component | Reference |
|---|---|
| Keycloak realm and users | `deploy/compose/keycloak/forge-realm.json`, `docs/runbooks/keycloak.md` |
| OpenFGA model | `contracts/openfga/authorization-model.json`, `docs/runbooks/openfga.md` |

Expected local Keycloak seed users are documented in `docs/getting-started.md`.

### Step 0.4: Apply Database Migrations

Migrations are stored under `db/migrations/<service>/`. The expected databases include at least:

| Database | Service |
|---|---|
| `forge_control_plane` | Control Plane |
| `forge_registry` | Registry |
| `forge_audit` | Audit |
| service-specific DBs | Phase 1+ services |

If migrations are not wired into compose yet, apply them manually with the migration tool selected for the environment. Record the command here when the migration runner is finalized.

### Step 0.5: Start Application Services

Application services may be started from source locally or deployed through Helm in cluster environments.

| Service | Local target | Source |
|---|---|---|
| Control Plane | `http://localhost:8081` | `services/control-plane/` |
| Registry | `http://localhost:8082` | `services/registry/` |
| Audit | `http://localhost:8083` | `services/audit/` |
| Alfred stub/full Alfred | `http://localhost:8090` or configured service port | `services/alfred/` |
| Portal | `http://localhost:3000` | `portal/` |

As services are wired into compose or Helm, update this table with exact commands and health endpoints.

### Step 0.6: Validate Foundations

```sh
make smoke
```

Also run targeted checks when touching a specific service:

```sh
go test ./...
uv run --extra dev pytest -q
corepack pnpm build
```

Phase 0 sign-off evidence belongs in `docs/governance/phase-0-signoff.md`.

## Phase 1: Agentic Core

Goal: enable Alfred as the control-plane agent with OpenSpec, RAG, policies, approvals, delegated permissions, MCPs, Agent Skills, prompt registry, guardrails, Registry lifecycle enforcement and AI observability.

### Step 1.1: Start Required Services

Phase 1 requires the Phase 0 foundations plus these services:

| Capability | Service/Path |
|---|---|
| Alfred control plane | `services/alfred/` |
| RAG ingest/query | `services/rag-ingest/`, `services/rag-query/` |
| OpenSpec API | `services/openspec/` |
| Policy engine | `services/policy-engine/` |
| Approvals | `services/approvals/` |
| Permissions | `services/permissions/` |
| MCP SDK and servers | `services/mcp/` |
| Prompt Registry | `services/prompt-registry/` |
| Reference Agent Skills | `skills/reference/agent-skills/` |

### Step 1.2: Register MCPs And Agent Skills

The source packages for reference skills follow the Agent Skills format from `agentskills.io`:

```text
skills/reference/agent-skills/<skill-name>/SKILL.md
```

The Registry asset manifest is:

```text
skills/reference/registry-assets.yaml
```

The intended flow is:

1. Read `SKILL.md` metadata and Forge metadata.
2. Publish or update the corresponding Registry asset as `type=skill`.
3. Run deterministic evals.
4. Promote the asset through `proposed -> in_review -> approved` only when eval thresholds pass.
5. Allow Alfred to invoke only approved assets in production-relevant flows.

The seed/import automation is still to be finalized. Until then, use the Registry API or environment-specific seed job and record the command here.

### Step 1.3: Configure Observability

Alfred and LiteLLM must use Langfuse-compatible AI observability with shared `correlation_id`.

Required staging variables for the current integrated evidence test:

| Variable | Meaning |
|---|---|
| `REGISTRY_API_URL` | Base URL for Registry |
| `WORKSPACE_ID` | Existing Workspace UUID with proper OpenFGA permissions |
| `AUTH_TOKEN` | Keycloak JWT for Registry operations |
| `ALFRED_API_URL` | Base URL for Alfred |
| `ALFRED_TOKEN` | Keycloak JWT for Alfred, optional if `AUTH_TOKEN` also works |
| `LANGFUSE_API_URL` or `LANGFUSE_HOST` | Base URL for Langfuse |
| `LANGFUSE_PUBLIC_KEY` and `LANGFUSE_SECRET_KEY` | Preferred Langfuse project credentials |
| `LANGFUSE_API_KEY` | Bearer-token alternative if the deployment uses it |

Do not include the literal `Bearer ` prefix in token variables; tests add it.

### Step 1.4: Run Integrated Evidence Checks

```powershell
.\scripts\integration\run_phase1_integrated_checks.ps1
```

The script runs:

```sh
uv run pytest -q services/registry/tests/test_integration_promotion.py
```

Evidence is written under:

```text
docs/governance/evidence/phase-1/<timestamp>/
```

Use `docs/governance/phase-1-integrated-runbook.md` for the full manual/instrumented validation path.

### Step 1.5: Complete Sign-Off

Update `docs/governance/phase-1-signoff.md` after evidence exists. Do not mark tasks 13.5, 13.6 or 13.8 complete until the staging evidence and SDLC approval are present.

## Phase 2: App Onboarding

Goal: onboard applications into Forge with CI, security scanning, SBOM, image signing and registry publication.

Source of truth: `openspec/changes/archive/2026-05-09-phase-2-app-onboarding/`. Sign-off: [`docs/governance/phase-2-signoff.md`](governance/phase-2-signoff.md).

### Step 2.1: Register the GitHub App

1. Owner of the customer GitHub Org creates a GitHub App with the permissions listed in [`docs/runbooks/github-app.md`](runbooks/github-app.md).
2. Record the App ID, slug and installation ID in the secret manager: `forge/github-app/{client_id, client_secret, private_key, app_id, installation_id}`.
3. Configure `services/control-plane` env: `GITHUB_APP_ID`, `GITHUB_APP_INSTALLATION_ID`, `GITHUB_APP_PRIVATE_KEY_PATH` (mounted from secret).

Evidence to capture: registered GitHub App URL, installation ID, screenshot of permissions panel, archived to `docs/governance/evidence/phase-2/`.

### Step 2.2: Enable the Reusable CI Workflow

The platform ships a reusable CI workflow that scaffolded repos consume:

```yaml
# .github/workflows/ci.yml in a scaffolded repo
jobs:
  forge-ci:
    uses: forge-eng-fabric/forge-actions/.github/workflows/forge-ci.yml@v1
    with:
      service: ${{ github.event.repository.name }}
      sbom: true
      cosign: true
      trivy: true
```

The reusable workflow runs lint → build → unit tests → SAST (CodeQL) → SCA (Dependabot/OSV) → SBOM (Syft) → container scan (Trivy) → image sign (Cosign keyless) → push to Artifact Registry. Each step writes evidence as a workflow artifact.

### Step 2.3: Provision Artifact Registry

Use the Terraform module `infra/terraform/modules/artifact-registry/`:

```sh
cd infra/terraform/modules/artifact-registry
terraform init
terraform apply -var "project_id=$GCP_PROJECT" -var "location=us-central1" -var "repo_id=forge-platform"
```

Outputs include the repo URL, IAM policy bindings for the runtime SAs, and the immutable retention policy for prod images.

### Step 2.4: Validate

```sh
# 1. Run the reusable CI on a scaffolded repo
gh workflow run ci.yml --repo <org>/<scaffolded-repo>

# 2. Verify the signed image
cosign verify <region>-docker.pkg.dev/<project>/forge-platform/<image>:<digest> \
  --certificate-identity-regexp '.*<org>/<repo>.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com'
```

Phase 2 sign-off requires: a registered GitHub App, the reusable CI workflow used by ≥ 1 scaffolded repo, SBOM/Cosign/Trivy evidence for ≥ 1 image, an Artifact Registry record. Tag the merge commit with `phase-2-signoff-<YYYYMMDD>` once approvers have signed.

## Phase 3: Deployable Apps

Goal: provision runtimes and deploy user applications to GKE, Cloud Run or local/minikube targets with preflight checks and image verification.

Source of truth: `openspec/changes/archive/2026-05-09-phase-3-deployable-apps/`. Sign-off: [`docs/governance/phase-3-signoff.md`](governance/phase-3-signoff.md).

### Step 3.1: Apply Terraform Modules

The Phase 3 module set lives at `infra/terraform/modules/`. Each module has a README with required inputs and example invocation:

| Module | Purpose |
|---|---|
| [`gke-cluster`](../infra/terraform/modules/gke-cluster/README.md) | GKE Autopilot or Standard cluster with Workload Identity |
| [`cloud-run-service`](../infra/terraform/modules/cloud-run-service/README.md) | Cloud Run service with custom SA + ingress controls |
| [`cloud-sql`](../infra/terraform/modules/cloud-sql/README.md) | Cloud SQL Postgres with private IP and CMEK |
| [`memorystore`](../infra/terraform/modules/memorystore/README.md) | Memorystore Redis with private VPC access |
| [`artifact-registry`](../infra/terraform/modules/artifact-registry/README.md) | Artifact Registry repo with retention and IAM bindings |
| [`iam-delegated-permissions`](../infra/terraform/modules/iam-delegated-permissions/README.md) | Delegated SAs for federated runtimes |

Apply order: `gke-cluster` → `iam-delegated-permissions` → `artifact-registry` → `cloud-sql` / `memorystore` → `cloud-run-service`.

### Step 3.2: Federated Project Setup

For each Tenant project:

```sh
# 1. Bootstrap the Terraform state backend
terraform -chdir=infra/terraform/bootstrap apply -var "tenant_id=<tenant>"

# 2. Apply the federated IAM module
terraform -chdir=infra/terraform/modules/iam-delegated-permissions apply -var "tenant_id=<tenant>"
```

The federated IAM grants the Forge platform SA the minimum scopes listed in [`runtime-connectors`](../openspec/specs/runtime-connectors/spec.md) — never `roles/owner`, never `roles/editor`.

### Step 3.3: Register the Runtime in `runtime-registry`

```sh
curl -X POST http://localhost:8110/v1/runtimes \
  -H 'content-type: application/json' \
  -d '{
    "workspace_id": "<ws>",
    "tenant_id": "<tenant>",
    "type": "gke",
    "mode": "byo",
    "kubeconfig": "<base64-encoded>",
    "name": "pilot-prod-gke"
  }'
```

### Step 3.4: Run Preflight and Verify

```sh
# Preflight (existing endpoint)
curl -X POST http://localhost:8110/v1/runtimes/<id>/preflight

# Verify (new — produces a structured report)
make verify-runtime RUNTIME=<id> WORKSPACE=<ws>
```

The verify command exits non-zero on any `fail` and prints remediation hints.

### Step 3.5: Image-Verification-at-Deploy

Deploy actions verify the image's Cosign signature against the workflow's expected identity before applying the runtime change. The verification step is implemented in `services/deploy-orchestrator` and is non-overridable in production.

Phase 3 sign-off requires: ≥ 1 BYO and ≥ 1 Provisioned runtime onboarded, successful `verify-runtime` reports for both, image-verification-at-deploy evidence for ≥ 1 deployment.

## Phase 4: SDLC Orchestration

Goal: expose SDLC capabilities for product, architecture, design, development, QA, security, DevOps, SRE and FinOps as governed skills and workflows.

Source of truth: `openspec/changes/archive/2026-05-09-phase-4-sdlc-orchestration/`. Sign-off: [`docs/governance/phase-4-signoff.md`](governance/phase-4-signoff.md).

### Step 4.1: Register SDLC Skills

The reference skills under `skills/reference/agent-skills/` cover each SDLC capability. Register each via the Registry API or the seed automation:

```sh
make seed-registry  # idempotent — skips assets already at the same version
```

Each Skill carries a per-capability policy binding. Eval thresholds are documented in `docs/eval-harness/` and enforced at promotion to `approved`.

### Step 4.2: Bind Capabilities to Policies

Each Workspace gets a default policy bundle that maps capabilities to autonomy modes. Override via `services/policy-engine` API or the Portal's Permissions page.

Capability examples:

| Capability | Default mode | Approver role |
|---|---|---|
| `sdlc-product` | `autonomous` | — |
| `sdlc-architecture` | `requires_approval` | architecture-lead |
| `sdlc-devops:deploy:prod` | `requires_dual_control` | sre + workspace-admin |
| `sdlc-security` | `requires_approval` | security-lead |

### Step 4.3: Seed Prompt Templates

```sh
curl -X POST http://localhost:8124/v1/templates -d @skills/reference/prompt-templates/seed.json
```

### Step 4.4: Validate

Run the reference workflow end-to-end:

```sh
make demo-intent-to-deploy
```

Phase 4 sign-off requires: registered Skills with eval reports per capability, capability-bound policies, prompt templates seeded, ≥ 1 successful run of `forge.reference.intent-to-deploy@1`.

## Phase 5: Workflow Marketplace

Goal: enable governed workflows, marketplace installation, workflow versioning, advanced evals and per-asset observability.

Source of truth: `openspec/changes/archive/2026-05-09-phase-5-workflow-marketplace/`. Sign-off: [`docs/governance/phase-5-signoff.md`](governance/phase-5-signoff.md).

### Step 5.1: Durable Workflow Runtime

The default runtime is `workflow-runtime` backed by Postgres (event store). For production at scale, consider Temporal — that decision is tracked in a follow-up ADR (see `openspec/changes/` for any active proposals).

```sh
helm install forge-platform infra/helm/forge-platform \
  -f infra/helm/forge-platform/values-staging.yaml \
  --set workflow-runtime.replicaCount=3
```

### Step 5.2: Marketplace

The marketplace surfaces installable workflow templates. The internal seed includes:

- `forge.reference.intent-to-deploy@1.0.0` (tag: `reference,forge`)
- additional reference workflows as they ship

Install a workflow into a Workspace:

```sh
curl -X POST http://localhost:8118/v1/installations \
  -d '{"workspace_id": "<ws>", "workflow_id": "forge.reference.intent-to-deploy", "version": "1.0.0"}'
```

### Step 5.3: Advanced Eval Harness

`services/eval-harness-adv` runs per-workflow evaluations on each new version. Configure pass thresholds in the workflow metadata; the registry rejects publishes below threshold.

Phase 5 sign-off requires: a long-lived workflow execution record (≥ 1 hour wall-clock), ≥ 1 marketplace installation in a Workspace, ≥ 1 advanced eval-harness run with pass/fail evidence.

## Phase 6: Autonomous Ops

Goal: enable autonomous operations, healing actions, incident response and governed remediation loops.

Source of truth: `openspec/changes/archive/2026-05-09-phase-6-autonomous-ops/`. Sign-off: [`docs/governance/phase-6-signoff.md`](governance/phase-6-signoff.md).

### Step 6.1: Healing Actions Catalog

Healing actions are registered in `services/healing-engine`. Each action specifies:

- Reversibility classification (`reversible`, `irreversible`)
- Pre-condition probes
- Post-condition assertions
- Approval requirement (per autonomy preset)

Seed the catalog:

```sh
curl -X POST http://localhost:8129/v1/actions -d @docs/healing/catalog-seed.json
```

### Step 6.2: Simulated Remediation

Remediation runs in a sandboxed namespace before applying to production. The sandbox runs the same NetworkPolicy and OpenFGA scopes as the target Workspace, so policy violations surface in simulation.

```sh
curl -X POST http://localhost:8129/v1/incidents/<id>/simulate-remediation
```

### Step 6.3: Evolution Loop

The evolution loop captures simulated and applied remediations as candidate OpenSpec changes (`source: autonomous-loop`). Reviewers approve or reject in the [Evolution Inbox](../portal/src/app/evolution/page.tsx).

Phase 6 sign-off requires: healing action catalog populated, ≥ 1 simulated remediation under guardrails, ≥ 1 evolution-loop record proposing an OpenSpec update.

## New Phases: `iac` and `observability`

Two new SDLC phases were introduced after Phase 4 and are opt-in by default for all tenants. Feature flags control their availability per tenant before global rollout.

### New Phases Summary

| Phase | Position in order | Default target policy | Description |
|---|---|---|---|
| `iac` | Between `devops` and `sre` | `opt-in` | Infrastructure-as-Code generation and drift reconciliation |
| `observability` | After `finops` (end of pipeline) | `opt-in` | Observability harness provisioning, SLO definition, dashboard seeding |

### Full Phase Targets Matrix

The table below shows the default target policy for every phase as returned by `DefaultTargets()` in `services/sdlc-orchestrator/internal/sdlc/types.go`. Policies are applied at workflow-plan creation time and can be overridden per-app or per-spec run.

| Phase | Default target policy | Notes |
|---|---|---|
| `product` | not configured (skipped if absent) | Managed by the product owner outside the SDLC orchestrator |
| `architecture` | `required` | Gate failure blocks progression |
| `design` | `optional` | Gate failure emits a warning; progression continues |
| `development` | `required` | Gate failure blocks progression |
| `qa` | `required` | Gate failure blocks progression |
| `security` | `required` | Gate failure blocks progression |
| `devops` | `required` | Gate failure blocks progression |
| `iac` | `opt-in` | Phase runs only when explicitly requested; must be enabled via feature flag `forge.sdlc.iac.enabled` |
| `sre` | `optional` | Gate failure emits a warning; progression continues |
| `finops` | `opt-in` | Phase runs only when explicitly requested |
| `observability` | `opt-in` | Phase runs only when explicitly requested; must be enabled via feature flag |

### Enabling opt-in phases per tenant

```bash
# Enable iac phase for a tenant
curl -s -X PATCH http://localhost:8082/v1/tenants/{tenant_id}/feature-flags \
  -H "content-type: application/json" \
  -d '{"forge.sdlc.iac.enabled": true}'

# Enable observability phase for a tenant
curl -s -X PATCH http://localhost:8082/v1/tenants/{tenant_id}/feature-flags \
  -H "content-type: application/json" \
  -d '{"forge.sdlc.observability.enabled": true}'

# Verify
curl http://localhost:8082/v1/tenants/{tenant_id}/feature-flags | jq .
```

The control-plane serves these endpoints at `:8082`. See the per-tenant rollout sequence in `docs/runbooks/sdlc-phase-rollout.md`.

## Alfred Console Redesign (cross-phase capability)

Source of truth: `openspec/changes/alfred-console-redesign/`. This change ships across all phases because the console is the primary operator surface.

### Friendly and Advanced views

The Alfred console (`/alfred`) now resolves one of two views per user session:

| View | Target audience | Default for |
|---|---|---|
| Friendly | Product owners, PMs, business stakeholders | `workspace.member` role on first sign-in |
| Advanced | Engineers and platform operators | `workspace.developer` and above on first sign-in |

The resolver runs in order: explicit `?view=` query param → `user.console_view_preference` (persisted) → role-based default from `resolveAndPersistDefault()`.

#### Role-based default rules

| OpenFGA role | Resolved default |
|---|---|
| `workspace:member` only | Friendly |
| `workspace:developer` | Advanced |
| `workspace:admin` | Advanced |
| `workspace:owner` | Advanced |
| No workspace membership found | Friendly (safe fallback) |

The resolved default is persisted via `PUT /api/user/preferences` (`console_view_preference: "friendly" | "advanced"`) so the resolver runs only once per user.

#### Tenant-level default override

Set `tenant.console_default_view` in the tenant config record to override the role-based default for all new users in that tenant:

```sh
# Example: force Friendly for a non-technical tenant
curl -X PATCH http://localhost:8081/v1/tenants/<tenant_id>/config \
  -H 'content-type: application/json' \
  -d '{"console_default_view": "friendly"}'
```

Valid values: `"friendly"`, `"advanced"`. The tenant override takes precedence over the role-based rule but is overridden by any explicit user preference already persisted.

#### Feature flag

The console v2 is gated by the `ALFRED_CONSOLE_V2_ENABLED` environment variable on the Alfred service (`services/alfred/alfred/config.py`) and the per-tenant `forge.alfred_console_v2.enabled` flag. Both must be enabled for a tenant to access the new views.

```sh
# Alfred service env
ALFRED_CONSOLE_V2_ENABLED=true

# Per-tenant flag (via control-plane API)
curl -X PUT http://localhost:8081/v1/tenants/<tenant_id>/flags/forge.alfred_console_v2.enabled \
  -d '{"value": true}'
```

See the tenant rollout runbook at `docs/runbooks/alfred-console-rollout.md`.

### Spec deduplication (RAG match)

When a user submits an intent, Alfred runs a Milvus retrieval pass before creating a draft. If a candidate spec scores above the tenant threshold (default 0.80, floor 0.65), the UI shows the match dialog.

| Config key | Default | Description |
|---|---|---|
| `SPEC_MATCH_THRESHOLD_DEFAULT` | `0.80` | Score at or above which the match dialog fires |
| `SPEC_MATCH_THRESHOLD_FLOOR` | `0.65` | Minimum allowed threshold; writes below this return `422 threshold_below_floor` |
| `DEDUP_INDEX_URL` | `http://localhost:8086` | Milvus/index service URL |

The dedup index is kept current by `services/alfred/alfred/dedup_indexer.py` which reacts to `spec.purged.v1`, `spec.reparented.v1`, and `intent.committed.v1` events.

### `/forge` command (replaces `/openspec`)

The canonical command is now `/forge` in all surfaces (CLI, Portal palette, docs). `/openspec` is kept as a deprecated alias for two minor versions and emits `alfred.command.deprecated_alias.v1` on every invocation.

| Surface | Canonical | Deprecated alias |
|---|---|---|
| Portal command palette | `/forge new`, `/forge list` | `/openspec new` (shows yellow toast) |
| CLI | `forge new`, `forge list`, … | `openspec …` (prints stderr warning) |
| Docs | `/forge` | — |

Removal of the `/openspec` alias is tracked in the release calendar. Tenants that need more time can set `force_keep_openspec_alias: true` — see the runbook at `docs/runbooks/alfred-openspec-alias-exception.md`.

### Dashboards

Import `docs/dashboards/alfred-console-v2.json` into Grafana. The board contains:

| Panel | Source metric |
|---|---|
| Friendly vs Advanced ratio | `alfred_console_view_total` by `view` label |
| Match-found rate | `alfred.intent.match_found.v1` / total intents |
| Match dismissed rate | `alfred.intent.match_dismissed.v1` / match-found |
| False-positive ratio | manual survey input (monthly) |
| `/openspec` alias volume | `alfred.command.deprecated_alias.v1` count by tenant |

### SLOs

| SLO | Target | Measurement |
|---|---|---|
| Dedup retrieval p95 latency | < 100 ms | `/v1/intent/match` response time, Tempo traces |
| Friendly view first-paint p95 | < 1 s | Portal RUM (Grafana Faro or equivalent) |

Both SLOs are checked by the Grafana alerting rules in `infra/grafana/alerts/alfred-console-v2.yaml`.

### Agent-mode `start_step`

Agent-mode sessions now accept `start_step` in `POST /v1/agent-mode/sessions` to jump directly to a specific SDLC phase without running the discovery/architect ramp-up:

| Value | Requires spec lifecycle state |
|---|---|
| `discovery` | any |
| `architect` | `approved` or `committed` |
| `design`, `test`, `iac`, `deploy` | any |

If the spec is not ready for the requested step, the API returns `409 spec_not_ready_for_architect`.

## Active Registry Gateways (cross-phase capability)

Status: rolling out across releases N → N+3. Spec: [openspec/changes/active-registry-gateways](../openspec/changes/active-registry-gateways/). Operator guide: [docs/platform/active-registry-gateways.md](platform/active-registry-gateways.md).

Three platform capabilities ship together; each registers an active runtime gateway in front of an asset family that previously had only a catalog. The Asset Registry gains `how_to_json` and `active_surface_json` on every row; the lifecycle gate for `approved` requires both blocks to be populated.

| Capability | Service / Package | Default port | New env vars |
|---|---|---|---|
| `mcp-gateway` | [services/mcp-gateway](../services/mcp-gateway/) | 8092 | `REGISTRY_URL`, `POLICY_ENGINE_URL`, `BUDGET_URL`, `REDIS_ADDR`, `IDENTITY_ROTATION`, `SSE_BUFFER_SIZE` |
| `a2a-gateway` | [services/a2a-gateway](../services/a2a-gateway/) | 8093 | same envs as mcp-gateway |
| `skill-artifact-store` (adapter) | [pkg/artifact-store-adapter](../pkg/artifact-store-adapter/) — N drivers (Nexus, Artifactory, GitHub Packages, CodeArtifact) | n/a (library) | per-binding configured in `artifact_store_binding` |

Runtime ingress flips through a per-Tenant `gateway.enforced` flag, defaulting `false` for compatibility through Release N+2 and `true` from Release N+3. Operators read the per-release rollout in [docs/platform/active-registry-gateways-rollout.md](platform/active-registry-gateways-rollout.md) and the per-task runbooks under [docs/runbooks/active-registry-gateways/](runbooks/active-registry-gateways/).

Migrations: registry [0007_active_registry_gateways.sql](../db/migrations/registry/0007_active_registry_gateways.sql) introduces `how_to_json`, `active_surface_json`, `external_provenance` and the three side-tables (`external_mcp_endpoint`, `external_a2a_agent`, `artifact_store_binding`).

OpenFGA: new principal types `gateway_caller` and `external_partner` in [contracts/openfga/authorization-model.json](../contracts/openfga/authorization-model.json).

Policy bundles: [policies/gateway/mcp.rego](../policies/gateway/mcp.rego) and [policies/gateway/a2a.rego](../policies/gateway/a2a.rego).

## Standard Validation Matrix

Run the relevant subset for every change:

| Area | Command |
|---|---|
| Go services | `go test ./...` from the service directory |
| Python services | `uv run --extra dev pytest -q` from the package directory |
| Python lint | `uv run --extra dev ruff check .` from the package directory |
| Portal | `corepack pnpm build` from `portal/` |
| Registry integration evidence | `uv run pytest -q services/registry/tests/test_integration_promotion.py` |
| OpenSpec status | `openspec status --change "<change>" --json` |
| OpenSpec apply instructions | `openspec instructions apply --change "<change>" --json` |

## Standard Evidence Bundle

Every phase exit should produce:

| Evidence | Location |
|---|---|
| Test output | CI job logs or local command output |
| API responses | `docs/governance/evidence/<phase>/<timestamp>/` |
| Audit events | JSON export with `correlation_id` |
| Langfuse traces | Trace IDs or exported JSON |
| Tempo traces | Trace IDs or screenshots/exports |
| Approval records | Approvals API export or Portal evidence |
| Sign-off | `docs/governance/<phase>-signoff.md` |

## Update Checklist For Future Specs

When implementing or changing a phase, update this guide with:

1. New service names, ports and health endpoints.
2. New environment variables and secret sources.
3. New database names and migration commands.
4. New seed/import commands.
5. New Registry asset manifests or package formats.
6. New validation commands.
7. New evidence locations.
8. New sign-off requirements.

Keep this document operational and factual. If a step is not automated yet, mark it as pending and point to the exact file or OpenSpec task that will complete it.
