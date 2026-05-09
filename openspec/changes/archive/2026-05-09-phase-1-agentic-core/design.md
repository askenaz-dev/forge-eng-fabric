## Context

Fase 0 dejó base operable (tenancy, IAM, audit, Kafka+CloudEvents, Postgres/Redis, Milvus listo, OTel, LiteLLM como gateway, Registry mínimo y Portal bootstrap). Fase 1 monta sobre esa base **el primer valor agéntico**: Alfred capaz de comprender intención, consultar contexto vía RAG, evaluar policy y operar herramientas de forma trazable, con OpenSpec como contrato vivo y un Registry productivo.

## Goals / Non-Goals

**Goals:**
- Alfred como **Control Plane Agent** operativo, con autonomía por defecto dentro de delegated permissions.
- OpenSpec editable por Alfred y humanos; trazabilidad bidireccional con GitHub/Jira/Confluence.
- Engine de policies/approvals configurable por Workspace/OpenSpec/asset/env/criticidad.
- Registry productivo con lifecycle completo, trust levels T0–T5 y eval scores.
- MCPs (GitHub, Jira, Confluence, OpenSpec) y primeras Skills/Prompt Templates publicados.
- Guardrails básicos y AI observability con Langfuse.

**Non-Goals:**
- Onboarding de apps con scaffolding (Fase 2).
- Despliegues de aplicaciones de usuarios sobre GKE/Cloud Run/Minikube (Fase 3).
- Capabilities por fase del SDLC y orquestación multi-fase profunda (Fase 4).
- Editor visual de workflows y marketplace interno (Fase 5).
- Self-healing (Fase 6).

## Decisions

### D1.1 — Framework de agente
**Decisión**: Alfred sobre **ADK** o framework equivalente compatible con MCP y A2A; Python+FastAPI como host.
**Alternativas**: framework propio.
**Rationale**: Reuso del ecosistema, soporte de tools/sessions/memory; evita reinventar el loop razonamiento/acción.

### D1.2 — Modelo de policy
**Decisión**: Policies declarativas en YAML/CEL evaluadas por un servicio dedicado (Go) consultando OpenFGA y atributos contextuales (env, criticality, trust_level, data_classification).
**Alternativas**: OPA/Rego.
**Rationale**: CEL es más accesible para autores de policy y suficiente para los predicados requeridos; OPA queda como opción si la complejidad crece.

### D1.3 — Approvals Inbox
**Decisión**: Aprobaciones modeladas como eventos durables con TTL, notificaciones por canal configurable (email/Slack), UI Inbox en el Portal y CLI.
**Alternativas**: workflow externo (Jira) como única vía.
**Rationale**: Internalizar reduce latencia y mantiene trazabilidad nativa.

### D1.4 — Permisos delegados
**Decisión**: Modelo "grant" explícito por (sujeto=Alfred, scope=Workspace/Repo/Env/CloudProject, action_class, max_criticality, expiration). Tuples reflejados en OpenFGA y registros versionados en Postgres. Revocación inmediata.
**Alternativas**: SA con todos los permisos.
**Rationale**: Least privilege; permite auditoría y rotación.

### D1.5 — Lifecycle del Registry
**Decisión**: Máquina de estados `proposed → in_review → approved → deprecated → retired`, transiciones audit-ables, con aprobaciones por trust level (T4 requiere DevOps/SRE, T5 requiere SDLC Team). Producción solo invoca `approved`.
**Alternativas**: lifecycle relajado.
**Rationale**: Necesario para reuso seguro y supply chain.

### D1.6 — RAG isolation
**Decisión**: Una colección Milvus por Workspace (más índice cross-workspace **opcional** para fuentes con visibilidad `tenant`); cada documento etiquetado con `workspace_id`, `data_classification`, `source_ref`, `provenance_signed`.
**Alternativas**: una colección global con filtros.
**Rationale**: Aislamiento físico reduce blast radius y simplifica retention/erase requests.

