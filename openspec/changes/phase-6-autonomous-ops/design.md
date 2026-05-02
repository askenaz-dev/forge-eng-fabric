## Context

Fase 6 es la culminación del roadmap de bootstrap: convertir a Forge en una plataforma que **opera, aprende y mejora** con mínima intervención humana, siempre dentro de un envelope auditable y revertible. Asume todas las fases previas: tenancy, IAM, observability, Alfred + RAG, OpenSpec, policies, MCPs, onboarding, deploys, SDLC orchestration, workflows y eval avanzada.

## Constraints

- **Autonomía gradual**: el default de un asset/env nuevo es L1 o L2; L3+ requiere policy explícita y eval pass del action.
- **L4/L5 nunca por defecto en prod**: requieren approval Tenant + eval ≥ threshold + dry-run histórico positivo.
- **Kill switch**: existe un mecanismo global y por Workspace/env que detiene toda acción autónoma inmediatamente.
- **Reversibilidad**: cada healing action en L4/L5 declara `reversible: true` o NO puede subir a esos niveles.
- **Auditabilidad total**: cada decisión incluye qué evidencia se usó, qué citas, qué nivel se eligió, por qué.
- **Sin bypass de Approvals**: L3 sigue exigiendo approval; el envelope no puede saltar L3 hacia L4 sin policy explícita.

## Decisions

### D6.1 — Modelo de niveles y envelope

```
healing_envelope:
  capability: <id>          # ej. sdlc-devops, observability
  asset_pattern: <regex>    # ej. application/svc-* 
  env: dev|stage|prod
  criticality: low..critical
  default_level: L1..L5
  allowed_levels: [L1, L2, L3]
  time_windows: [...]       # ej. business hours only para L4
  max_actions_per_hour: N
  kill_switch: false
```

El motor consulta el envelope antes de ejecutar; si la acción excede el nivel permitido, degrada al máximo nivel allowed.

### D6.2 — Diagnosis pipeline

Etapas (todas auditables):
1. **Context gather**: extraer asset, env, deploy reciente, OpenSpec(s), runbook(s), KB similar, telemetría (Prom queries, Loki logs, Tempo traces), evals recientes, FinOps.
2. **Hypothesis generation**: LLM via LiteLLM, prompt template versionado `diagnose-incident@vX`.
3. **Citation enforcement**: cada hipótesis MUST citar fuentes (URL/id); hipótesis sin citas son descartadas.
4. **Ranking**: hipótesis ordenadas por confidence (modelo) + match con KB histórico.
5. **Output**: `diagnosis_report` con top-N hipótesis, evidencia, healing actions sugeridas.

Latency target p95 ≤ 60s para incidentes prod.

### D6.3 — Healing action catalog

Cada action es asset `type=healing_action`:
```yaml
id: restart-pod
runbook_url: confluence://...
parameters: [namespace, pod_name]
risk: low
allowed_levels_by_env: { dev: [L1..L5], stage: [L1..L4], prod: [L1..L3] }
reversible: true
eval_suite: ds-restart-pod-eval@1.0.0
implementation: workflow:wf-restart-pod@1.2.0   # delega a un workflow Fase 5
```

Ejecución: el motor invoca el workflow correspondiente; resultado emite `healing.executed.v1`. Si `reversible: true` y verify falla → invoca workflow inverso `wf-restart-pod-rollback`.

### D6.4 — Detection layer

Fuentes:
- Prometheus alertmanager → webhook → normaliza a `incident.detected.v1`.
- Cloud Monitoring policies → idem.
- Loki alert rules.
- CloudEvents internos: `slo.burn-rate.fast.v1`, `cost.spike.v1`, `eval.regression.detected.v1`, `iac.drift.detected.v1` (Fase 3), `deployment.failed.v1` (Fase 3).
- Manual: humans crean `incident.declared.v1` desde Portal.

Dedup window: 5 min; correlación por `service`, `env`, `signature_hash`.

### D6.5 — Postmortem generator

Tras `incident.resolved.v1`:
1. Recupera timeline completo (eventos del bus filtrados por `incident_id`).
2. Reúne diagnosis_report, healing actions invocadas, evals, postmortem template.
3. Genera postmortem estructurado (markdown): summary, impact, timeline, root cause, what went well, what went wrong, action items.
4. Publica en Confluence (Fase 4 MCP) en space del Workspace; copia link al OpenSpec del asset; emite `postmortem.published.v1`.
5. Genera issues Jira para action items con vínculo al postmortem.

Eval suite revisa calidad mínima (presencia de secciones, citas, action items con owners).

### D6.6 — Evolution loop

