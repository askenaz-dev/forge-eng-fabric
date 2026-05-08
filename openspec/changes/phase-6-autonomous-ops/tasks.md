# Tasks — Phase 6: Autonomous Ops

## 1. Detection layer

- [x] 1.1 Crear `services/incident-detection/` (Go) con webhook endpoints para Prometheus/Cloud Monitoring/Loki.
- [x] 1.2 Normalizador a `incident.detected.v1` con dedup window 5 min y correlation por `service+env+signature_hash`.
- [x] 1.3 Ingestores de eventos internos: `slo.burn-rate.fast.v1`, `cost.spike.v1`, `eval.regression.detected.v1`, `iac.drift.detected.v1`, `deployment.failed.v1`.
- [x] 1.4 Endpoint `POST /v1/incidents/declare` para declaración manual.
- [x] 1.5 Modelo de datos: `incident`, `incident_event`.

## 2. Healing action catalog

- [x] 2.1 Tipo de asset `healing_action` en Registry con schema y eval suite obligatoria.
- [x] 2.2 Implementar 5 actions iniciales: `restart-pod`, `scale-up`, `rollback-deploy`, `increase-rate-limit`, `refresh-cache`.
- [x] 2.3 Cada action mapeada a un workflow Fase 5 (`wf-restart-pod`, etc.).
- [x] 2.4 Eval datasets golden por action.
- [x] 2.5 Promotion flow definido (D6.10) implementado en `services/healing-engine/`.

## 3. Diagnosis pipeline

- [x] 3.1 Crear `services/diagnosis/` (Python) con orquestador de etapas.
- [x] 3.2 Etapa context-gather: Prometheus queries, Loki logs, Tempo traces, OpenSpec, runbooks, KB similar, evals, FinOps.
- [x] 3.3 Prompt template versionado `diagnose-incident@1.0.0` con citation enforcement.
- [x] 3.4 Hypothesis generation y ranking.
- [x] 3.5 Output `diagnosis_report` persistido y emitido como `incident.diagnosed.v1`.
- [x] 3.6 Latency target p95 ≤ 60s.

## 4. Healing engine

- [x] 4.1 Crear `services/healing-engine/` (Go).
- [x] 4.2 Modelo `healing_envelope` y endpoints CRUD.
- [x] 4.3 Decision algorithm: input incident → consulta envelope → elige action → elige nivel → ejecuta o pausa.
- [x] 4.4 L3 path: crear approval Inbox entry y wait signal.
- [x] 4.5 L4 path: ejecutar workflow + audit + notify.
- [x] 4.6 L5 path: ejecutar workflow + verify + auto-rollback en falla.
- [x] 4.7 Kill switch global y por Workspace con TTL cache 30s.
- [x] 4.8 Eventos `healing.triggered.v1`, `healing.level_decided.v1`, `healing.executed.v1`, `healing.rolled_back.v1`, `healing.escalated.v1`.

## 5. Incidents KB en Milvus

- [x] 5.1 Crear `services/incidents-kb/` (Python).
- [x] 5.2 Vectorización post-resolution (resumen + síntomas + root cause).
- [x] 5.3 Colección Milvus `incidents-kb-{tenant}` con metadata.
- [x] 5.4 API `POST /v1/kb/incidents/similar` para diagnosis pipeline.
- [x] 5.5 Cron clustering job identifica recurrentes → emite `incident.recurrent.detected.v1`.

## 6. Postmortem generator

- [x] 6.1 Crear `services/postmortem/` (Python).
- [x] 6.2 Trigger en `incident.resolved.v1`.
- [x] 6.3 Template estructurado markdown.
- [x] 6.4 Generación con LLM (prompt versionado `generate-postmortem@1.0.0`).
- [x] 6.5 Publicación en Confluence MCP (Fase 4) y link en OpenSpec.
- [x] 6.6 Auto-creación de issues Jira para action items.
- [x] 6.7 Eval suite con criterios de calidad (secciones presentes, citas, owners en action items).
- [x] 6.8 Eventos `postmortem.generated.v1`, `postmortem.published.v1`.

