# Getting Started — Forge Engineering Fabric (Local Slice)

This is the **local-first slice** of Phase 0. It runs entirely on your laptop with Docker Compose. Cloud (GKE, Cloud SQL, Memorystore, Artifact Registry, etc.) is out of scope here and tracked as remaining tasks in `openspec/changes/phase-0-foundations/tasks.md`.

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Docker Desktop | latest | Compose v2 enabled |
| Go | ≥ 1.22 | for building services locally |
| Node.js | ≥ 20 | + `pnpm` (`npm i -g pnpm`) |
| Python | ≥ 3.12 | + `uv` recommended (`pipx install uv`) |
| make | any | optional but recommended |

Verify:

```sh
docker --version
go version
node -v && pnpm -v
python --version
```

## Bring up the stack

```sh
make up
# or:
docker compose -f deploy/compose/docker-compose.yaml up -d
```

The first run downloads images for: Postgres, Redis, Kafka (KRaft), Keycloak, OpenFGA, Milvus, LiteLLM, OpenTelemetry collector, Prometheus, Grafana, Loki, Tempo. Expect 5–10 minutes on a fresh machine.

## What runs locally

| Service | URL | Notes |
|---|---|---|
| Portal (Next.js) | http://localhost:3000 | login via Keycloak |
| Control Plane API | http://localhost:8081 | Workspace CRUD |
| Registry API | http://localhost:8082 | Asset CRUD |
| Audit API | http://localhost:8083 | read-only query |
| Alfred stub (FastAPI) | http://localhost:8090 | demo agent |
| Keycloak | http://localhost:8080 | admin/admin |
| OpenFGA | http://localhost:8088 | playground at /playground |
| Grafana | http://localhost:3001 | admin/admin |
| Prometheus | http://localhost:9090 | |
| LiteLLM | http://localhost:4000 | |

## First-run smoke test

```sh
make smoke
```

The script:
1. Waits for Keycloak / Postgres / Kafka / OpenFGA to be healthy.
2. Bootstraps the OpenFGA model from `contracts/openfga/authorization-model.json`.
3. Logs in to Keycloak as the seed user.
4. Creates a Tenant, BU and Workspace via the Control Plane API.
5. Registers an asset in the Registry.
6. Calls LiteLLM via the Go SDK from the Alfred stub.
7. Verifies that audit entries exist for each step.

## Tearing down

```sh
make down
# or to also wipe volumes:
docker compose -f deploy/compose/docker-compose.yaml down -v
```

## Repo layout

```
services/
  control-plane/   # Go — Tenant/BU/Workspace CRUD
  registry/        # Go — Asset CRUD
  audit/           # Go — append-only audit consumer + query API
  alfred/          # Python (FastAPI) — minimal agent stub
portal/            # Next.js 14 — login + workspaces list
pkg/
  llmclient/       # Go SDK for LiteLLM
contracts/
  events/          # CloudEvents v1.0 schemas
  openapi/         # OpenAPI specs for Control Plane / Registry
  openfga/         # OpenFGA authorization model + tests
db/migrations/     # SQL migrations (goose-style)
deploy/compose/    # docker-compose stack + seed configs
infra/             # Helm + Terraform — placeholders for now
docs/              # policies, runbooks, governance
openspec/          # OpenSpec changes/specs
```

## What is NOT yet implemented

See `openspec/changes/phase-0-foundations/tasks.md` for the complete list. The local slice deliberately stops at the boundary where credentials, cloud accounts or organizational decisions are required:

- No GKE / Cloud SQL / Memorystore / Artifact Registry.
- No Helm charts for the platform services (only docker-compose).
- No Terraform modules for GCP base infra.
- No real GitHub App registration (manifest documented; install flow stubbed).
- No NetworkPolicies / GCP egress firewall — the equivalent boundary in compose is documented but not enforced beyond network isolation.
- No partitioning of `audit_event` and no GCS archival job.
- No multi-language SDK codegen (Go only for now).
- No Loki/Tempo dashboards — Grafana ships empty.

These are tracked as open tasks for follow-up changes.
