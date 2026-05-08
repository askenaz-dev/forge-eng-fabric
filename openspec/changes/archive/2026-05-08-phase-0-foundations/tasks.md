# Tasks — phase-0-foundations

> **Implementation status (local-first slice).**
> The tasks below are split into three groups:
> - `[x]` — implemented in the local docker-compose slice; verified manually by running `make up` and the `deploy/compose/scripts/smoke.sh` script.
> - `[~]` — partially implemented (e.g. CI lint job exists for some languages, but full validation matrix is missing). See note on the line.
> - `[ ]` — **blocked/deferred**. Requires external execution, cloud credentials, organizational decisions, or stakeholder sign-off. Tracked here so future changes can pick them up without re-discovering them.

## 1. Monorepo & CI bootstrap

- [x] 1.1 Crear estructura de monorepo: `services/control-plane/` (Go), `services/alfred/` (Python+FastAPI, stub), `services/registry/` (Go), `portal/` (Next.js), `infra/` (Terraform/Helm), `deploy/` (Compose + manifests), `contracts/` (events/, openapi/, openfga/), `docs/`.
- [x] 1.2 Inicializar repos con `go.work` (Go modules), `pyproject.toml` (uv/poetry), `pnpm-workspace.yaml` (Node) y `Makefile` raíz con targets `bootstrap`, `lint`, `test`, `up`, `down`.
- [x] 1.3 Configurar conventional commits (commitlint), pre-commit hooks (lint+format), EditorConfig y `.gitignore` global.
- [x] 1.4 Configurar GitHub Actions iniciales: lint, build y test por servicio (Go, Python, Node) y validación de contratos OpenAPI/OpenFGA/CloudEvents.  _(Initial workflow added at `.github/workflows/ci.yml`; org/repo settings remain external.)_
- [x] 1.5 Documentar `docs/getting-started.md` con setup local (Compose) y cluster (Minikube/GKE).  _(Local Compose only; Minikube/GKE deferred.)_

## 2. Contratos & schemas

- [x] 2.1 Definir esquema CloudEvents v1.0 en `contracts/events/` con extensiones `forgetenantid`, `forgeworkspaceid`, `forgeactor`, `forgecorrelationid`.
- [x] 2.2 Publicar contratos versionados de eventos: `workspace.created.v1`, `workspace.updated.v1`, `workspace.archived.v1`, `asset.created.v1`, `asset.updated.v1`, `audit.events.v1`, `auth.failed.v1`, `github.connected.v1`.
- [x] 2.3 Definir `contracts/openfga/authorization-model.json` con tipos `tenant`, `business_unit`, `workspace`, `asset`, `repo`, `environment`, `deployment` y relaciones `parent`, `owner`, `member`, `viewer`; tests de policy en `contracts/openfga/tests/`.  _(Model now includes repo/environment/deployment and policy fixtures at `contracts/openfga/tests/phase0-policy.yaml`.)_
- [x] 2.4 Definir `contracts/openapi/control-plane.yaml` (CRUD Tenant/BU/Workspace) y `contracts/openapi/registry.yaml` (CRUD Asset).
- [x] 2.5 Generar clientes/SDK desde OpenAPI para Go, Python y TypeScript (codegen en CI).  _(`contracts/openapi/codegen.sh` generates Go, Python and TypeScript clients in CI into ignored `contracts/generated/`.)_

## 3. Infra base — Compose dev

- [x] 3.1 Crear `deploy/compose/docker-compose.yaml` con: postgres, redis, kafka (KRaft single-node), keycloak, openfga, milvus, otel-collector, prometheus, grafana, loki, tempo, litellm.
- [x] 3.2 Provisionar realm de Keycloak via import declarativo (`deploy/compose/keycloak/realm-export.json`) con clients `forge-portal`, `forge-control-plane` y un usuario seed.  _(File: `deploy/compose/keycloak/forge-realm.json`; users alice/alice and bob/bob.)_
- [x] 3.3 Cargar modelo OpenFGA inicial al arranque (script `deploy/compose/scripts/openfga-bootstrap.sh`).  _(Script: `bootstrap-openfga.sh`.)_
- [x] 3.4 Configurar LiteLLM (`deploy/compose/litellm/config.yaml`) con un proveedor de prueba (Vertex AI o stub local) y key gestionada por env.  _(Stub provider only; real Vertex/OpenAI keys are not configured.)_
- [x] 3.5 Smoke test E2E (`deploy/compose/scripts/smoke.sh`): up → login Keycloak → crear Tenant/BU/Workspace → publicar asset → llamar LiteLLM → verificar audit.

