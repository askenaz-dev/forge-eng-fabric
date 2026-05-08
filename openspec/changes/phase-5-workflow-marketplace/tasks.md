# Tasks — Phase 5: Workflow Marketplace

## 1. AST y DSL

- [x] 1.1 Definir AST canónico (`pkg/workflow/ast`) con tipos de nodo (skill, mcp, prompt, branch, loop, human-in-the-loop, sub-workflow, event-trigger).
- [x] 1.2 Implementar parser DSL YAML → AST y serializer AST → DSL.
- [x] 1.3 JSON Schema de DSL y validador.
- [x] 1.4 Linter (unreachable steps, dangling deps, type mismatches, cycle detection).
- [x] 1.5 Tests round-trip YAML ↔ AST.

## 2. Workflow runtime (Temporal)

- [x] 2.1 Crear `services/workflow-runtime/` con worker pool por Tenant.
- [x] 2.2 Provisionar Temporal namespaces por Tenant.
- [x] 2.3 Implementar activities por tipo de step.
- [x] 2.4 RetryPolicy mapping desde DSL.
- [x] 2.5 Compensations (saga reversa) en `on_failure`.
- [x] 2.6 Signals/queries (cancel, get-status, get-step-output).
- [x] 2.7 Modelo de datos: `workflow_execution`, `workflow_step_event`.
- [x] 2.8 Eventos `workflow.execution.started.v1`, `workflow.step.completed.v1`, `workflow.step.failed.v1`, `workflow.retried.v1`, `workflow.compensated.v1`.

## 3. Workflow registry y versionado

- [x] 3.1 Crear `services/workflow-registry/` con CRUD de workflows y versions.
- [x] 3.2 SemVer enforcement + immutability (DB triggers).
- [x] 3.3 Breaking-change detector comparando ASTs.
- [x] 3.4 Endpoint `POST /v1/workflows/{id}/versions` con auto-bump según diff.

## 4. Editor visual

- [x] 4.1 Bootstrap módulo Portal "Workflows" con React Flow.
- [x] 4.2 Sidebar con catálogo de Skills/MCPs/Prompts (consume Registry en vivo).
- [x] 4.3 Validación + lint inline; marcadores de error en nodos.
- [x] 4.4 Import/export DSL.
- [x] 4.5 Debug mode con dry-run.
- [x] 4.6 Diff viewer entre versiones.
- [x] 4.7 Tests E2E con Playwright.

## 5. Marketplace

- [x] 5.1 Crear `services/marketplace/` con CRUD de listings.
- [x] 5.2 Modelo `marketplace_listing` con visibility (private/workspace/tenant/forge-certified).
- [x] 5.3 Endpoint `GET /v1/marketplace?visibility=tenant` con filtros y búsqueda.
- [x] 5.4 Endpoint `POST /v1/marketplace/install` con `workflow_install` record y pinning a versión exacta.
- [x] 5.5 Approval flow para visibility `tenant` (Tenant admin) y `forge-certified` (eval+security).
- [x] 5.6 Eventos `workflow.published.v1`, `workflow.installed_to_workspace.v1`.
- [x] 5.7 UI Marketplace: browse, detail, install button.

## 6. Eval harness avanzado

- [x] 6.1 Crear `services/eval-harness-adv/` (Python) extendiendo el de Fase 1.
- [x] 6.2 Modelo `eval_dataset` (asset type) con golden inputs/outputs.
- [x] 6.3 Regression test runner: ejecuta dataset contra versión nueva; bloquea publish si métrica clave cae > Δ.
- [x] 6.4 A/B runner: split traffic entre versiones para Workspaces opt-in.
- [x] 6.5 Business metric instrumentation (declarado en workflow `success_metric`).
- [x] 6.6 Storage de runs en `workflow_eval_run`.

## 7. Per-asset observability

- [x] 7.1 Crear `services/asset-observability/` (Go) con agregación desde Langfuse + Temporal + bus.
- [x] 7.2 Storage opcional ClickHouse para alto volumen.
- [x] 7.3 Endpoints `GET /v1/assets/{id}/metrics?range=...&granularity=...`.
- [x] 7.4 Tab Observability en Portal Asset detail con: invocations, success rate, p50/p95/p99 latency, cost/execution, eval drift, top failing steps.

## 8. Human-in-the-loop

- [x] 8.1 Activity Temporal `human_in_the_loop` que crea entry en Approvals Inbox y waits en signal.
- [x] 8.2 Configuración `on_timeout` (fail/proceed/escalate).
- [x] 8.3 Logging completo: inputs presented, modifications, decision, approver.
- [x] 8.4 UI Approvals Inbox extendida para mostrar workflow context (steps previos, próximo step).

## 9. Registry asset extension

- [x] 9.1 Añadir tipo `workflow` con sub-recursos `version`, `eval_run`, `installation`.
- [x] 9.2 Asset detail muestra grafos de versiones, instalaciones por Workspace, eval runs.

## 10. Policies extensions

- [x] 10.1 Templates `require-eval-pass`, `require-security-review`, `require-tenant-share-approval`.
- [x] 10.2 Integración con marketplace publish/install flow.

## 11. Workflows iniciales (forge-certified)

- [x] 11.1 `release-train@1.0.0`: orquesta release multi-asset con gates.
- [x] 11.2 `scaffold-and-deploy@1.0.0`: onboarding (Fase 2) + deploy a dev (Fase 3).
- [x] 11.3 `incident-response@1.0.0`: triage + comms + initial mitigation skeleton.
- [x] 11.4 Eval suites por cada uno.

## 12. Observabilidad y métricas

- [x] 12.1 Métricas: `workflow_execution_duration_seconds`, `workflow_step_failure_rate`, `marketplace_install_count`, `eval_regression_blocks`, `human_in_loop_timeout_rate`.
- [x] 12.2 Dashboards Grafana globales + per-asset.

## 13. Validación y sign-off

- [ ] 13.1 Workspace piloto: diseñar workflow custom, eval pass, publicar a tenant, instalar en otro Workspace. _(playbook in `docs/workflows/validation.md`; pending live execution)_
- [ ] 13.2 Ejecutar los 3 workflows forge-certified en producción real. _(playbook in `docs/workflows/validation.md`; pending live execution)_
- [ ] 13.3 Sign-off Platform + Engineering + 2 Workspaces piloto. _(awaiting 13.1/13.2 evidence)_
- [x] 13.4 Documentación: `docs/workflows/dsl.md`, `docs/workflows/editor.md`, `docs/marketplace/`, `docs/eval-harness/advanced.md`.
