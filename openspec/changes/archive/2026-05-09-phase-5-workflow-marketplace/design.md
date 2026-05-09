## Context

Fase 5 introduce el sustrato para que los Workspaces compongan capacidades agénticas reusables sin pedirlo a Platform Engineering cada vez. Las decisiones definen el modelo del workflow (AST único editor visual + DSL), la ejecución durable, el versionado, el marketplace por Tenant, eval avanzada y observability per-asset. Asume Fase 4 entregando Skills/MCPs registrados y trazabilidad.

## Constraints

- **AST único**: editor visual y DSL YAML producen el mismo AST canónico; ningún flujo se persiste en formato propietario del editor.
- **Inmutabilidad**: una `workflow_version` publicada es inmutable; correcciones requieren nueva versión.
- **Tenancy estricta**: workflows con visibility `private` o `workspace` son inaccesibles fuera de su scope; visibility `tenant` requiere approval Tenant-level; visibility `forge-certified` requiere eval pass + security review.
- **No bypass de LiteLLM**: nodos LLM van por LiteLLM (Fase 0).
- **Durabilidad**: workflows usan Temporal con `durable: true`; ningún state crítico vive en memoria del proceso.
- **Idempotencia de steps**: cada activity Temporal es idempotente o se compensa.

## Decisions

### D5.1 — AST canónico y DSL

```yaml
apiVersion: forge.workflows/v1
kind: Workflow
metadata: { id, name, version, owners, visibility, criticality, openspec_ids }
spec:
  inputs: [{name, type, validations}]
  outputs: [{name, type}]
  steps:
    - id: refine-story
      type: skill
      ref: registry:skill/sdlc-product/refine-user-story@1.2.0
      inputs: { story: $inputs.story }
      retries: { max: 3, backoff: exponential }
      timeout: 60s
    - id: human-approval
      type: human-in-the-loop
      approver_role: product-owner
      timeout: 24h
    - id: open-pr
      type: mcp
      ref: registry:mcp/github@write
      tool: create_pr
      depends_on: [human-approval]
      inputs: { ... }
  on_failure:
    - id: notify
      type: skill
      ref: registry:skill/sdlc-devops/post-incident-note@1.0.0
```

DSL valida contra JSON Schema antes de aceptar; lint reporta unreachable steps, dangling deps, type mismatches.

### D5.2 — Editor visual

Implementado con **React Flow** en el Portal. Cada nodo del editor mapea 1:1 a un step del AST. El editor consume el Registry en tiempo real para listar Skills/MCPs/Prompts disponibles con autocompletar. Validación en vivo (lint) con marcadores en los nodos. Debug mode: ejecutar con `dry_run=true` mostrando entradas/salidas mock por step.

### D5.3 — Workflow runtime sobre Temporal

- Cada Tenant tiene su **Temporal namespace** (aislamiento de colas y task queues).
- Cada step del AST → activity Temporal. Sub-workflows → child workflows.
- Retries declarativos del DSL → `RetryPolicy` Temporal.
- Signals/queries expuestos para integración (cancel, get-status).
- **Compensations**: el DSL puede declarar `compensate_with` para steps; en falla se ejecuta saga reversa.
- Worker pool por Tenant con autoscaling basado en lag.

Alternativa descartada: motor propio basado en Kafka — reinventa Temporal sin beneficio.

### D5.4 — Versionado SemVer + immutability + breaking detection

- Versiones formato `MAJOR.MINOR.PATCH`.
- Detector de breaking changes compara AST entre versiones:
  - Cambio en `inputs` schema (remove/required-add/type-change) → MAJOR.
  - Cambio en `outputs` schema → MAJOR.
  - Eliminación de step output usado por consumidores → MAJOR.
  - Cambio interno (logic step nuevo, retry tweaks) → MINOR.
  - Doc/metadata only → PATCH.
- El runtime mantiene compatibilidad de instalaciones existentes pinneando versión exacta.

### D5.5 — Marketplace y visibilidades

```
visibility:
  - private        # solo el autor
  - workspace      # todo el Workspace
  - tenant         # todo el Tenant (requiere Tenant approval)
  - forge-certified # certificado por Forge (eval pass + security review)
```

