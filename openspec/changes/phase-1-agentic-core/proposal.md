## Why

Con la base de Fase 0 lista (tenancy, IAM, audit, eventos, persistencia, observabilidad, LiteLLM y Registry mínimo), Fase 1 hace **operativo a Alfred** y convierte el Registry mínimo en un **Registry productivo con lifecycle, trust levels y evals**, e introduce **OpenSpec como contrato vivo**, **policies/approvals** y los **MCPs/Skills/Prompts** iniciales. Esta fase entrega el primer valor agéntico end-to-end.

## What Changes

- **NUEVO**: Servicio **Alfred** (Python + FastAPI) con loop razonamiento/acción, decision log, tool execution, llamadas a LiteLLM y delegación a agentes especializados.
- **NUEVO**: Pipeline **RAG sobre Milvus**: ingesta de OpenSpecs, runbooks, ADRs, documentación, repos, PRs, workflows, assets, postmortems y políticas, respetando aislamiento por Workspace y data classification.
- **NUEVO**: **OpenSpec service** con CRUD completo, modelo de datos según spec (intent, requirements, autonomy_policy, decision_log, linked_artifacts, audit), endpoints para Alfred y humanos.
- **NUEVO**: **Editor de OpenSpec** en el Portal (creación, edición, versionado, comparación, links a Jira/GitHub/Confluence).
- **NUEVO**: **Engine de policies/approvals**: evaluación por Workspace/OpenSpec/asset/env/criticidad y **Approvals Inbox** en el Portal.
- **NUEVO**: **Permisos delegados** de Alfred — modelo de grants explícitos, scoped, auditables y revocables; UI de concesión/revocación.
- **NUEVO**: **MCP base SDK** (Python) y MCP servers iniciales: GitHub, Jira, Confluence, OpenSpec.
- **NUEVO**: Primeras **Skills** de referencia (≥3): `create-user-stories`, `scaffold-service`, `generate-test-cases`, con eval básica.
- **NUEVO**: **Prompt Template service** con versionado, variables, ejemplos, modelo recomendado, eval suite y guardrails.
- **NUEVO**: **Trust levels T0–T5** en el Registry y enforcement de aprobaciones por nivel.
- **NUEVO**: **Guardrails** básicos (separación system vs context, sanitización RAG, allowlists, schema validation) + métricas de guardrail-trip.
- **NUEVO**: Integración **Langfuse** (o equivalente) para AI observability, con redacción según política.
- **MODIFICADO**: **Lifecycle del Registry** completo (proposed → in_review → approved → deprecated → retired) reemplazando el comportamiento de Fase 0 que solo aceptaba `proposed`.
- **Criterio de salida (E2E)**: Alfred crea/edita un OpenSpec e invoca al menos un MCP, una Skill y un Prompt Template aprobados con audit/telemetría completos.

## Capabilities

### New Capabilities

- `alfred-control-plane`: agente principal Alfred, autonomía por defecto, tool execution, delegación, decision log, RAG sobre Milvus, integración con LiteLLM.
- `openspec-backbone`: contrato vivo de intención/requerimientos/decisiones, editable por Alfred y humanos, con trazabilidad bidireccional.
- `policies-and-approvals`: motor de evaluación de políticas y aprobaciones configurables por Workspace/OpenSpec/asset/env/criticidad, con Approvals Inbox.
- `delegated-permissions`: grants explícitos, scoped, auditables y revocables para Alfred sobre Workspaces, repos, ambientes y proyectos federados.
- `mcp-and-skills`: MCP base SDK y MCP servers iniciales (GitHub, Jira, Confluence, OpenSpec); primeras Skills de referencia con eval.
- `prompt-template-service`: gestión versionada de prompt templates con guardrails y eval suite.
- `agentic-guardrails`: separación system/context, sanitización RAG, allowlists para tools sensibles, schema validation y métricas de guardrail-trip.
- `ai-observability`: integración con Langfuse para trazas/evals/costos con redacción por data classification.

### Modified Capabilities

- `ai-asset-registry-minimal`: extender el Registry mínimo a **lifecycle completo** (proposed → in_review → approved → deprecated → retired), enforcement de **trust levels T0–T5** y **eval scores** como precondición de aprobación.

## Impact

- **Servicios nuevos**: `services/alfred/` (FastAPI), `services/openspec/` (Go o Python), `services/policy-engine/` (Go), `services/prompt-registry/` (Python), `services/mcp/{github,jira,confluence,openspec}/` (Python), `services/eval-harness/` (Python). Portal: módulos OpenSpec editor, Approvals Inbox, Asset detail (lifecycle/trust/eval), Alfred Console (chat).
- **Datos**: nuevas tablas `openspec`, `openspec_decision_log`, `openspec_link`, `policy`, `approval_request`, `approval_decision`, `delegated_permission`, `prompt_template`, `prompt_version`, `eval_run`, `eval_score`. Colecciones Milvus por Workspace para RAG.
- **Eventos**: `openspec.*`, `approval.*`, `permission.granted.v1`/`permission.revoked.v1`, `asset.lifecycle.transitioned.v1`, `eval.run.completed.v1`, `guardrail.trip.v1`.
- **Integraciones**: Jira, Confluence, OpenSpec MCP, Langfuse, Vertex AI / proveedores aprobados via LiteLLM.
- **Riesgos clave**: prompt injection desde fuentes ingeridas, sobre-ejecución de Alfred, calidad de evals iniciales, costos LLM sin caching maduro.
- **Out of scope**: onboarding de apps con scaffolding (Fase 2), runtimes de despliegue (Fase 3), capabilities por fase del SDLC (Fase 4), editor visual de workflows y marketplace (Fase 5), self-healing (Fase 6).
