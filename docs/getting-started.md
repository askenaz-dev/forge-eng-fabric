# Getting Started — Forge Engineering Fabric (Local Slice)

This is the **local-first slice** of Phase 0. It runs entirely on your laptop with Docker Compose. Cloud (GKE, Cloud SQL, Memorystore, Artifact Registry, etc.) is out of scope here and tracked as remaining tasks in `openspec/changes/phase-0-foundations/tasks.md`.

For the cross-phase platform enablement path, use `docs/platform-enablement.md` as the canonical living guide. This page stays focused on the Phase 0 local slice.

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
| Portal (Next.js) | http://localhost:3000 | login with `alice/alice` or `bob/bob` |
| Control Plane API | http://localhost:8081 | Workspace CRUD |
| Registry API | http://localhost:8082 | Asset CRUD |
| Audit API | http://localhost:8083 | read-only query |
| App Onboarding API | http://localhost:8085 | templates and onboarding requests |
| Alfred (FastAPI) | http://localhost:8090 | control-plane agent |
| Keycloak admin | http://localhost:8080/admin | `admin/admin` for the master realm only |
| OpenFGA | http://localhost:8088 | playground at /playground |
| Grafana | http://localhost:3001 | admin/admin |
| Prometheus | http://localhost:9090 | |
| LiteLLM | http://localhost:4000 | UI: `admin` / `sk-forge-local`; API key: `sk-forge-local` |

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
6. Verifies Alfred and LiteLLM local gateway readiness.
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
  alfred/          # Python (FastAPI) — control-plane agent
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

## Portal — local UI

The Portal ships the Forge Engineering Fabric brand and is built with Next.js
14, NextAuth (Keycloak provider) and a CSS-variable design system. To run it
against the local compose stack:

```sh
make up                       # bring up the platform services
cd portal && pnpm install     # one-off
PORTAL_REBRAND=1 pnpm dev     # start the Portal on :3000
```

Top-bar affordances:

- **⌘K (macOS) / Ctrl K (other)** — open the global command palette. Search
  agents, runs, skills, specs; toggle theme / density / language; switch
  workspace; sign out.
- **ES / EN pill** — toggle locale (default Spanish). Persists in
  `localStorage` and `POST /api/i18n/preference`.
- **Theme menu** — `Claro / Oscuro / Sistema`. System follows
  `prefers-color-scheme`.
- **Notifications bell** — subscribes to `/api/notifications/stream` (SSE
  proxy to `audit-stream`).

See [docs/portal/design-system.md](portal/design-system.md) for the token
reference, accessibility contract and component inventory.
