## Why

Forge necesita una base segura, observable y extensible antes de habilitar cualquier capacidad agéntica. Sin tenancy, IAM, persistencia, event backbone, observabilidad y un gateway único de LLM, todo lo demás (Alfred, Registry productivo, Workflows, Deploys) se construiría sobre cimientos frágiles e inconsistentes. Esta fase establece la **plataforma mínima viable y gobernada** sobre la cual se montarán las fases 1–6.

## What Changes

- **NUEVO**: Estructura de monorepo (Go control-plane, Python agentic-plane, Next.js portal, IaC, deploy, docs) con conventional commits, linters y CI inicial.
- **NUEVO**: Esquema **CloudEvents** para eventos de plataforma (`workspace.*`, `asset.*`, `agent.*`, `workflow.*`, `deployment.*`, `audit.*`, `incident.*`) publicado como contrato versionado.
- **NUEVO**: **Apache Kafka** desplegado y validado como event backbone async desde el inicio.
- **NUEVO**: **PostgreSQL** (Cloud SQL) con backups y PITR; **Redis** (Memorystore) para caching/sesiones; esquemas iniciales de Tenant/BU/Workspace.
- **NUEVO**: **Milvus** desplegado para el futuro RAG de Alfred (capacidad de ingesta/recuperación validada con dataset de prueba; uso productivo en Fase 1).
- **NUEVO**: **Keycloak** (AuthN OIDC/SAML federado al IdP corporativo) y **OpenFGA** (AuthZ ReBAC) con modelo Tenant → BU → Workspace → {Asset, Repo, Environment, Deployment}.
- **NUEVO**: API de **Control Plane** en Go con CRUD de Tenant/BU/Workspace, autenticado por Keycloak y autorizado por OpenFGA; sin permisos cross-Workspace globales por defecto.
- **NUEVO**: **Audit service** append-only (Postgres + publicación a Kafka) con prohibición de update/delete por policy.
- **NUEVO**: Bootstrap del **Custom Portal** (Next.js + React + Tailwind/shadcn): login Keycloak, listado de Workspaces, vistas vacías de los módulos.
- **NUEVO**: Stack de observabilidad base: **OpenTelemetry collector**, **Prometheus**, **Grafana**, **Loki**, **Tempo**; instrumentación con `correlation_id`, métricas SLO y `/healthz` en cada servicio.
- **NUEVO**: **LiteLLM** como gateway único de modelos con al menos un proveedor (Vertex AI), SDK/cliente interno y network policies que bloquean acceso directo a proveedores.
- **NUEVO**: **AI Asset Registry mínimo** (Go API + Postgres) con CRUD y modelo de metadata; suficiente para registrar assets y validar el contrato (lifecycle completo y trust-level enforcement aterrizan en Fase 1).
- **NUEVO**: **GitHub App** de Forge con scopes mínimos; UI de "Conectar GitHub" en el Portal y prueba de listar repos del usuario.
- **NUEVO**: Políticas de retención de audit, telemetría y datos RAG por clasificación de datos.
- **Criterio de salida (E2E)**: crear Workspace, registrar asset, conectar GitHub, autenticar un Alfred-stub y ejecutar una acción simple auditada; **todo acceso LLM pasa por LiteLLM**.

## Capabilities

### New Capabilities

- `platform-foundations-bootstrap`: tenancy (Tenant/BU/Workspace), IAM (Keycloak + OpenFGA), audit append-only, event backbone (Kafka + CloudEvents), persistencia (PostgreSQL/Redis), Milvus (provisión), observabilidad base (OTel/Prometheus/Grafana/Loki/Tempo) y Custom Portal bootstrap.
- `model-gateway-bootstrap`: LiteLLM como punto único de acceso a modelos, con al menos un proveedor aprobado, SDK/cliente interno y bloqueo de acceso directo a proveedores.
- `ai-asset-registry-minimal`: CRUD básico de assets en el Registry con metadata mínima validada por schema, suficiente para registrar y descubrir assets de prueba; el lifecycle completo y trust-level enforcement se completan en Fase 1.

### Modified Capabilities

<!-- No hay capabilities preexistentes en openspec/specs/. -->

## Impact

- **Código y servicios nuevos**:
  - Monorepo `forge/` con `services/control-plane/` (Go), `services/alfred/` (Python+FastAPI, stub mínimo), `services/registry/` (Go), `portal/` (Next.js), `infra/` (Terraform/Helm), `deploy/` (Compose dev y manifests), `docs/`.
  - SQL migrations para Tenant/BU/Workspace, Audit y Asset.
  - OpenAPI specs para Control Plane API y Registry API.
  - Modelo OpenFGA inicial (`authorization-model.json`).
  - Schemas CloudEvents publicados como contratos versionados (`contracts/events/`).
- **Infraestructura**:
  - GCP como cloud base; runtime de bootstrap puede ser Minikube o GKE de desarrollo.
  - Postgres (Cloud SQL), Redis (Memorystore), Kafka (managed o self-hosted), Milvus, Keycloak, OpenFGA, LiteLLM, OTel + stack Grafana.
- **Dependencias / herramientas**: Go (≥1.22), Node.js (≥20), Python (≥3.12), Docker, Terraform, Helm, kubectl, kind/minikube para dev local.
- **Sistemas afectados**: GitHub (alta de la Forge App).
- **Riesgos**: complejidad inicial de IAM/ReBAC, configuración de network policies para bloquear bypass de LiteLLM, costos de bootstrap. Mitigaciones detalladas en design.md.
- **Out of scope (en esta fase)**: Alfred operativo (Fase 1), MCPs/Skills/Prompts (Fase 1), policies/approvals avanzadas (Fase 1), onboarding de apps (Fase 2), runtimes de despliegue de aplicaciones (Fase 3), workflows visuales (Fase 5), self-healing (Fase 6).