### D1.7 — Guardrails
**Decisión**: Capa de guardrails entre Alfred y LLM/tool: prompt structuring (system vs untrusted context separados), sanitización con detector de injection en el contexto recuperado, allowlists de tools por trust_level y data_classification, schema validation de outputs JSON.
**Alternativas**: confiar solo en el modelo.
**Rationale**: Defense-in-depth; mitiga prompt injection desde Confluence/Jira/GitHub y memory poisoning.

### D1.8 — Eval harness
**Decisión**: Harness en Python que ejecuta suites por asset (calidad/seguridad/costo/latencia), persiste `eval_run` y `eval_score`, integra con Langfuse y bloquea aprobación si scores < umbral por trust level.
**Alternativas**: framework externo only (Promptfoo).
**Rationale**: Necesidad de integración con lifecycle y telemetría propia; el harness puede invocar herramientas externas internamente.

### D1.9 — Ingesta RAG
**Decisión**: Pipeline batch + on-change (webhooks GitHub, eventos OpenSpec, eventos Confluence/Jira) con chunking por documento/sección, embedding via LiteLLM (modelo aprobado) y validación de procedencia.
**Alternativas**: solo batch nocturno.
**Rationale**: Frescura del contexto crítico para utilidad; on-change reduce el lag.

## Risks / Trade-offs

| Riesgo | Mitigación |
|---|---|
| Prompt injection desde fuentes externas | Guardrails (separación, sanitización, allowlists), detección de instrucciones maliciosas en chunks recuperados, evals de seguridad. |
| Memory poisoning en Milvus | Procedencia firmada, validación de fuente, segmentación por Workspace y data_classification, evals de retrieval. |
| Alfred ejecuta acciones fuera de policy | Policy checks pre-tool, dry-run para acciones destructivas, audit y rollback documentado, HITL configurable. |
| Costos LLM crecientes | LiteLLM budgets, caching de embeddings y prompts repetidos, model routing por cost class, alertas. |
| Calidad inicial de evals | Comenzar con evals deterministas (schema, latencia, costo), añadir LLM-as-judge gradualmente con calibración humana. |
| Sobrecarga de Approvals | Granularidad por action_class y criticality; defaults razonables; métricas de approval-time como KPI. |
| Deuda de OpenSpec sin uso humano | Editor en Portal con UX clara, plantillas, integración con PRs y comandos slash en chat. |

## Migration Plan

Aditivo sobre Fase 0:
1. Extender Registry: lifecycle, trust levels, eval scores; migración de assets existentes a `lifecycle_state = proposed` (default) con backfill de `trust_level = T0`.
2. Desplegar OpenSpec service y Editor; importar OpenSpecs ya existentes en `openspec/` del repo si se decide, o dejarlos como filesystem (decisión abierta).
3. Desplegar Policy Engine + Approvals con defaults conservadores; activar enforcement progresivo.
4. Desplegar Alfred + RAG + MCPs; iniciar con Skills `create-user-stories`, `scaffold-service`, `generate-test-cases`.
5. Activar Langfuse y guardrails; baseline de métricas.

**Rollback**: feature flags por capability; el lifecycle extendido del Registry mantiene compatibilidad con clients de Fase 0 (estados nuevos opcionales en client). Permisos delegados pueden revocarse en bloque por SDLC Team.

## Open Questions

1. ¿OpenSpec service usa el filesystem `openspec/` del repo Forge como source-of-truth o lleva una BD propia con sync? Decisión inicial: filesystem como source of truth con índice en Postgres para queries.
2. ¿LLM-as-judge en evals desde Fase 1 o postergar a Fase 5? Propuesta: deterministas en Fase 1, LLM-as-judge desde Fase 5.
3. Política definitiva de `expiration` por defecto en delegated permissions (¿30/60/90 días?) — pendiente de Security.
4. Definición exacta de umbrales de eval por trust level — pendiente de evaluación piloto.
