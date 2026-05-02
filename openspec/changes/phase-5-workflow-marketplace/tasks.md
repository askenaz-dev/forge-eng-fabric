# Tasks — Phase 5: Workflow Marketplace

## 1. AST y DSL

- [ ] 1.1 Definir AST canónico (`pkg/workflow/ast`) con tipos de nodo (skill, mcp, prompt, branch, loop, human-in-the-loop, sub-workflow, event-trigger).
- [ ] 1.2 Implementar parser DSL YAML → AST y serializer AST → DSL.
- [ ] 1.3 JSON Schema de DSL y validador.
- [ ] 1.4 Linter (unreachable steps, dangling deps, type mismatches, cycle detection).
- [ ] 1.5 Tests round-trip YAML ↔ AST.

## 2. Workflow runtime (Temporal)

- [ ] 2.1 Crear `services/workflow-runtime/` con worker pool por Tenant.
- [ ] 2.2 Provisionar Temporal namespaces por Tenant.
- [ ] 2.3 Implementar activities por tipo de step.
- [ ] 2.4 RetryPolicy mapping desde DSL.
- [ ] 2.5 Compensations (saga reversa) en `on_failure`.
- [ ] 2.6 Signals/queries (cancel, get-status, get-step-output).
- [ ] 2.7 Modelo de datos: `workflow_execution`, `workflow_step_event`.
- [ ] 2.8 Eventos `workflow.execution.started.v1`, `workflow.step.completed.v1`, `workflow.step.failed.v1`, `workflow.retried.v1`, `workflow.compensated.v1`.

## 3. Workflow registry y versionado

- [ ] 3.1 Crear `services/workflow-registry/` con CRUD de workflows y versions.
- [ ] 3.2 SemVer enforcement + immutability (DB triggers).
- [ ] 3.3 Breaking-change detector comparando ASTs.
- [ ] 3.4 Endpoint `POST /v1/workflows/{id}/versions` con auto-bump según diff.

## 4. Editor visual

- [ ] 4.1 Bootstrap módulo Portal "Workflows" con React Flow.
- [ ] 4.2 Sidebar con catálogo de Skills/MCPs/Prompts (consume Registry en vivo).
- [ ] 4.3 Validación + lint inline; marcadores de error en nodos.
- [ ] 4.4 Import/export DSL.
- [ ] 4.5 Debug mode con dry-run.
- [ ] 4.6 Diff viewer entre versiones.
- [ ] 4.7 Tests E2E con Playwright.

## 5. Marketplace

- [ ] 5.1 Crear `services/marketplace/` con CRUD de listings.
- [ ] 5.2 Modelo `marketplace_listing` con visibility (private/workspace/tenant/forge-certified).
- [ ] 5.3 Endpoint `GET /v1/marketplace?visibility=tenant` con filtros y búsqueda.
- [ ] 5.4 Endpoint `POST /v1/marketplace/install` con `workflow_install` record y pinning a versión exacta.
- [ ] 5.5 Approval flow para visibility `tenant` (Tenant admin) y `forge-certified` (eval+security).
- [ ] 5.6 Eventos `workflow.published.v1`, `workflow.installed_to_workspace.v1`.
- [ ] 5.7 UI Marketplace: browse, detail, install button.

## 6. Eval harness avanzado

- [ ] 6.1 Crear `services/eval-harness-adv/` (Python) extendiendo el de Fase 1.
- [ ] 6.2 Modelo `eval_dataset` (asset type) con golden inputs/outputs.
- [ ] 6.3 Regression test runner: ejecuta dataset contra versión nueva; bloquea publish si métrica clave cae > Δ.
- [ ] 6.4 A/B runner: split traffic entre versiones para Workspaces opt-in.
- [ ] 6.5 Business metric instrumentation (declarado en workflow `success_metric`).
- [ ] 6.6 Storage de runs en `workflow_eval_run`.

## 7. Per-asset observability

- [ ] 7.1 Crear `services/asset-observability/` (Go) con agregación desde Langfuse + Temporal + bus.
- [ ] 7.2 Storage opcional ClickHouse para alto volumen.
- [ ] 7.3 Endpoints `GET /v1/assets/{id}/metrics?range=...&granularity=...`.
- [ ] 7.4 Tab Observability en Portal Asset detail con: invocations, success rate, p50/p95/p99 latency, cost/execution, eval drift, top failing steps.

## 8. Human-in-the-loop

- [ ] 8.1 Activity Temporal `human_in_the_loop` que crea entry en Approvals Inbox y waits en signal.
- [ ] 8.2 Configuración `on_timeout` (fail/proceed/escalate).
- [ ] 8.3 Logging completo: inputs presented, modifications, decision, approver.
- [ ] 8.4 UI Approvals Inbox extendida para mostrar workflow context (steps previos, próximo step).

## 9. Registry asset extension

- [ ] 9.1 Añadir tipo `workflow` con sub-recursos `version`, `eval_run`, `installation`.
- [ ] 9.2 Asset detail muestra grafos de versiones, instalaciones por Workspace, eval runs.

## 10. Policies extensions

- [ ] 10.1 Templates `require-eval-pass`, `require-security-review`, `require-tenant-share-approval`.
- [ ] 10.2 Integración con marketplace publish/install flow.

## 11. Workflows iniciales (forge-certified)

- [ ] 11.1 `release-train@1.0.0`: orquesta release multi-asset con gates.
- [ ] 11.2 `scaffold-and-deploy@1.0.0`: onboarding (Fase 2) + deploy a dev (Fase 3).
- [ ] 11.3 `incident-response@1.0.0`: triage + comms + initial mitigation skeleton.
- [ ] 11.4 Eval suites por cada uno.

## 12. Observabilidad y métricas

- [ ] 12.1 Métricas: `workflow_execution_duration_seconds`, `workflow_step_failure_rate`, `marketplace_install_count`, `eval_regression_blocks`, `human_in_loop_timeout_rate`.
- [ ] 12.2 Dashboards Grafana globales + per-asset.

## 13. Validación y sign-off

- [ ] 13.1 Workspace piloto: diseñar workflow custom, eval pass, publicar a tenant, instalar en otro Workspace.
- [ ] 13.2 Ejecutar los 3 workflows forge-certified en producción real.
- [ ] 13.3 Sign-off Platform + Engineering + 2 Workspaces piloto.
- [ ] 13.4 Documentación: `docs/workflows/dsl.md`, `docs/workflows/editor.md`, `docs/marketplace/`, `docs/eval-harness/advanced.md`.