Instalación: `POST /v1/marketplace/install { workflow_id, version, target_workspace }` crea `workflow_install` registrando la versión pinneada y emite `workflow.installed_to_workspace.v1`.

Tenant approval para promoción a `tenant` se gestiona via Approvals Inbox con `tenant-admin` role.

### D5.6 — Eval harness avanzado

- **Golden datasets** versionados por workflow/skill, almacenados como assets `type=eval_dataset`.
- **Regression tests**: cada nueva versión corre el dataset; si la métrica clave cae > Δ configurable, bloquea publicación.
- **A/B**: marketplace permite A/B entre versiones para Workspaces opt-in; métricas comparadas tras N ejecuciones.
- **Métricas de negocio**: el workflow declara `success_metric` (e.g., `pr_merged_within_24h`, `incident_resolved_in_15m`); harness instrumenta y reporta.

### D5.7 — Per-asset observability dashboards

Cada asset (skill, prompt, workflow) tiene tab "Observability" en Portal con:
- Invocations / hour, day, week.
- Success rate y trend.
- Latency p50/p95/p99.
- Cost per execution (LLM + compute).
- Eval score trend (drift detection).
- Top failing steps (workflows).

Backend: agregaciones desde Langfuse + Temporal + workflow events; storage en ClickHouse para alto volumen (opcional).

### D5.8 — Human-in-the-loop step

- Tipo de nodo `human-in-the-loop` con `approver_role`, `timeout`, `inputs`, `expected_outputs`.
- Crea entry en Approvals Inbox al ejecutarse; workflow queda waiting.
- En timeout: configurable `on_timeout: {fail | proceed | escalate}`.
- Approver puede modificar inputs antes de aprobar (queda audited).

### D5.9 — Worker security boundary

- Cada Tenant Temporal namespace con SA dedicada y network policies.
- Activities ejecutan en workers que NO pueden alcanzar workspaces ajenos (enforced via OpenFGA + network policies).
- Skills invocadas via SDK de Skills (Fase 1) que ya respeta scoping.

## Risks / Trade-offs

- **Costo de Temporal**: aceptado por durabilidad; mitigado con TTL agresivo de history y archival.
- **Curva de adopción del editor**: mitigado con templates de workflow iniciales (release-train, on-call response, scaffold-and-deploy).
- **Eval harness puede dar falsa sensación de calidad**: mitigado con monitoring continuo en producción + drift alerts.
- **Marketplace con workflows mal diseñados**: mitigado con publish gate (eval pass + lint clean) y revisión Tenant para visibility=tenant.

## Migration Plan

1. Implementar AST + DSL parser/validator; round-trip tests YAML ↔ AST.
2. Implementar `workflow-runtime` sobre Temporal con steps tipos `skill`, `mcp`, `prompt`, `branch`, `loop`, `human-in-the-loop`, `sub-workflow`.
3. Implementar `workflow-registry` con SemVer y immutability + breaking detection.
4. Implementar editor visual (React Flow) con import/export DSL.
5. Implementar marketplace y flujos de install + visibility approvals.
6. Implementar eval harness avanzado y per-asset observability dashboards.
7. Migrar 3 flujos manuales clave a workflows (e.g., onboarding-app, deploy-canary, release-train) y publicar como `forge-certified`.
8. Sign-off Platform + Engineering Leads + 2 Workspaces piloto.

## Open Questions

- ¿Soportar lenguajes de scripting (e.g., JS sandbox) en steps custom o limitar a referencias Registry? — **Decisión inicial**: solo referencias Registry; scripts en sandbox quedan diferidos.
- ¿Multi-Tenant en un solo Temporal cluster con namespaces o cluster por Tenant? — **Decisión inicial**: namespaces por Tenant; cluster dedicado solo si Tenant lo requiere por compliance.
- ¿Marketplace público inter-Tenant en futuro? — **Out of scope** en Fase 5; arquitectura preparada para separar publishing public vs tenant.
- ¿Soportar workflows event-driven (suscriptos a eventos del bus)? — **Decisión inicial**: sí, vía nodo trigger `event` que arranca el workflow al recibir un CloudEvent matcheable.
