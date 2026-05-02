# Tasks — phase-0-foundations

## 1. Monorepo & CI bootstrap

- [ ] 1.1 Crear estructura de monorepo: `services/control-plane/` (Go), `services/alfred/` (Python+FastAPI, stub), `services/registry/` (Go), `portal/` (Next.js), `infra/` (Terraform/Helm), `deploy/` (Compose + manifests), `contracts/` (events/, openapi/, openfga/), `docs/`.
- [ ] 1.2 Inicializar repos con `go.work` (Go modules), `pyproject.toml` (uv/poetry), `pnpm-workspace.yaml` (Node) y `Makefile` raíz con targets `bootstrap`, `lint`, `test`, `up`, `down`.
- [ ] 1.3 Configurar conventional commits (commitlint), pre-commit hooks (lint+format), EditorConfig y `.gitignore` global.
- [ ] 1.4 Configurar GitHub Actions iniciales: lint, build y test por servicio (Go, Python, Node) y validación de contratos OpenAPI/OpenFGA/CloudEvents.
- [ ] 1.5 Documentar `docs/getting-started.md` con setup local (Compose) y cluster (Minikube/GKE).

## 2. Contratos & schemas

- [ ] 2.1 Definir esquema CloudEvents v1.0 en `contracts/events/` con extensiones `forgetenantid`, `forgeworkspaceid`, `forgeactor`, `forgecorrelationid`.
- [ ] 2.2 Publicar contratos versionados de eventos: `workspace.created.v1`, `workspace.updated.v1`, `workspace.archived.v1`, `asset.created.v1`, `asset.updated.v1`, `audit.events.v1`, `auth.failed.v1`, `github.connected.v1`.
- [ ] 2.3 Definir `contracts/openfga/authorization-model.json` con tipos `tenant`, `business_unit`, `workspace`, `asset`, `repo`, `environment`, `deployment` y relaciones `parent`, `owner`, `member`, `viewer`; tests de policy en `contracts/openfga/tests/`.
- [ ] 2.4 Definir `contracts/openapi/control-plane.yaml` (CRUD Tenant/BU/Workspace) y `contracts/openapi/registry.yaml` (CRUD Asset).
- [ ] 2.5 Generar clientes/SDK desde OpenAPI para Go, Python y TypeScript (codegen en CI).

## 3. Infra base — Compose dev

- [ ] 3.1 Crear `deploy/compose/docker-compose.yaml` con: postgres, redis, kafka (KRaft single-node), keycloak, openfga, milvus, otel-collector, prometheus, grafana, loki, tempo, litellm.
- [ ] 3.2 Provisionar realm de Keycloak via import declarativo (`deploy/compose/keycloak/realm-export.json`) con clients `forge-portal`, `forge-control-plane` y un usuario seed.
- [ ] 3.3 Cargar modelo OpenFGA inicial al arranque (script `deploy/compose/scripts/openfga-bootstrap.sh`).
- [ ] 3.4 Configurar LiteLLM (`deploy/compose/litellm/config.yaml`) con un proveedor de prueba (Vertex AI o stub local) y key gestionada por env.
- [ ] 3.5 Smoke test E2E (`deploy/compose/scripts/smoke.sh`): up → login Keycloak → crear Tenant/BU/Workspace → publicar asset → llamar LiteLLM → verificar audit.

## 4. Infra base — Cluster (Helm/Terraform)

- [ ] 4.1 Charts Helm en `infra/helm/` para cada servicio de plataforma (control-plane, registry, alfred-stub, portal, audit-service, otel-collector).
- [ ] 4.2 Charts Helm para dependencias (kafka via Strimzi o managed, postgres operator, redis, milvus, keycloak, openfga, grafana stack, litellm).
- [ ] 4.3 Módulos Terraform en `infra/terraform/` para GCP base (VPC, GKE Autopilot pequeño, Cloud SQL Postgres, Memorystore Redis, Artifact Registry).
- [ ] 4.4 NetworkPolicies para bloquear egress LLM a todo namespace excepto `litellm`.
- [ ] 4.5 Secrets management: Secret Manager + External Secrets Operator (mapeado por namespace).

## 5. Persistencia & migrations

- [ ] 5.1 Migrations SQL (sqlc/goose) para: `tenant`, `business_unit`, `workspace`, `workspace_owner`, `asset`, `audit_event`, `github_installation`.
- [ ] 5.2 Trigger en `audit_event` que rechaza UPDATE/DELETE; columna `prev_hash` y stored procedure para chain por `tenant_id`.
- [ ] 5.3 Particionamiento mensual de `audit_event` por `tenant_id`+`event_time`.
- [ ] 5.4 Job de archivado de audit a object storage (GCS) con retención por clasificación.
- [ ] 5.5 Esquema Milvus para futura colección de RAG (creación de schema, índice HNSW); validar ingest/query con dataset sintético.

## 6. Control Plane API (Go)

- [ ] 6.1 Servicio Go con `chi`/`echo`, middleware de auth (Keycloak JWT), middleware OpenFGA check, middleware `correlation_id`, middleware OTel.
- [ ] 6.2 Endpoints CRUD para Tenant/BU/Workspace conformes al OpenAPI; validación 400 ante request inválido.
- [ ] 6.3 Validación de "≥1 owner por Workspace" y rechazo de remoción del último owner.
- [ ] 6.4 Publicación de eventos `workspace.*` a Kafka con CloudEvents.
- [ ] 6.5 Tests unitarios e integración (postgres real via testcontainers).

## 7. Audit Service

