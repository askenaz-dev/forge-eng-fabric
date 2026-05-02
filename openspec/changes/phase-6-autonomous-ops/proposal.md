## Why

Con SDLC, deploys, workflows y observability per-asset (Fases 0–5), Forge debe **cerrar el ciclo de vida operativo de manera autónoma y gobernada**: detectar incidentes/anomalías, diagnosticar citando runbook + OpenSpec + telemetría + KB, ejecutar healing en 5 niveles graduales (Notify → Suggest → Act-with-approval → Act-autonomously → Act-and-rollback), generar postmortems, y **cerrar el loop alimentando OpenSpec con aprendizajes**. Sin esto, la organización paga el costo de adopción agéntica sin recoger el valor operativo más alto: reducir MTTR, evitar repetir incidentes y mejorar continuamente sin intervención manual.

## What Changes

- **NUEVO**: **Healing engine** con **5 niveles** configurables por capability/asset/env/criticidad:
  - **L1 — Notify**: detecta y avisa sin proponer.
  - **L2 — Suggest**: propone acción concreta para review humano.
  - **L3 — Act with approval**: ejecuta tras approval del Inbox (TTL corto).
  - **L4 — Act autonomously**: ejecuta sin approval dentro de envelope predefinido (per-runbook, per-env), con audit + notify.
  - **L5 — Act and rollback**: ejecuta y verifica; rollback automático si la acción no resuelve.
- **NUEVO**: **Diagnosis pipeline** que integra runbooks (Confluence/repo), OpenSpec del Workspace/asset, telemetría (Prometheus/Loki/Tempo), incidentes históricos (KB), evals, deploy history y FinOps.
- **NUEVO**: **Detector layer**: integración con Prometheus alerts, Cloud Monitoring, Loki query alerts, custom CloudEvents (`incident.*`, `slo.burn-rate.*`, `cost.spike.*`, `eval.regression.*`).
- **NUEVO**: **Action catalog gobernado**: cada healing action es un asset (`type=healing_action`) con runbook, parámetros, riesgo, niveles permitidos por env, eval suite y telemetría.
- **NUEVO**: **Postmortem generator**: tras cada incidente cerrado, Alfred genera postmortem estructurado (timeline, impact, root cause, remediation, lessons) y lo publica en Confluence + OpenSpec del asset afectado.
- **NUEVO**: **Evolution loop**: lessons learned se transforman en propuestas de cambio de OpenSpec (acceptance criteria nuevos, runbook updates, SLO ajustados, gates añadidos a Fase 4).
- **NUEVO**: **FinOps reports & recommendations**: Alfred analiza tendencias y propone cost-reduction PRs (downsize, schedule-based scaling, model downgrade, prompt simplification) con expected savings.
- **NUEVO**: **Knowledge Base de incidentes**: indexada en Milvus, con clusterización por similaridad para detectar incidentes recurrentes.
- **NUEVO**: Eventos `incident.*`, `healing.*` (`triggered`, `level_decided`, `executed`, `rolled_back`, `escalated`), `postmortem.*` (`generated`, `published`), `kb.incident.indexed.v1`, `evolution.openspec_proposal.v1`.
- **MODIFICADO**: `policies-and-approvals` — añadir templates de envelope autónomo (`autonomy-envelope`), límites por env/criticidad/horario, kill-switch global.
- **MODIFICADO**: `ai-observability` — extender con métricas de healing (success rate por nivel, MTTR por capability), funnel detection→action.
- **MODIFICADO**: `openspec-backbone` — aceptar propuestas de evolución generadas por Alfred como changes con marker `source=autonomous-loop`.
- **Criterio de salida (E2E)**: un incidente sintético dispara el flujo: detect → diagnose con citas → healing L3 (con approval) → execute → verify → postmortem auto-generado → propuesta de evolución de OpenSpec lista para revisión humana; toda la cadena es trazable y auditada.

## Capabilities

### New Capabilities

- `healing-engine`: motor de 5 niveles con selección por policy y envelope.
- `diagnosis-pipeline`: composición de evidencia (runbook + OpenSpec + telemetría + KB + history) con citas.
- `incident-detection`: integración con detectores externos e internos y normalización a `incident.*` events.
- `healing-action-catalog`: assets `type=healing_action` con runbook, riesgo, eval, niveles permitidos.
- `postmortem-generator`: generación estructurada y publicación en Confluence + OpenSpec.
- `evolution-loop`: aprendizajes → propuestas de change OpenSpec con marker `autonomous-loop`.
- `finops-recommendations`: análisis de tendencias y PRs de cost-reduction con savings esperados.
- `incidents-kb`: índice Milvus de incidentes con similaridad y clustering.

### Modified Capabilities

- `policies-and-approvals`: templates `autonomy-envelope`, `kill-switch`, `level-by-env`, `level-by-criticality`.
- `ai-observability`: métricas de healing (success por nivel, MTTR), funnel detect→act.
- `openspec-backbone`: aceptar propuestas con marker `source=autonomous-loop` y workflow de revisión humana.

## Impact

- **Servicios nuevos**: `services/healing-engine/` (Go), `services/diagnosis/` (Python, RAG-heavy), `services/postmortem/` (Python), `services/evolution/` (Go), `services/finops-advisor/` (Python), `services/incidents-kb/` (Python), `services/incident-detection/` (Go).
- **Datos**: tablas `incident`, `incident_event`, `healing_action_invocation`, `healing_envelope`, `postmortem`, `evolution_proposal`, `finops_recommendation`. Colecciones Milvus por Tenant para KB.
- **Eventos**: `incident.detected.v1`, `incident.diagnosed.v1`, `healing.triggered.v1`, `healing.level_decided.v1`, `healing.executed.v1`, `healing.rolled_back.v1`, `healing.escalated.v1`, `postmortem.generated.v1`, `postmortem.published.v1`, `evolution.openspec_proposal.v1`, `finops.recommendation.created.v1`.
- **Integraciones**: Prometheus alertmanager, Cloud Monitoring, Loki, PagerDuty/Opsgenie (opcional), Slack/Teams (notifications), Confluence (postmortem publishing), GitHub (evolution PRs), Cloud Billing.
- **Portal**: módulo "Incidents" (timeline, diagnosis, healing decisions, postmortem), módulo "Evolution Inbox" (propuestas de change pending review), módulo "FinOps Recommendations".
- **Riesgos**: acción autónoma incorrecta (mitigado con envelopes estrictos por env, kill-switch global, rollback automático L5, dry-run obligatorio antes de promover una action a L4); falsa alarma de detección (mitigado con dedup + correlation + time-windows); postmortems superficiales (mitigado con eval suite específica y human review).
- **Out of scope**: ya nada — Fase 6 cierra el roadmap de bootstrap; futuras fases serán mejoras incrementales gobernadas via OpenSpec.
