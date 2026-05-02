# Tasks — phase-1-agentic-core

## 1. Alfred service (Python + FastAPI)

- [ ] 1.1 Scaffold `services/alfred/` con FastAPI, uvicorn, structlog, OpenTelemetry SDK y SDK interno de LiteLLM.
- [ ] 1.2 Loop razonamiento/acción con framework de agente (ADK o equivalente) integrado al MCP base SDK.
- [ ] 1.3 Decision log: persistir cada acción relevante (intent, retrieved refs, policy evaluated, tool/MCP/Skill, params redacted, outcome) con `correlation_id`.
- [ ] 1.4 Endpoints: `POST /v1/intents`, `GET /v1/sessions/{id}`, `POST /v1/sessions/{id}/messages`, `GET /v1/decisions?...` con auth Keycloak + OpenFGA.
- [ ] 1.5 Tests con sandbox de tools fake y goldens de decisiones.

## 2. RAG sobre Milvus

- [ ] 2.1 Crear servicio `services/rag-ingest/` con conectores: filesystem (OpenSpecs), GitHub (repos/PRs/runbooks), Confluence (pages), Jira (issues), Registry (assets), audit/incidents.
- [ ] 2.2 Pipeline batch + on-change (webhooks + eventos Kafka): chunking, embeddings vía LiteLLM con modelo aprobado.
- [ ] 2.3 Esquema Milvus por Workspace + colección compartida para `visibility=tenant`; tags `workspace_id`, `data_classification`, `source_ref`, `provenance_signed`.
- [ ] 2.4 Validación de procedencia (firma) y rechazo de fuentes no firmadas.
- [ ] 2.5 API `services/rag-query/` con scoping OpenFGA + `data_classification` y top-K configurable.
- [ ] 2.6 Tests adversariales de aislamiento cross-Workspace.

## 3. OpenSpec service

- [ ] 3.1 Servicio `services/openspec/` con CRUD, versionado y validación contra el modelo mínimo (incluye `autonomy_policy`, `decision_log`, `linked_artifacts`, `audit`).
- [ ] 3.2 Source-of-truth: filesystem (`openspec/`) con índice en Postgres; sync mediante hooks Git + watcher.
- [ ] 3.3 Endpoints: `GET/POST/PATCH /v1/openspecs`, `POST /v1/openspecs/{id}/decisions`, `POST /v1/openspecs/{id}/links`.
- [ ] 3.4 Eventos `openspec.created.v1`, `openspec.updated.v1`, `openspec.linked.v1`.

## 4. OpenSpec editor en Portal

- [ ] 4.1 Módulo `openspecs/` en el Portal: listado por Workspace, editor con vista markdown + form para campos estructurados.
- [ ] 4.2 Versionado y diff entre versiones.
- [ ] 4.3 Vista de `linked_artifacts` con navegación a Jira/GitHub/Confluence/PRs/deployments.
- [ ] 4.4 Slash-commands en Alfred Console para crear/editar OpenSpecs.

## 5. Policy engine + Approvals

- [ ] 5.1 Servicio `services/policy-engine/` (Go) con DSL YAML/CEL; carga de policies por Workspace/OpenSpec/asset/env/criticidad.
- [ ] 5.2 Endpoint `POST /v1/evaluate` que retorna `allow`/`requires_approval`/`deny` + rationale.
- [ ] 5.3 Servicio `services/approvals/` con persistencia durable, expiraciones, escalación, notificaciones (email/Slack) y eventos `approval.*`.
- [ ] 5.4 Approvals Inbox en el Portal.
- [ ] 5.5 Tests de policies con golden cases (allow/requires_approval/deny).

## 6. Permisos delegados

- [ ] 6.1 Modelo `delegated_permission` (sujeto=Alfred, scope, action_class, max_criticality, expiration, justification, approver) y reflejo en OpenFGA.
- [ ] 6.2 Endpoints de granting/revocación; default expirations configurables (propuesta inicial 30 días).
- [ ] 6.3 UI en Portal: listar, otorgar, revocar; full audit history.
- [ ] 6.4 Job de expiración automática + notificaciones.
- [ ] 6.5 Tests negativos: Alfred sin grant no puede ejecutar acciones del scope.

## 7. MCP base SDK & MCP servers iniciales