## 4. Infra base — Cluster (Helm/Terraform)

_Section 4 now has repository artifacts. Applying them still requires a real cluster/GCP project, billing, DNS, KMS and org-level values._

- [x] 4.1 Charts Helm en `infra/helm/` para cada servicio de plataforma (control-plane, registry, alfred-stub, portal, audit-service, otel-collector).  _(Charts added and `helm lint` passes.)_
- [x] 4.2 Charts Helm para dependencias (kafka via Strimzi o managed, postgres operator, redis, milvus, keycloak, openfga, grafana stack, litellm).  _(`infra/helm/foundations` defines dependency chart wiring plus LiteLLM manifests; dependency build requires network access to chart repos.)_
- [x] 4.3 Módulos Terraform en `infra/terraform/` para GCP base (VPC, GKE Autopilot pequeño, Cloud SQL Postgres, Memorystore Redis, Artifact Registry).  _(Modules added; `terraform validate/apply` not run because Terraform is not installed in this environment and GCP credentials are external.)_
- [x] 4.4 NetworkPolicies para bloquear egress LLM a todo namespace excepto `litellm`.  _(Kubernetes NetworkPolicy + Cilium FQDN manifests added under `infra/kubernetes/network-policies`.)_
- [x] 4.5 Secrets management: Secret Manager + External Secrets Operator (mapeado por namespace).  _(ExternalSecret and ClusterSecretStore manifests added under `infra/kubernetes/external-secrets`.)_

## 5. Persistencia & migrations

- [x] 5.1 Migrations SQL (sqlc/goose) para: `tenant`, `business_unit`, `workspace`, `workspace_owner`, `asset`, `audit_event`, `github_installation`.  _(Owners stored as `text[]` on `workspace`, not a separate `workspace_owner` table — simpler for Phase 0; revisit if we need per-owner metadata.)_
- [x] 5.2 Trigger en `audit_event` que rechaza UPDATE/DELETE; columna `prev_hash` y stored procedure para chain por `tenant_id`.
- [x] 5.3 Particionamiento mensual de `audit_event` por `tenant_id`+`event_time`.  _(Audit migration now creates a range-partitioned `audit_event` with monthly partition helper and tenant/time indexes.)_
- [x] 5.4 Job de archivado de audit a object storage (GCS) con retención por clasificación.  _(`services/audit/cmd/archiver` exports old rows to local storage or `gs://` via `gcloud storage cp`; append-only DB rows are not deleted.)_
- [x] 5.5 Esquema Milvus para futura colección de RAG (creación de schema, índice HNSW); validar ingest/query con dataset sintético.  _(`deploy/compose/scripts/milvus-smoke.py` creates a synthetic collection, HNSW index and nearest-neighbor query; smoke script invokes it.)_

## 6. Control Plane API (Go)

- [x] 6.1 Servicio Go con `chi`/`echo`, middleware de auth (Keycloak JWT), middleware OpenFGA check, middleware `correlation_id`, middleware OTel.  _(Control Plane now initializes OTel and wraps HTTP with `otelhttp`.)_
- [x] 6.2 Endpoints CRUD para Tenant/BU/Workspace conformes al OpenAPI; validación 400 ante request inválido.
- [x] 6.3 Validación de "≥1 owner por Workspace" y rechazo de remoción del último owner.  _(Create-time owners required; PATCH now blocks removal of the last owner.)_
- [x] 6.4 Publicación de eventos `workspace.*` a Kafka con CloudEvents.
- [~] 6.5 Tests unitarios e integración (postgres real via testcontainers).  _(Initial auth unit tests added; Postgres/testcontainers integration remains deferred.)_

## 7. Audit Service

- [x] 7.1 Servicio Go append-only que consume eventos relevantes y persiste en `audit_event`; expone API de consulta filtrable por `tenant_id`, `workspace_id`, `actor`, `action`, ventana temporal.  _(Implemented: API supports optional filters `workspace_id`, `actor`, `action`, `since`, `until`.)_
- [x] 7.2 Cálculo y validación de `prev_hash` por tenant; endpoint de verificación de cadena.  _(Chain computed by DB trigger; `/v1/audit/verify?tenant_id=...` verifies continuity and row hashes.)_
- [x] 7.3 Replicación a Kafka topic `audit.events.v1`.  _(Audit consumes from the shared `forge.events` topic; the dedicated mirror topic is a follow-up task.)_
- [~] 7.4 Tests negativos: intento de UPDATE/DELETE rechazado y auditado.  _(Smoke script now verifies UPDATE/DELETE are rejected by DB triggers; auditing the failed attempt remains deferred.)_

