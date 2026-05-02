## Context

Fase 4 transforma a Forge de "asistente puntual" a "compañero a lo largo del ciclo de vida". Las decisiones definen cómo se modela una iniciativa, qué fases obligatorias existen, cómo se evalúan los gates, cómo se mantiene la trazabilidad, y cómo se evita que Alfred avance fases sin cumplir condiciones de calidad/seguridad. Asume Fases 0–3 entregadas: tenancy, Alfred + RAG, OpenSpec backbone, policies, MCPs base, onboarding y deploys gobernados.

## Constraints

- **Una iniciativa = un OpenSpec raíz**: el OpenSpec es la fuente de verdad de intención y decisiones; Jira/Confluence son views operativas.
- **Sin avance sin gates**: la progresión `phase_n → phase_n+1` requiere `gate_result=passed` o `override_approved`.
- **Trazabilidad obligatoria**: cada artefacto generado en una fase MUST referenciar al menos un OpenSpec; ausencias son bloqueantes (mismo principio que Fase 2 PR-OpenSpec).
- **No bypass de LiteLLM**: las skills siguen usando el gateway único.
- **No mutaciones cross-Workspace**: las MCPs (Jira/Confluence) operan scoped al Workspace.
- **Idempotencia**: re-ejecutar una skill con mismos inputs no produce duplicados (delegada al servicio receptor — Jira "create or update", OpenSpec versionado, etc.).

## Decisions

### D4.1 — Modelo de iniciativa y fases

```
Initiative
 ├── openspec_root (id de OpenSpec)
 ├── jira_epic_key (opcional)
 ├── workspace_id
 ├── criticality
 ├── current_phase (product|architecture|design|development|qa|security|devops|sre|finops|done)
 └── phase_states[]:
       phase, status (not_started|in_progress|gate_pending|passed|failed|skipped|overridden),
       entered_at, completed_at, gates[], blockers[]
```

Fases ordenadas: `product → architecture → design → development → qa → security → devops → sre → finops`. Las dos últimas (sre, finops) son **transversales** (pueden iniciar en paralelo a `development` y persisten más allá del done de la iniciativa). Configurables por Workspace para permitir omitir fases en proyectos pequeños (con override).

### D4.2 — Gates por fase (catálogo inicial)

| Fase | Gates default | Configurable |
|---|---|---|
| product | acceptance_criteria_present, story_size_estimated | sí |
| architecture | adrs_published, security_review_passed, openspec_updated | sí |
| design | api_contracts_defined, threat_model_present (`criticality≥medium`), data_model_documented | sí |
| development | code_complete, lint_clean, unit_tests_passing, coverage≥threshold | sí |
| qa | integration_tests_passing, e2e_tests_passing, perf_budget_met (`criticality≥high`) | sí |
| security | sast_clean, sca_clean, dast_passed (`criticality≥high`), secrets_clean | sí |
| devops | pipelines_green, image_signed, deploy_to_stage_successful, rollback_plan_present | sí |
| sre | slos_defined, runbook_published, alerts_configured, on_call_assigned | sí |
| finops | cost_estimate_within_budget, llm_budget_within_limit | sí |

Gates expresados como YAML+CEL evaluables por `policy-engine` (extensión Fase 1).

### D4.3 — Jira MCP (write)

Tools expuestas (subset clave): `jira.create_issue`, `jira.update_issue`, `jira.transition_issue`, `jira.add_comment`, `jira.link_issue`, `jira.create_epic`, `jira.list_sprints`. Auth: API token o OAuth 2.0 (cliente recomienda OAuth para Jira Cloud). Rate-limit awareness con backoff exponencial; cache de issues read-only por 60s.

### D4.4 — Confluence MCP (write)

Tools: `confluence.create_page`, `confluence.update_page`, `confluence.attach_file`, `confluence.add_label`, `confluence.search`. Páginas creadas por Forge llevan label `forge-managed` y un comentario header con `OpenSpec: <id>` + advertencia "Editado por Alfred — cambios humanos posibles, ver historial".

### D4.5 — Modelo de trazabilidad

Grafo persistido en Postgres (tablas) + opcionalmente expuesto vía endpoint a Neo4j/JanusGraph en el futuro:

```
traceability_node (id, type, external_id, workspace_id, metadata)
traceability_link (id, from_node, to_node, relation, created_at, source)
```

Tipos de nodo: `openspec`, `jira_issue`, `jira_epic`, `confluence_page`, `adr`, `pr`, `commit`, `deployment`, `test_run`, `slo`, `incident`, `cost_record`. Relaciones: `derives_from`, `implements`, `validates`, `deploys`, `monitored_by`, `cost_attributed_to`.

Materialized views por OpenSpec id para queries `traceability_for_openspec(id)` con TTL 5min.

### D4.6 — Phase progression engine

`sdlc-orchestrator` mantiene una máquina de estados por iniciativa. Transiciones disparadas por:
- Eventos del bus (PR merged, deploy applied, test run completed, scanner finished).
- Acciones explícitas en Portal (operador o Alfred completa una fase manualmente).

