## Why

Con SDLC orquestado y trazabilidad cross-fase (Fase 4), Forge debe **democratizar la composición de capacidades**: editor visual + DSL para que equipos de producto/ingeniería compongan workflows agénticos reusables (desde scaffolding de un microservicio hasta un release-train), ejecutarlos durablemente con Temporal, versionarlos como assets gobernados, y compartirlos en un **Tenant marketplace** interno con eval avanzada y dashboards. Sin esto, cada Workspace reinventa flujos cada vez y la organización pierde reutilización + governance.

## What Changes

- **NUEVO**: **Workflow DSL** YAML versionable (alternativa al editor visual; ambos producen el mismo AST canónico).
- **NUEVO**: **Editor visual** estilo n8n/Flowise en el Portal: nodos = MCPs, Skills, Prompt Templates, Sub-Workflows, gates, branches, loops, human-in-the-loop steps.
- **NUEVO**: Servicio **workflow-runtime** sobre **Temporal** con `durable: true`: ejecuciones reanudables, retries, timeouts, signals, queries, child workflows.
- **NUEVO**: **SemVer + immutability**: una versión de workflow publicada es inmutable; ediciones generan nuevas versiones; compatibility checks (major bump si breaking).
- **NUEVO**: **Marketplace interno por Tenant**: catálogo navegable de workflows (privados al Workspace, compartidos al Tenant, certificados Forge), con instalación en un click a Workspaces destino.
- **NUEVO**: **Workflow assets en Registry** con `lifecycle_state` y `trust_level`; certificación por Forge requiere eval ≥ threshold y security review.
- **NUEVO**: **Eval harness avanzado**: golden datasets por workflow, regression tests, A/B de variantes, métricas de éxito de negocio (conversion, latency, cost).
- **NUEVO**: **Dashboards por asset**: para cada workflow en producción, métricas de invocaciones, success rate, p95 latency, cost per execution, drift en evals.
- **NUEVO**: Eventos `workflow.*` (`created`, `published`, `executed`, `step_completed`, `step_failed`, `retried`, `compensated`, `installed_to_workspace`).
- **NUEVO**: **Human-in-the-loop step type** integrado con Approvals Inbox (Fase 1) — pausa workflow esperando approval con timeout configurable.
- **MODIFICADO**: `mcp-and-skills` — los nodos visuales referencian Skills/MCPs/Prompts del Registry; el editor consume el catálogo.
- **MODIFICADO**: `ai-asset-registry-minimal` — añadir tipo de asset `workflow` con sub-recursos `version`, `eval_run`, `installation`.
- **MODIFICADO**: `policies-and-approvals` — policies para publicación/instalación de workflows (`require-eval-pass`, `require-security-review`, `require-tenant-share-approval`).
- **Criterio de salida (E2E)**: un usuario diseña un workflow en el editor visual, lo guarda como asset versionado, pasa eval suite, lo publica al marketplace del Tenant, otro Workspace lo instala, ejecuta y observa métricas en el dashboard del asset.

## Capabilities

### New Capabilities

- `workflow-dsl`: lenguaje YAML canónico (AST shared con editor visual) con tipos de nodo, validación schema y lint.
- `workflow-visual-editor`: editor en el Portal con drag/drop, debug, preview y publish.
- `workflow-runtime`: ejecución durable sobre Temporal con retries, signals, queries, child workflows y compensations.
- `workflow-versioning`: SemVer + immutability + breaking-change detection automatizada.
- `tenant-workflow-marketplace`: catálogo Tenant-wide con visibilidades (private/tenant/forge-certified) e instalación en Workspaces.
- `advanced-eval-harness`: golden datasets, regression, A/B, métricas de negocio para workflows y skills.
- `per-asset-observability`: dashboards por asset (skill, prompt, workflow) con invocaciones, éxito, latencia, costo, drift.
- `human-in-the-loop-steps`: tipo de nodo que pausa el workflow esperando approval del Inbox.

### Modified Capabilities

- `mcp-and-skills`: el editor consume catálogo Registry en vivo; nodos referencian assets por id+versión.
- `ai-asset-registry-minimal`: añadir asset `type=workflow` con sub-recursos `version`, `eval_run`, `installation`.
- `policies-and-approvals`: añadir policies de publicación (`require-eval-pass`, `require-security-review`) e instalación (`require-tenant-share-approval`).

## Impact

- **Servicios nuevos**: `services/workflow-runtime/` (Go o Python wrapper de Temporal), `services/workflow-registry/` (Go), `services/marketplace/` (Go), `services/eval-harness-adv/` (Python), `services/asset-observability/` (Go).
- **Datos**: tablas `workflow`, `workflow_version`, `workflow_execution`, `workflow_step_event`, `workflow_eval_run`, `workflow_install`, `marketplace_listing`.
- **Eventos**: `workflow.created.v1`, `workflow.published.v1`, `workflow.execution.started.v1`, `workflow.step.started.v1`, `workflow.step.completed.v1`, `workflow.step.failed.v1`, `workflow.retried.v1`, `workflow.compensated.v1`, `workflow.installed_to_workspace.v1`.
- **Integraciones**: Temporal cluster (self-hosted o Temporal Cloud), Postgres extra schemas, ClickHouse o BigQuery para analytics de ejecuciones (opcional para alto volumen).
- **Portal**: módulo "Workflows" (lista + editor + debug + ejecuciones), módulo "Marketplace" (browse, install), enriquecimiento de Asset detail con tab Observability.
- **Riesgos**: complejidad del editor (mitigado adoptando libs maduras tipo React Flow + AST canónico estable), durabilidad y costo Temporal, fairness multi-tenant en Temporal namespaces.
- **Out of scope**: marketplace público inter-Tenant, monetización, self-healing autónomo (Fase 6).