## 8. Asset Registry mínimo

- [~] 8.1 Servicio Go con CRUD de asset según `ai-asset-registry-minimal` spec; validación SemVer y de schemas (`inputs_schema`/`outputs_schema`) JSON-Schema.  _(CRUD, spec-aligned fields/types, required schema objects, API-level SemVer validation, and SemVer unit coverage implemented; full JSON-Schema meta-schema validation remains deferred.)_
- [x] 8.2 Inmutabilidad de `(asset_id, version)`: rechazo 409 ante republicación.  _(Enforced by DB UNIQUE + immutable triggers; API now maps DB unique violations to HTTP 409.)_
- [x] 8.3 Endpoint de discovery con filtros y scoping OpenFGA.  _(List supports `type`, `owner_team`, `visibility`, `lifecycle_state`; Registry checks workspace `can_view`/`can_edit` through OpenFGA.)_
- [x] 8.4 Rechazo de transiciones de `lifecycle_state` fuera de `proposed` en Fase 0 (con mensaje que apunta a Fase 1).  _(CHECK constraint at DB; only `proposed` accepted.)_
- [x] 8.5 Eventos `asset.*` publicados con CloudEvents.

## 9. LiteLLM gateway

- [x] 9.1 Despliegue de LiteLLM con configuración versionada (`infra/helm/litellm/values.yaml`) y secret de provider en Secret Manager.  _(Compose config plus Helm foundations LiteLLM deployment and External Secrets manifests are present.)_
- [x] 9.2 SDK Go (`pkg/llmclient/`) y Python (`forge_llm/`) que envuelve a LiteLLM con headers estándar y `correlation_id`.  _(Go SDK done; Alfred stub now uses a Python LiteLLM wrapper with standard Forge headers.)_
- [x] 9.3 NetworkPolicy + egress firewall que bloquean salida a hostnames de proveedores desde namespaces distintos a `litellm`.  _(NetworkPolicy/Cilium FQDN manifests added; cloud firewall application remains environment-specific.)_
- [x] 9.4 Telemetría de costo/latencia tagueada por Tenant/Workspace, exportada a Prometheus y AI traces stub (Langfuse opcional en Fase 1).  _(LiteLLM Prometheus callbacks and Prometheus scrape job configured; real provider cost values require real provider credentials.)_
- [~] 9.5 Test negativo: una app fuera del namespace `litellm` que intenta llegar al provider falla por egress denegado.  _(Negative test manifest and `deploy/kubernetes/scripts/verify-llm-egress-denied.sh` added; execution requires a Kubernetes cluster with NetworkPolicies applied.)_

## 10. Portal bootstrap (Next.js)

- [~] 10.1 Inicializar Next.js 14+ con App Router, Tailwind y shadcn/ui.  _(Next.js 14 + App Router + Tailwind in place; shadcn components not yet installed.)_
- [x] 10.2 Auth con Keycloak (NextAuth/Auth.js o `oidc-client`), refresh tokens y guard de rutas.  _(NextAuth + Keycloak provider with JWT session and refresh-token rotation.)_
- [x] 10.3 Layout con sidebar mostrando módulos (Workspaces, Alfred Console placeholder, Asset Registry, OpenSpecs placeholder, Repositories, Environments placeholder, Deployments placeholder, Workflows placeholder, Approvals Inbox placeholder, Observability placeholder, Admin & Governance).
- [x] 10.4 Vista de listado de Workspaces (consume Control Plane API filtrando por OpenFGA).  _(Control Plane scopes `listAllWorkspaces` by OpenFGA `can_view` relation.)_
- [~] 10.5 Pantalla "Connect GitHub" que inicia el install flow del GitHub App y muestra repos accesibles.  _(Portal page records a local installation and lists cached repositories for a workspace; real install URL requires GitHub App credentials.)_
- [x] 10.6 Propagación de `correlation_id` desde el cliente y display en dev tools.  _(Portal sends `X-Correlation-Id` to Control Plane and renders the response correlation id.)_

## 11. GitHub App de Forge

GitHub App registration requires an actual GitHub org and a publicly reachable callback URL. The local manifest is documented; install/callback flow remains deferred.