- [ ] 7.1 Servicio Go append-only que consume eventos relevantes y persiste en `audit_event`; expone API de consulta filtrable por `tenant_id`, `workspace_id`, `actor`, `action`, ventana temporal.
- [ ] 7.2 Cálculo y validación de `prev_hash` por tenant; endpoint de verificación de cadena.
- [ ] 7.3 Replicación a Kafka topic `audit.events.v1`.
- [ ] 7.4 Tests negativos: intento de UPDATE/DELETE rechazado y auditado.

## 8. Asset Registry mínimo

- [ ] 8.1 Servicio Go con CRUD de asset según `ai-asset-registry-minimal` spec; validación SemVer y de schemas (`inputs_schema`/`outputs_schema`) JSON-Schema.
- [ ] 8.2 Inmutabilidad de `(asset_id, version)`: rechazo 409 ante republicación.
- [ ] 8.3 Endpoint de discovery con filtros y scoping OpenFGA.
- [ ] 8.4 Rechazo de transiciones de `lifecycle_state` fuera de `proposed` en Fase 0 (con mensaje que apunta a Fase 1).
- [ ] 8.5 Eventos `asset.*` publicados con CloudEvents.

## 9. LiteLLM gateway

- [ ] 9.1 Despliegue de LiteLLM con configuración versionada (`infra/helm/litellm/values.yaml`) y secret de provider en Secret Manager.
- [ ] 9.2 SDK Go (`pkg/llmclient/`) y Python (`forge_llm/`) que envuelve a LiteLLM con headers estándar y `correlation_id`.
- [ ] 9.3 NetworkPolicy + egress firewall que bloquean salida a hostnames de proveedores desde namespaces distintos a `litellm`.
- [ ] 9.4 Telemetría de costo/latencia tagueada por Tenant/Workspace, exportada a Prometheus y AI traces stub (Langfuse opcional en Fase 1).
- [ ] 9.5 Test negativo: una app fuera del namespace `litellm` que intenta llegar al provider falla por egress denegado.

## 10. Portal bootstrap (Next.js)

- [ ] 10.1 Inicializar Next.js 14+ con App Router, Tailwind y shadcn/ui.
- [ ] 10.2 Auth con Keycloak (NextAuth/Auth.js o `oidc-client`), refresh tokens y guard de rutas.
- [ ] 10.3 Layout con sidebar mostrando módulos (Workspaces, Alfred Console placeholder, Asset Registry, OpenSpecs placeholder, Repositories, Environments placeholder, Deployments placeholder, Workflows placeholder, Approvals Inbox placeholder, Observability placeholder, Admin & Governance).
- [ ] 10.4 Vista de listado de Workspaces (consume Control Plane API filtrando por OpenFGA).
- [ ] 10.5 Pantalla "Connect GitHub" que inicia el install flow del GitHub App y muestra repos accesibles.
- [ ] 10.6 Propagación de `correlation_id` desde el cliente y display en dev tools.

## 11. GitHub App de Forge

- [ ] 11.1 Registrar GitHub App "Forge" con scopes mínimos (`metadata:read`, `repo` selectivo, webhooks). Documentar manifest en `infra/github-app/manifest.json`.
- [ ] 11.2 Endpoint en Control Plane para callback de instalación; persistencia en `github_installation`.
- [ ] 11.3 Servicio que lista repos accesibles para una instalación (cache en Redis).
- [ ] 11.4 Audit y evento `github.connected.v1`.
- [ ] 11.5 Documentar rotación de la GitHub App private key.

## 12. Observabilidad

- [ ] 12.1 OpenTelemetry SDKs en Go/Python/Node con auto-instrumentación HTTP/DB/Kafka.
- [ ] 12.2 Middleware `correlation_id` end-to-end (request → log → trace → kafka headers → audit).
- [ ] 12.3 Dashboards Grafana base: salud de servicios, p95/p99 latency, error rate, auth failures, audit volume, LiteLLM cost/tokens.
- [ ] 12.4 Loki labels estandarizadas: `service`, `env`, `tenant_id`, `workspace_id`, `correlation_id`.
- [ ] 12.5 Tempo + service map; verificación visual de trazas cross-service.

## 13. Políticas y documentación de gobierno

- [ ] 13.1 `docs/policies/retention.md` con retenciones por clasificación (audit, telemetría operacional, AI traces, RAG data) — borrador a validar con Security/Compliance.
- [ ] 13.2 `docs/policies/iam-bootstrap.md` con mapeo de claims IdP → tuples OpenFGA.
- [ ] 13.3 `docs/policies/network-egress.md` con la lista de hostnames de proveedores LLM y la política de bloqueo.
- [ ] 13.4 `docs/runbooks/keycloak.md` y `docs/runbooks/openfga.md` para operaciones básicas.

## 14. Validación end-to-end (criterio de salida Fase 0)

- [ ] 14.1 Crear un Tenant, BU y Workspace via API y Portal, autenticado en Keycloak.
- [ ] 14.2 Registrar un asset de prueba en el Registry y descubrirlo desde otro usuario autorizado.
- [ ] 14.3 Conectar GitHub desde el Portal y listar repos del usuario.
- [ ] 14.4 Autenticar el stub de Alfred (Python+FastAPI) contra Control Plane y ejecutar una acción simple (e.g., `list workspaces`) auditada con `correlation_id` end-to-end.
- [ ] 14.5 Llamar a LiteLLM via SDK desde el stub Alfred y comprobar que el cost/latency aparece en Grafana etiquetado por Workspace.
- [ ] 14.6 Verificar que un servicio fuera del namespace `litellm` NO puede llegar a un provider externo (test negativo de NetworkPolicy).
- [ ] 14.7 Verificar que UPDATE/DELETE sobre `audit_event` es rechazado e intento auditado.
- [ ] 14.8 Sign-off del SDLC Team (registrar en `docs/governance/phase-0-signoff.md`).