En cada transición:
1. Evalúa gates de la fase actual.
2. Emite `sdlc.phase.gate_evaluated.v1` con resultado por gate.
3. Si todos pasan → emite `sdlc.phase.progressed.v1` y avanza a siguiente fase.
4. Si fallan → emite `sdlc.phase.blocked.v1`, crea blockers en `phase_state.blockers`, notifica Approvals Inbox.

Override `phase-progression-bypass` requiere approval por `release-manager` con razón obligatoria; auditado.

### D4.7 — Skills por fase

Cada capability `sdlc-*` aporta ≥3 skills iniciales referenciadas como assets en el Registry con `lifecycle_state` y `trust_level`. Ejemplos:
- `sdlc-product`: `refine-user-story`, `generate-acceptance-criteria`, `prioritize-backlog`.
- `sdlc-architecture`: `propose-adr`, `evaluate-options`, `check-openspec-alignment`.
- `sdlc-design`: `generate-api-contract`, `propose-data-model`, `lightweight-threat-model`.
- `sdlc-development`: `implement-feature-tests-first`, `refactor-with-safety-net`, `apply-code-review-feedback`.
- `sdlc-qa`: `generate-test-plan`, `generate-e2e-tests`, `triage-test-failures`.
- `sdlc-security`: `triage-vuln`, `propose-fix-for-finding`, `update-threat-model`.
- `sdlc-devops`: `prepare-release-notes`, `validate-rollback-plan`, `update-pipeline`.
- `sdlc-sre`: `define-slos-from-spec`, `generate-runbook`, `tune-alerts`.
- `sdlc-finops`: `estimate-cost-from-spec`, `monitor-budget`, `propose-cost-reduction`.

Cada skill tiene eval suite mínima (Fase 1 framework) y trust level inicial T2 que progresa con uso.

### D4.8 — FinOps mecanismos

Fuentes:
- **Cloud cost**: GCP Billing export → BigQuery → consultas filtradas por `project_id` (asociado a Workspace/env).
- **LLM cost**: agregación desde Langfuse (Fase 1) y LiteLLM logs (Fase 0).
- **Atribución**: cada deploy lleva tags `workspace`, `env`, `asset`, `initiative_openspec` para joinable BigQuery analytics.

Presupuestos definidos en `finops_budget` por Workspace/initiative; alertas en 50/80/100% emitidas como eventos.

### D4.9 — Quality gates: thresholds por defecto

| Gate | low | medium | high | critical |
|---|---|---|---|---|
| coverage | 70% | 75% | 80% | 85% |
| cyclomatic complexity (max func) | 15 | 12 | 10 | 8 |
| duplication % | 10 | 8 | 5 | 3 |
| perf p95 latency budget | n/a | n/a | per-spec | per-spec |
| accessibility WCAG | AA optional | AA required | AA required | AAA |

Configurables por policy.

### D4.10 — Confluence/Jira tenancy

Mapeos `workspace ↔ jira_project_keys[]` y `workspace ↔ confluence_space_keys[]` configurables. El MCP rechaza operaciones a projects/spaces no mapeados al Workspace del actor.

## Risks / Trade-offs

- **Sobrecarga de gates** que ralentiza entregas: mitigado con thresholds configurables por criticidad y overrides documentados.
- **Drift entre Jira/Confluence y OpenSpec**: mitigado con webhooks y reconciliation jobs (cada 15 min); divergencias generan eventos.
- **Permisos amplios de Jira/Confluence MCP**: mitigado con scoped service accounts por Workspace + rate-limit + audit.
- **Traceability graph creciendo sin podas**: mitigado con archivado por edad + retención por data classification.
- **Skill quality variability**: mitigado con eval suites obligatorias y monitoring de regresión.

## Migration Plan

1. Implementar `services/traceability/` y modelo de nodos/links + ingestion de eventos existentes (Fases 1–3) para llenar grafo histórico.
2. Implementar `services/sdlc-orchestrator/` con máquina de estados y gate evaluation.
3. Implementar Jira MCP write y mapping `workspace ↔ jira projects`; piloto con un Workspace.
4. Implementar Confluence MCP write y mapping `workspace ↔ confluence spaces`.
5. Publicar skills iniciales por capability (≥3 c/u) en Registry como assets `lifecycle_state=approved, trust_level=T2`.
6. Habilitar policies de gates en Workspace piloto; correr 1 iniciativa end-to-end con datos reales.
7. UI Portal: "Initiative" + "Traceability".
8. Sign-off Platform + Engineering Leads + Workspace piloto.

## Open Questions

- ¿Soportar GitHub Issues + Projects como alternativa a Jira? — **Decisión inicial**: priorizar Jira; abstracción `IssueTracker` permite añadir GitHub Issues luego.
- ¿Confluence Cloud only o también Server/Data Center? — **Decisión inicial**: Cloud y Server vía adapter; Data Center si demanda.
- ¿Permitir múltiples OpenSpec roots por iniciativa? — **Decisión inicial**: uno raíz + N satélites referenciados; el orchestrator usa el raíz para gating.
- ¿FinOps con presupuestos hard (bloquean) o soft (alertan)? — **Decisión inicial**: soft por defecto; hard requiere policy explícita por Workspace.