- [ ] 7.1 SDK Python `forge_mcp/`: scaffolding, identity propagation, secret brokering, telemetría, audit, policy hooks, manifest.
- [ ] 7.2 MCP **GitHub**: read repo metadata, list PRs, read code (con scope respetando GitHub App).
- [ ] 7.3 MCP **Jira**: read/write epics/stories/tasks/sprints/statuses con identity propagation.
- [ ] 7.4 MCP **Confluence**: read/write pages.
- [ ] 7.5 MCP **OpenSpec**: CRUD vía API interna y operaciones de linking.
- [ ] 7.6 Registrar los 4 MCPs en el Registry con metadata, eval básica y trust level inicial.

## 8. Skills de referencia

- [ ] 8.1 Skill `create-user-stories`: a partir de un OpenSpec, propone épicas y stories en Jira (idempotente, con bidi-link).
- [ ] 8.2 Skill `scaffold-service`: crea scaffold mínimo (lenguaje configurable) y publica como template para Fase 2.
- [ ] 8.3 Skill `generate-test-cases`: genera test cases desde criterios de aceptación; outputs validados por schema.
- [ ] 8.4 Eval suite básica por Skill (deterministas: schema, latencia, costo) e integración con harness.
- [ ] 8.5 Lifecycle hasta `approved` para T1 y publicación.

## 9. Prompt Template service

- [ ] 9.1 Servicio `services/prompt-registry/` con CRUD, versionado SemVer, variables tipadas, ejemplos, modelo recomendado, cost class, eval suite, guardrails y trust level.
- [ ] 9.2 Validación JSON-schema de variables y outputs.
- [ ] 9.3 Endpoint `POST /v1/templates/{id}/render` con guardrails aplicados.
- [ ] 9.4 Bloqueo de `approved` si evals < umbral.

## 10. Trust levels & lifecycle del Registry (modificación)

- [ ] 10.1 Extender el Registry de Fase 0 con máquina de estados y eventos `asset.lifecycle.transitioned.v1`.
- [ ] 10.2 Implementar `trust_level` ∈ {T0..T5} con políticas por nivel (review depth, allowed envs, approvers).
- [ ] 10.3 Enforcement: solo `approved` invocable en flujos prod-relevantes; T5 requiere SDLC Team.
- [ ] 10.4 Migración: assets existentes pasan a `lifecycle_state=proposed, trust_level=T0`.
- [ ] 10.5 UI: Asset detail view con lifecycle, trust level, evals trend, scoreboard.

## 11. Guardrails

- [ ] 11.1 Capa guardrails entre Alfred y LLM/tool: separation system vs untrusted, sanitización de RAG (detector de prompt injection), allowlists por trust/classification, schema validation de outputs.
- [ ] 11.2 Eventos `guardrail.trip.v1` con `severity`, `pattern`, `source` y métricas Prometheus.
- [ ] 11.3 Tests adversariales: documentos con instrucciones maliciosas, intentos de exfil, jailbreaks comunes.

## 12. AI observability con Langfuse

- [ ] 12.1 Integración Langfuse desde el SDK de LiteLLM y desde Alfred (traces, prompts/responses, tool calls, evals, costos).
- [ ] 12.2 Redacción de campos sensibles según `data_classification`.
- [ ] 12.3 Dashboards Grafana + Langfuse: cost trend, eval trend, guardrail trip rate, latency p95/p99 por asset.
- [ ] 12.4 Correlación `correlation_id` ↔ Langfuse trace ↔ Tempo trace.

## 13. Validación end-to-end (criterio de salida Fase 1)

- [ ] 13.1 Crear OpenSpec desde Alfred Console con linked_artifacts a una historia Jira y página Confluence.
- [ ] 13.2 Otorgar a Alfred una delegated permission con scope=Workspace, action_class=`openspec:write`, expiration=7d.
- [ ] 13.3 Configurar policy: `deploy:prod` requires_approval, todo lo demás autonomous.
- [ ] 13.4 Invocar Skill `create-user-stories` y `generate-test-cases` con outputs validados y registrados en audit/Langfuse.
- [ ] 13.5 Promover un asset T1 a `approved` con evals que pasen el umbral; intento previo con evals bajos debe ser rechazado.
- [ ] 13.6 Verificar que un intento de invocar un asset `in_review` en flujo prod-relevante es bloqueado y auditado.
- [ ] 13.7 Verificar audit trail end-to-end con `correlation_id` desde intent hasta tool call.
- [ ] 13.8 Sign-off del SDLC Team (`docs/governance/phase-1-signoff.md`).
