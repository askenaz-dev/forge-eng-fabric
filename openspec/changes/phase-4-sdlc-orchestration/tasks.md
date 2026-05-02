# Tasks — Phase 4: SDLC Orchestration

## 1. Modelo de iniciativa y persistencia

- [ ] 1.1 Crear `services/sdlc-orchestrator/` (Go) con API REST y workers de eventos.
- [ ] 1.2 Modelo de datos: `sdlc_initiative`, `sdlc_phase_state`, `sdlc_gate_result`, `sdlc_blocker`.
- [ ] 1.3 Endpoints: `POST /v1/initiatives`, `GET /v1/initiatives/{id}`, `POST /v1/initiatives/{id}/phase/{phase}/complete`.
- [ ] 1.4 Máquina de estados con transiciones por evento + manuales.
- [ ] 1.5 Eventos `sdlc.phase.entered.v1`, `sdlc.phase.gate_evaluated.v1`, `sdlc.phase.progressed.v1`, `sdlc.phase.blocked.v1`, `sdlc.phase.regressed.v1`.

## 2. Gate engine

- [ ] 2.1 Extender `services/policy-engine/` con tipo `sdlc-gate` y evaluador YAML+CEL.
- [ ] 2.2 Catálogo inicial de gates por fase (D4.2) como policy templates.
- [ ] 2.3 Thresholds por defecto por criticidad (D4.9) configurables.
- [ ] 2.4 Override `phase-progression-bypass` con TTL y approval.

## 3. Traceability graph

- [ ] 3.1 Crear `services/traceability/` (Go) con modelo `traceability_node`, `traceability_link`.
- [ ] 3.2 Ingestion workers que consumen eventos Fases 1–3 (`pr.linked-to-openspec.v1`, `deployment.applied.v1`, `app.onboarding.completed.v1`, etc.) y crean nodos/links.
- [ ] 3.3 Endpoint `GET /v1/traceability/{openspec_id}` con grafo materializado.
- [ ] 3.4 Materialized views con refresh cada 5 min.
- [ ] 3.5 Backfill histórico desde audit log existente.

## 4. Jira MCP

- [ ] 4.1 Crear `services/mcp/jira/` (Python) con tools `create_issue`, `update_issue`, `transition_issue`, `add_comment`, `link_issue`, `create_epic`, `list_sprints`, `search`.
- [ ] 4.2 Soporte OAuth 2.0 (Jira Cloud) y API token; almacenamiento cifrado de credenciales.
- [ ] 4.3 Mapping `workspace ↔ jira_project_keys[]` con enforcement.
- [ ] 4.4 Rate-limit awareness con backoff y circuit breaker.
- [ ] 4.5 Webhook listener para eventos Jira → emite `jira.issue.*.v1` al bus.
- [ ] 4.6 Reconciliation job (cada 15 min) entre OpenSpec linked Jira issues.
- [ ] 4.7 Tests E2E contra Jira sandbox.

## 5. Confluence MCP

- [ ] 5.1 Crear `services/mcp/confluence/` (Python) con tools `create_page`, `update_page`, `attach_file`, `add_label`, `search`.
- [ ] 5.2 Mapping `workspace ↔ confluence_space_keys[]` con enforcement.
- [ ] 5.3 Páginas creadas por Forge llevan label `forge-managed` y header con OpenSpec link.
- [ ] 5.4 Webhook listener → emite `confluence.page.*.v1`.
- [ ] 5.5 Tests E2E contra Confluence sandbox.

## 6. Skills por capability SDLC

- [ ] 6.1 `sdlc-product`: skills `refine-user-story`, `generate-acceptance-criteria`, `prioritize-backlog` con eval suites.
- [ ] 6.2 `sdlc-architecture`: `propose-adr`, `evaluate-options`, `check-openspec-alignment`.
- [ ] 6.3 `sdlc-design`: `generate-api-contract`, `propose-data-model`, `lightweight-threat-model`.
- [ ] 6.4 `sdlc-development`: `implement-feature-tests-first`, `refactor-with-safety-net`, `apply-code-review-feedback`.
- [ ] 6.5 `sdlc-qa`: `generate-test-plan`, `generate-e2e-tests`, `triage-test-failures`.
- [ ] 6.6 `sdlc-security`: `triage-vuln`, `propose-fix-for-finding`, `update-threat-model`.
- [ ] 6.7 `sdlc-devops`: `prepare-release-notes`, `validate-rollback-plan`, `update-pipeline`.
- [ ] 6.8 `sdlc-sre`: `define-slos-from-spec`, `generate-runbook`, `tune-alerts`.
- [ ] 6.9 `sdlc-finops`: `estimate-cost-from-spec`, `monitor-budget`, `propose-cost-reduction`.
- [ ] 6.10 Registrar todas las skills como assets con `lifecycle_state=approved, trust_level=T2`.

## 7. OpenSpec backbone extensions

- [ ] 7.1 Extender modelo `decision_log` con tipos `jira_link`, `confluence_link`, `test_run_link`, `slo_link`.
- [ ] 7.2 Extender `linked_artifacts` con namespace `jira:`, `confluence:`, `test:`, `slo:`, `incident:`.
- [ ] 7.3 Hooks bidireccionales: cuando un PR es merged (Fase 2), updaten OpenSpec; cuando un issue Jira cambia status, ídem.

## 8. FinOps integration

- [ ] 8.1 Pipeline GCP Billing export → BigQuery con tags `workspace`, `env`, `asset`, `initiative_openspec`.
- [ ] 8.2 Importer LLM costs desde Langfuse y LiteLLM agregando por initiative.
- [ ] 8.3 Modelo `finops_budget` con thresholds 50/80/100%.
- [ ] 8.4 Alertas como eventos `finops.budget.threshold_reached.v1`.
- [ ] 8.5 Dashboard FinOps por initiative y Workspace.

## 9. Portal — UI

- [ ] 9.1 Módulo "Initiatives" con lista y detalle por iniciativa.
- [ ] 9.2 Vista "Phase progression" con estado por fase, gates evaluados y blockers.
- [ ] 9.3 Vista "Traceability graph" con drill-down (D3.js o vis-network).
- [ ] 9.4 Enriquecer OpenSpec viewer con tabs Jira/Confluence/Tests/SLOs/Costs.
- [ ] 9.5 Tests E2E con Playwright.

## 10. Observabilidad y métricas

- [ ] 10.1 Métricas: `sdlc_phase_duration_seconds`, `gate_pass_rate`, `phase_block_rate`, `traceability_coverage`, `traceability_query_latency_p95`, `jira_sync_lag_seconds`, `confluence_sync_lag_seconds`.
- [ ] 10.2 Dashboards Grafana por Workspace.
- [ ] 10.3 SLOs iniciales: gate eval p95 ≤ 5s, traceability query p95 ≤ 1s, jira/confluence sync lag ≤ 5min.

## 11. Validación y sign-off

- [ ] 11.1 Workspace piloto: ejecutar 1 iniciativa real end-to-end (product → finops).
- [ ] 11.2 Verificar trazabilidad bidireccional en cada nodo del grafo.
- [ ] 11.3 Verificar bloqueo en gate fallido y override flow.
- [ ] 11.4 Sign-off Platform + Engineering Leads + Workspace piloto.
- [ ] 11.5 Documentación: `docs/sdlc/`, `docs/sdlc/gates.md`, `docs/sdlc/traceability.md`, `docs/finops/`.
