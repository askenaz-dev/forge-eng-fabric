# Tasks — Phase 6: Autonomous Ops

## 1. Detection layer

- [ ] 1.1 Crear `services/incident-detection/` (Go) con webhook endpoints para Prometheus/Cloud Monitoring/Loki.
- [ ] 1.2 Normalizador a `incident.detected.v1` con dedup window 5 min y correlation por `service+env+signature_hash`.
- [ ] 1.3 Ingestores de eventos internos: `slo.burn-rate.fast.v1`, `cost.spike.v1`, `eval.regression.detected.v1`, `iac.drift.detected.v1`, `deployment.failed.v1`.
- [ ] 1.4 Endpoint `POST /v1/incidents/declare` para declaración manual.
- [ ] 1.5 Modelo de datos: `incident`, `incident_event`.

## 2. Healing action catalog

- [ ] 2.1 Tipo de asset `healing_action` en Registry con schema y eval suite obligatoria.
- [ ] 2.2 Implementar 5 actions iniciales: `restart-pod`, `scale-up`, `rollback-deploy`, `increase-rate-limit`, `refresh-cache`.
- [ ] 2.3 Cada action mapeada a un workflow Fase 5 (`wf-restart-pod`, etc.).
- [ ] 2.4 Eval datasets golden por action.
- [ ] 2.5 Promotion flow definido (D6.10) implementado en `services/healing-engine/`.

## 3. Diagnosis pipeline

- [ ] 3.1 Crear `services/diagnosis/` (Python) con orquestador de etapas.
- [ ] 3.2 Etapa context-gather: Prometheus queries, Loki logs, Tempo traces, OpenSpec, runbooks, KB similar, evals, FinOps.
- [ ] 3.3 Prompt template versionado `diagnose-incident@1.0.0` con citation enforcement.
- [ ] 3.4 Hypothesis generation y ranking.
- [ ] 3.5 Output `diagnosis_report` persistido y emitido como `incident.diagnosed.v1`.
- [ ] 3.6 Latency target p95 ≤ 60s.

## 4. Healing engine

- [ ] 4.1 Crear `services/healing-engine/` (Go).
- [ ] 4.2 Modelo `healing_envelope` y endpoints CRUD.
- [ ] 4.3 Decision algorithm: input incident → consulta envelope → elige action → elige nivel → ejecuta o pausa.
- [ ] 4.4 L3 path: crear approval Inbox entry y wait signal.
- [ ] 4.5 L4 path: ejecutar workflow + audit + notify.
- [ ] 4.6 L5 path: ejecutar workflow + verify + auto-rollback en falla.
- [ ] 4.7 Kill switch global y por Workspace con TTL cache 30s.
- [ ] 4.8 Eventos `healing.triggered.v1`, `healing.level_decided.v1`, `healing.executed.v1`, `healing.rolled_back.v1`, `healing.escalated.v1`.

## 5. Incidents KB en Milvus

- [ ] 5.1 Crear `services/incidents-kb/` (Python).
- [ ] 5.2 Vectorización post-resolution (resumen + síntomas + root cause).
- [ ] 5.3 Colección Milvus `incidents-kb-{tenant}` con metadata.
- [ ] 5.4 API `POST /v1/kb/incidents/similar` para diagnosis pipeline.
- [ ] 5.5 Cron clustering job identifica recurrentes → emite `incident.recurrent.detected.v1`.

## 6. Postmortem generator

- [ ] 6.1 Crear `services/postmortem/` (Python).
- [ ] 6.2 Trigger en `incident.resolved.v1`.
- [ ] 6.3 Template estructurado markdown.
- [ ] 6.4 Generación con LLM (prompt versionado `generate-postmortem@1.0.0`).
- [ ] 6.5 Publicación en Confluence MCP (Fase 4) y link en OpenSpec.
- [ ] 6.6 Auto-creación de issues Jira para action items.
- [ ] 6.7 Eval suite con criterios de calidad (secciones presentes, citas, owners en action items).
- [ ] 6.8 Eventos `postmortem.generated.v1`, `postmortem.published.v1`.

