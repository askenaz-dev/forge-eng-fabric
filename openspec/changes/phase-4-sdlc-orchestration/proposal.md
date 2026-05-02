## Why

Con apps onboardeadas y desplegables (Fases 2–3), Forge debe **orquestar el ciclo SDLC end-to-end de manera agéntica**: desde una idea/ticket en Jira hasta producción, atravesando product, arquitectura, diseño, desarrollo, QA, seguridad, DevOps, SRE y FinOps con **gates de progresión** y **trazabilidad bidireccional** OpenSpec ↔ Jira ↔ Confluence ↔ PR ↔ Deployment ↔ Telemetría. Sin esto, Alfred opera por "fragmentos" sin un hilo conductor cross-fase y la organización pierde visibilidad y control de calidad.

## What Changes

- **NUEVO**: Capabilities por fase del SDLC, cada una expuesta como conjunto de Skills + Prompts + MCP tools + policies:
  - `sdlc-product`: refinamiento de requisitos, generación de user stories, criterios de aceptación, priorización.
  - `sdlc-architecture`: ADRs, evaluación de opciones, validación contra OpenSpec del Workspace, security/compliance review.
  - `sdlc-design`: diseño técnico detallado, secuencias, esquemas de datos, contratos de API, threat modeling ligero.
  - `sdlc-development`: scaffolding incremental, generación/edición de código, refactors guiados por tests.
  - `sdlc-qa`: generación y mantenimiento de test suites (unit, integration, E2E, performance, contract), ejecución y triage.
  - `sdlc-security`: SAST/SCA/DAST orquestación, threat modeling, secret hygiene, vuln triage.
  - `sdlc-devops`: pipelines, runtimes, deploys, releases, configuración, observability hooks.
  - `sdlc-sre`: SLOs, SLIs, error budgets, alerting, runbooks, on-call rotation hooks.
  - `sdlc-finops`: estimación y monitoreo de costos (cloud + LLM), presupuestos, alertas y recomendaciones.
- **NUEVO**: **Jira MCP** con cobertura de issues/epics/sprints/comments y **Confluence MCP** con páginas/spaces; ambos read/write gobernados.
- **NUEVO**: **Trazabilidad cross-fase**: cada artefacto (story, ADR, diseño, PR, test plan, deploy) carga `correlation_id` + `openspec_ids` + `jira_keys` + `confluence_urls`, persistido en `traceability_link`.
- **NUEVO**: **SDLC orchestrator** (servicio): coordina la progresión por fases, evalúa **quality + security gates** entre fases, bloquea avance si gates fallan.
- **NUEVO**: **Quality gates** configurables por Workspace/criticidad: cobertura, complejidad ciclomática, deuda técnica, performance budgets, accesibilidad (frontend), test pyramid balance.
- **NUEVO**: **Security gates** configurables: ausencia de findings ≥severity, threat model approved, secrets hygiene, dependency policy, compliance tags.
- **NUEVO**: **Phase progression UI**: Portal muestra el estado de cada fase para una iniciativa, gates evaluados, blockers, accionables.
- **NUEVO**: Eventos `sdlc.phase.*` (`entered`, `gate_evaluated`, `progressed`, `blocked`, `regressed`), `traceability.link.created.v1`, `jira.issue.*`, `confluence.page.*`.
- **MODIFICADO**: `mcp-and-skills` — añadir Jira MCP (escritura), Confluence MCP (escritura), y skills por fase del SDLC (≥3 por capability).
- **MODIFICADO**: `policies-and-approvals` — añadir policy templates de gates por fase (`require-architecture-review`, `require-threat-model`, `require-test-coverage`, `require-slo-defined`).
- **MODIFICADO**: `openspec-backbone` — extender `decision_log` y `linked_artifacts` para incluir referencias Jira/Confluence/test-runs/SLOs.
- **Criterio de salida (E2E)**: una iniciativa nace como Jira epic, Alfred genera OpenSpec, ADRs, diseño, código, tests, deploy a stage; cada fase pasa sus gates, todo está vinculado bidireccionalmente, y un dashboard muestra el camino completo en una sola vista.

## Capabilities

### New Capabilities

- `sdlc-product`: skills/prompts/policies para refinamiento de requisitos y user stories.
- `sdlc-architecture`: ADRs, opciones, security/compliance review.
- `sdlc-design`: diseño técnico, contratos, threat modeling ligero.
- `sdlc-development`: codificación asistida con tests-first.
- `sdlc-qa`: test suites multi-nivel y triage.
- `sdlc-security`: orquestación de scanners y vuln triage.
- `sdlc-devops`: pipelines/deploys/releases en coordinación con Fase 3.
- `sdlc-sre`: SLOs/SLIs/runbooks/alerting.
- `sdlc-finops`: estimación, monitoreo, presupuestos y recomendaciones.
- `sdlc-orchestrator`: coordinación cross-fase, evaluación de gates, progresión bloqueable.
- `traceability-graph`: grafo de relaciones bidireccionales OpenSpec ↔ Jira ↔ Confluence ↔ PR ↔ Deploy ↔ Telemetría.

### Modified Capabilities

- `mcp-and-skills`: añadir Jira y Confluence MCP (read/write) y skills por fase del SDLC.
- `policies-and-approvals`: añadir policy templates de gates por fase.
- `openspec-backbone`: extender `decision_log` y `linked_artifacts` con referencias Jira/Confluence/tests/SLOs.

## Impact

- **Servicios nuevos**: `services/sdlc-orchestrator/` (Go), `services/traceability/` (Go), `services/mcp/{jira,confluence}/` (Python), librerías de Skills por fase (`skills/sdlc-*`).
- **Datos**: tablas `sdlc_initiative`, `sdlc_phase_state`, `sdlc_gate_result`, `traceability_link`, `traceability_node` (vista grafo), índice `correlation_id`.
- **Eventos**: `sdlc.phase.entered.v1`, `sdlc.phase.gate_evaluated.v1`, `sdlc.phase.progressed.v1`, `sdlc.phase.blocked.v1`, `sdlc.phase.regressed.v1`, `traceability.link.created.v1`, `jira.issue.created.v1`, `jira.issue.updated.v1`, `confluence.page.created.v1`, `confluence.page.updated.v1`.
- **Integraciones**: Jira (Cloud o Server), Confluence (Cloud o Server), test runners (Go test, pytest, Playwright, k6), scanners (de Fase 2), Cloud Billing (FinOps), Langfuse (LLM costs de Fase 1).
- **Portal**: módulo "Initiative" (vista por fase con gates), módulo "Traceability" (grafo con drill-down), enriquecimiento de OpenSpec viewer con links Jira/Confluence/tests/SLOs.
- **Riesgos**: explosión de complejidad de skills (mitigado con catálogo modular y eval suite por skill), latencia de queries de grafo (mitigado con materialized views), tokens y rate limits Jira/Confluence (mitigado con caching + backoff).
- **Out of scope**: workflows visuales y marketplace (Fase 5), self-healing autónomo (Fase 6).
