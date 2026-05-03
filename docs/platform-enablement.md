# Platform Enablement Guide

This is the living step-by-step guide for enabling Forge Engineering Fabric across all phases. Keep this document current as OpenSpec changes are implemented from Phase 0 through later phases.

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

Current source of truth:

```text
openspec/changes/phase-2-app-onboarding/
```

Enablement steps to add when implementation starts:

1. Configure GitHub App installation and repository selection.
2. Enable reusable CI workflows for lint, build, unit tests, SAST, SCA, SBOM, container scanning and signing.
3. Configure Artifact Registry or local registry target.
4. Record required secrets and OIDC trust setup.
5. Add onboarding validation commands and evidence path.

## Phase 3: Deployable Apps

Goal: provision runtimes and deploy user applications to GKE, Cloud Run or local/minikube targets with preflight checks and image verification.

Current source of truth:

```text
openspec/changes/phase-3-deployable-apps/
```

Enablement steps to add when implementation starts:

1. Apply Terraform modules for cloud runtime foundations.
2. Configure runtime registry and connectors.
3. Run runtime preflight checks.
4. Validate signed image digest and deployment release history.
5. Record rollback and evidence collection commands.

## Phase 4: SDLC Orchestration

Goal: expose SDLC capabilities for product, architecture, design, development, QA, security, DevOps, SRE and FinOps as governed skills and workflows.

Current source of truth:

```text
openspec/changes/phase-4-sdlc-orchestration/
```

Enablement steps to add when implementation starts:

1. Register SDLC capability skills in Registry.
2. Configure per-capability policies, approvers and delegated permissions.
3. Seed OpenSpec templates and prompt templates.
4. Validate orchestration traces and audit records per capability.

## Phase 5: Workflow Marketplace

Goal: enable governed workflows, marketplace installation, workflow versioning, advanced evals and per-asset observability.

Current source of truth:

```text
openspec/changes/phase-5-workflow-marketplace/
```

Enablement steps to add when implementation starts:

1. Start workflow runtime and workflow registry.
2. Register workflow assets and versions.
3. Configure marketplace installation scopes.
4. Run advanced eval harness and publication gates.
5. Validate observability tabs and workflow invocation audit.

## Phase 6: Autonomous Ops

Goal: enable autonomous operations, healing actions, incident response and governed remediation loops.

Current source of truth:

```text
openspec/changes/phase-6-autonomous-ops/
```

Enablement steps to add when implementation starts:

1. Register healing action catalog assets.
2. Configure reversible action policies and high-criticality approvals.
3. Seed incident/audit/RAG data sources.
4. Validate self-healing simulations with strict guardrails.
5. Record SRE/Security sign-off evidence.

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