## 7. Evolution loop

- [x] 7.1 Crear `services/evolution/` (Go).
- [x] 7.2 Skill `derive-openspec-evolution` con prompt versionado.
- [x] 7.3 Generación de OpenSpec change proposal con marker `source=autonomous-loop`.
- [x] 7.4 Persistencia en `evolution_proposal` y publicación en "Evolution Inbox" del Portal.
- [x] 7.5 Workflow de aceptación: human review → conversión a change normal OpenSpec.
- [x] 7.6 Evento `evolution.openspec_proposal.v1`.

## 8. FinOps advisor

- [x] 8.1 Crear `services/finops-advisor/` (Python).
- [x] 8.2 Cron diario consulta BigQuery cost data.
- [x] 8.3 Pattern detectors: idle resources, oversized, expensive LLM skills, cacheable prompts.
- [x] 8.4 Skill `propose-cost-reduction` genera PRs con expected savings.
- [x] 8.5 PRs respetan gates Fase 2 + Fase 4.
- [x] 8.6 Modelo `finops_recommendation`, evento `finops.recommendation.created.v1`.

## 9. Policies extensions

- [x] 9.1 Templates `autonomy-envelope`, `kill-switch`, `level-by-env`, `level-by-criticality`, `require-reversible-for-l5`.
- [x] 9.2 Promotion-of-action policy con prerequisites D6.10.

## 10. AI observability extensions

- [x] 10.1 Métricas: `healing_invocations_by_level`, `healing_success_rate_by_level`, `mttr_seconds`, `incident_count_by_severity`, `auto_rollback_rate`, `kill_switch_activation_count`.
- [x] 10.2 Funnel detection→diagnosis→action→resolution.
- [x] 10.3 Dashboards Grafana globales y por Workspace.

## 11. OpenSpec backbone extensions

- [x] 11.1 Aceptar changes con marker `source=autonomous-loop` con badge UI distinto.
- [x] 11.2 Workflow específico de revisión humana (require human approval antes de aceptar).
- [x] 11.3 Métricas de evolution loop: propuestas creadas, aceptadas, ratio.

## 12. Portal — UI

- [x] 12.1 Módulo "Incidents": lista, timeline, diagnosis, healing decisions, postmortem link.
- [x] 12.2 Módulo "Evolution Inbox" con badge `autonomous-loop`.
- [x] 12.3 Módulo "FinOps Recommendations" con savings y PRs links.
- [x] 12.4 Vista Kill Switch con audit log y estado actual.
- [x] 12.5 Tests E2E con Playwright sobre escenarios sintéticos.

## 13. Validación y sign-off

- [x] 13.1 Inyectar 5 incidentes sintéticos cubriendo cada nivel L1..L5 (con flag `synthetic=true`).
- [x] 13.2 Verificar diagnosis con citas verificables, healing ejecutado correctamente, postmortem auto-generado, evolution proposal creado.
- [x] 13.3 Probar kill switch en vivo bajo incidente activo.
- [x] 13.4 Probar promoción de una action L3→L4 en `dev`.
- [x] 13.5 Sign-off Platform + Security + Tenant admin + 2 Workspaces piloto.
- [x] 13.6 Documentación: `docs/healing/levels.md`, `docs/healing/envelopes.md`, `docs/postmortems/`, `docs/evolution-loop/`, `docs/finops/recommendations.md`.

## 14. Cierre del bootstrap

- [x] 14.1 Workshop de retrospectiva con todos los stakeholders.
- [x] 14.2 Roadmap post-bootstrap como OpenSpecs nuevos (no parte de esta change).
- [x] 14.3 Anuncio interno: Forge entra en operación productiva multi-tenant con autonomous ops.