## 7. Evolution loop

- [ ] 7.1 Crear `services/evolution/` (Go).
- [ ] 7.2 Skill `derive-openspec-evolution` con prompt versionado.
- [ ] 7.3 Generación de OpenSpec change proposal con marker `source=autonomous-loop`.
- [ ] 7.4 Persistencia en `evolution_proposal` y publicación en "Evolution Inbox" del Portal.
- [ ] 7.5 Workflow de aceptación: human review → conversión a change normal OpenSpec.
- [ ] 7.6 Evento `evolution.openspec_proposal.v1`.

## 8. FinOps advisor

- [ ] 8.1 Crear `services/finops-advisor/` (Python).
- [ ] 8.2 Cron diario consulta BigQuery cost data.
- [ ] 8.3 Pattern detectors: idle resources, oversized, expensive LLM skills, cacheable prompts.
- [ ] 8.4 Skill `propose-cost-reduction` genera PRs con expected savings.
- [ ] 8.5 PRs respetan gates Fase 2 + Fase 4.
- [ ] 8.6 Modelo `finops_recommendation`, evento `finops.recommendation.created.v1`.

## 9. Policies extensions

- [ ] 9.1 Templates `autonomy-envelope`, `kill-switch`, `level-by-env`, `level-by-criticality`, `require-reversible-for-l5`.
- [ ] 9.2 Promotion-of-action policy con prerequisites D6.10.

## 10. AI observability extensions

- [ ] 10.1 Métricas: `healing_invocations_by_level`, `healing_success_rate_by_level`, `mttr_seconds`, `incident_count_by_severity`, `auto_rollback_rate`, `kill_switch_activation_count`.
- [ ] 10.2 Funnel detection→diagnosis→action→resolution.
- [ ] 10.3 Dashboards Grafana globales y por Workspace.

## 11. OpenSpec backbone extensions

- [ ] 11.1 Aceptar changes con marker `source=autonomous-loop` con badge UI distinto.
- [ ] 11.2 Workflow específico de revisión humana (require human approval antes de aceptar).
- [ ] 11.3 Métricas de evolution loop: propuestas creadas, aceptadas, ratio.

## 12. Portal — UI

- [ ] 12.1 Módulo "Incidents": lista, timeline, diagnosis, healing decisions, postmortem link.
- [ ] 12.2 Módulo "Evolution Inbox" con badge `autonomous-loop`.
- [ ] 12.3 Módulo "FinOps Recommendations" con savings y PRs links.
- [ ] 12.4 Vista Kill Switch con audit log y estado actual.
- [ ] 12.5 Tests E2E con Playwright sobre escenarios sintéticos.

## 13. Validación y sign-off

- [ ] 13.1 Inyectar 5 incidentes sintéticos cubriendo cada nivel L1..L5 (con flag `synthetic=true`).
- [ ] 13.2 Verificar diagnosis con citas verificables, healing ejecutado correctamente, postmortem auto-generado, evolution proposal creado.
- [ ] 13.3 Probar kill switch en vivo bajo incidente activo.
- [ ] 13.4 Probar promoción de una action L3→L4 en `dev`.
- [ ] 13.5 Sign-off Platform + Security + Tenant admin + 2 Workspaces piloto.
- [ ] 13.6 Documentación: `docs/healing/levels.md`, `docs/healing/envelopes.md`, `docs/postmortems/`, `docs/evolution-loop/`, `docs/finops/recommendations.md`.

## 14. Cierre del bootstrap

- [ ] 14.1 Workshop de retrospectiva con todos los stakeholders.
- [ ] 14.2 Roadmap post-bootstrap como OpenSpecs nuevos (no parte de esta change).
- [ ] 14.3 Anuncio interno: Forge entra en operación productiva multi-tenant con autonomous ops.