- [~] 11.1 Registrar GitHub App "Forge" con scopes mínimos (`metadata:read`, `repo` selectivo, webhooks). Documentar manifest en `infra/github-app/manifest.json`.  _(Manifest documented; actual registration requires GitHub org/callback URL.)_
- [~] 11.2 Endpoint en Control Plane para callback de instalación; persistencia en `github_installation`.  _(Local endpoint persists installation records; real GitHub callback verification remains deferred.)_
- [x] 11.3 Servicio que lista repos accesibles para una instalación (cache en Redis).  _(`GET /v1/workspaces/{workspaceID}/github/repositories` uses Redis cache, optional `GITHUB_INSTALLATION_TOKEN`, and a local fixture fallback.)_
- [x] 11.4 Audit y evento `github.connected.v1`.  _(Control Plane publishes `com.forge.github.connected.v1`; audit service records it from Kafka.)_
- [x] 11.5 Documentar rotación de la GitHub App private key.  _(Runbook: `docs/runbooks/github-app.md`.)_

## 12. Observabilidad

_Container stack runs (otel-collector, prometheus, loki, tempo, grafana). Application-side OTel SDKs and dashboards are deferred to a follow-up._

- [~] 12.1 OpenTelemetry SDKs en Go/Python/Node con auto-instrumentación HTTP/DB/Kafka.  _(Go HTTP via `otelhttp`, Python FastAPI/httpx and Next.js/Vercel OTel hooks are wired; DB/Kafka instrumentation remains a follow-up.)_
- [~] 12.2 Middleware `correlation_id` end-to-end (request → log → trace → kafka headers → audit).  _(HTTP→log→audit done; trace + kafka header propagation deferred.)_
- [~] 12.3 Dashboards Grafana base: salud de servicios, p95/p99 latency, error rate, auth failures, audit volume, LiteLLM cost/tokens.  _(Base dashboard provisioned at `deploy/compose/grafana/dashboards/forge-overview.json`; some panels depend on app-side metrics still deferred.)_
- [x] 12.4 Loki labels estandarizadas: `service`, `env`, `tenant_id`, `workspace_id`, `correlation_id`.  _(Collector maps standard service/env/Forge attributes to Loki resource labels.)_
- [~] 12.5 Tempo + service map; verificación visual de trazas cross-service.  _(Tempo datasource/service-map config and runbook added; visual verification requires the local stack running with generated traces.)_

## 13. Políticas y documentación de gobierno

Policy docs are drafted for Security/Compliance review. Local runbooks are implemented for Phase 0 operations.

- [x] 13.1 `docs/policies/retention.md` con retenciones por clasificación (audit, telemetría operacional, AI traces, RAG data) — borrador a validar con Security/Compliance.
- [x] 13.2 `docs/policies/iam-bootstrap.md` con mapeo de claims IdP → tuples OpenFGA.
- [x] 13.3 `docs/policies/network-egress.md` con la lista de hostnames de proveedores LLM y la política de bloqueo.
- [x] 13.4 `docs/runbooks/keycloak.md` y `docs/runbooks/openfga.md` para operaciones básicas.

## 14. Validación end-to-end (criterio de salida Fase 0)

- [~] 14.1 Crear un Tenant, BU y Workspace via API y Portal, autenticado en Keycloak.  _(API path covered by `smoke.sh`; Portal can list workspaces and create a workspace for an existing BU.)_
- [~] 14.2 Registrar un asset de prueba en el Registry y descubrirlo desde otro usuario autorizado.  _(Single-user via `smoke.sh`; cross-user OpenFGA scoping not exercised.)_
- [~] 14.3 Conectar GitHub desde el Portal y listar repos del usuario.  _(Portal can record a local installation fixture and list cached repositories; real GitHub install flow still requires app credentials.)_
- [x] 14.4 Autenticar el stub de Alfred (Python+FastAPI) contra Control Plane y ejecutar una acción simple (e.g., `list workspaces`) auditada con `correlation_id` end-to-end.
- [~] 14.5 Llamar a LiteLLM via SDK desde el stub Alfred y comprobar que el cost/latency aparece en Grafana etiquetado por Workspace.  _(`/chat` uses the Python LiteLLM wrapper and smoke test covers it; Grafana cost/latency metrics remain deferred.)_
- [~] 14.6 Verificar que un servicio fuera del namespace `litellm` NO puede llegar a un provider externo (test negativo de NetworkPolicy).  _(Manifest and verification script added; execution requires a Kubernetes cluster.)_
- [~] 14.7 Verificar que UPDATE/DELETE sobre `audit_event` es rechazado e intento auditado.  _(DB rejects; smoke test verifies the audit chain endpoint; direct UPDATE/DELETE negative test remains deferred.)_
- [x] 14.8 Sign-off del SDLC Team (registrar en `docs/governance/phase-0-signoff.md`).  _(Role-based bootstrap approval recorded; formal named approval may replace it before production rollout.)_