Tras `postmortem.published.v1`:
1. Skill `derive-openspec-evolution` analiza el postmortem y propone:
   - Nuevos acceptance criteria (a OpenSpec del asset).
   - Updates de runbook.
   - Cambios de SLO/SLI.
   - Nuevos gates (Fase 4) si aplica.
   - Nuevas healing actions para añadir al catálogo.
2. Genera **OpenSpec change proposal** con marker `source=autonomous-loop` y referencia al postmortem.
3. Publica en "Evolution Inbox" del Portal para review humana.
4. Aprobada → la propuesta sigue el flujo normal de OpenSpec changes.

### D6.7 — FinOps recommendations

Cron diario:
1. Consulta BigQuery cost data (Fase 4).
2. Identifica patrones: idle resources, oversized, rampant LLM cost por skill, prompts caros con caching potencial.
3. Skill `propose-cost-reduction` genera PRs con cambios concretos (Terraform downsize, prompt template optimizations, cache TTL adjustments) con expected savings calculado.
4. PRs siguen el flujo normal Fase 2 + gates Fase 4.

### D6.8 — Incidents KB en Milvus

- Cada incidente cerrado se vectoriza (resumen + síntomas + root cause).
- Indexado en colección Tenant `incidents-kb`.
- Diagnosis pipeline consulta similar incidents (top-K, threshold de similitud).
- Clustering periódico identifica incidentes recurrentes → genera `incident.recurrent.detected.v1` para action items.

### D6.9 — Kill switch

- Flag por Workspace y global en config server (con cache TTL 30s).
- Activación instantánea via Portal o API; require role `platform-admin` o `security-approver`.
- Activado → motor degrada todas las acciones a L1 (Notify only); workflows en ejecución continúan pero NO se inician nuevos en L3+.
- Audit completo de activaciones/desactivaciones.

### D6.10 — Promoción de actions a niveles superiores

Para que una `healing_action` pase de L3 a L4 en un env:
1. Eval suite debe pasar consistentemente (>95% en últimos 50 runs).
2. Dry-run histórico: ejecutado en L3 al menos 20 veces con éxito ≥95%.
3. Postmortem-free window: 30 días sin incidente atribuible a la action.
4. Approval por `platform-admin` + `security-approver`.
5. Auditado y emitido `healing.action.promoted.v1`.

## Risks / Trade-offs

- **Acción autónoma incorrecta** en prod → mitigado con envelopes estrictos, dry-run obligatorio para promoción, rollback automático en L5, kill switch.
- **Diagnosis con citas falsas (hallucinations)** → mitigado con citation enforcement (descarta sin cita), KB grounding y eval suite del prompt.
- **Postmortem genérico/superficial** → mitigado con eval suite y human review obligatoria para cierre.
- **Cost-reduction PRs muy agresivos** → mitigado con thresholds de impact (≤Δ% performance) y human approval para `criticality≥high`.
- **Tormenta de incidents synthetic** durante test → mitigado con flag `synthetic=true` que evita ejecución real de healing.

## Migration Plan

1. Implementar `incident-detection` y normalización; ingestar alertas existentes Prometheus/Cloud Monitoring.
2. Implementar `healing-action-catalog` y publicar 5 actions iniciales (restart-pod, scale-up, rollback-deploy, increase-rate-limit, refresh-cache) en L1/L2.
3. Implementar `diagnosis-pipeline` con prompt versionado y citation enforcement.
4. Implementar `healing-engine` con envelopes y kill switch.
5. Implementar `incidents-kb` con vectorización e indexación.
6. Implementar `postmortem-generator` y `evolution-loop`.
7. Implementar `finops-advisor`.
8. Promover una action a L3 en un Workspace piloto; ejecutar incidentes sintéticos.
9. Promover una action a L4 en `dev` con todas las prerequisites.
10. Sign-off Platform + Security + 2 Workspaces piloto + Tenant admin.

## Open Questions

- ¿Permitir L5 en prod alguna vez? — **Decisión inicial**: solo para actions con `reversible: true` Y `critical_blast_radius=false` (ej. cache refresh, log level toggle); cambios destructivos jamás en L5 prod.
- ¿On-call rotation integration con healing? — **Decisión inicial**: notificar pero no asignar automáticamente; el engineer on-call ve la diagnosis al despertar.
- ¿KB pública entre Tenants para incidentes anonimizados? — **Out of scope**; arquitectura permite hacerlo a futuro con consent y anonymization layer.
- ¿Permitir healing actions custom por Workspace o sólo del catálogo Tenant? — **Decisión inicial**: ambos; Workspace-only actions limitadas a L1/L2 hasta certificación Tenant.
