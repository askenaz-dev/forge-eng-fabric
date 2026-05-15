## 1. Data model and persistence

- [x] 1.1 Add `application` table (id UUID v7, slug, name, description, workspace_id FK, tenant_id, lifecycle_state, design_system_ref, default_environments JSONB, repo_links JSONB, runtime_links JSONB, owners JSONB, audit timestamps) with `(workspace_id, slug)` unique index
- [x] 1.2 Add nullable `app_id` column + FK on `openspec`, `onboarding_request`, `deployment`, `runtime_registration`, `asset` tables; index `app_id` on each
- [x] 1.3 Add CHECK constraint on `openspec`: `workspace_id` must equal `application.workspace_id` of the referenced `app_id` (validated at app layer; enforced in DB after backfill)
- [x] 1.4 Add `application_audit` partitioned table for App lifecycle records
- [x] 1.5 Write Atlas/SQL migration script with up/down for steps 1.1–1.4

## 2. OpenFGA model

- [x] 2.1 Add `app` type definition with `owner`, `editor`, `viewer`, `parent` relations to `contracts/openfga/authorization-model.json`
- [x] 2.2 Wire `parent` to `workspace`; compute `viewer` as `direct | editor | owner | parent.viewer`, `editor` as `direct | owner | parent.editor`
- [x] 2.3 Update OpenFGA tests under `contracts/openfga/tests/` to cover App relations and computed inheritance (workspace viewer → app viewer)
- [x] 2.4 Update OpenFGA tests to cover the explicit-app-override case (no workspace tuple, direct app#editor tuple)

## 3. App CRUD service

- [x] 3.1 Scaffold `services/application` (Go) with the standard service layout (cmd, internal/handlers, internal/store, internal/events)
- [x] 3.2 Implement `POST /v1/workspaces/{ws}/apps`, `GET /v1/workspaces/{ws}/apps`, `GET /v1/apps/{id}`, `PATCH /v1/apps/{id}`, `POST /v1/apps/{id}:archive`, `POST /v1/apps/{id}:restore`, `DELETE /v1/apps/{id}`
- [x] 3.3 Enforce OpenFGA authz on every endpoint (`app#owner` / `app#editor` / `app#viewer` based on operation)
- [x] 3.4 Emit `app.created.v1`, `app.updated.v1`, `app.archived.v1`, `app.restored.v1`, `app.deleted.v1` via the platform event bus
- [x] 3.5 Reject DELETE when live artefacts exist (specs not terminal, deployments live, onboarding in-flight)
- [x] 3.6 Reject cross-workspace App references on every PATCH/POST input
- [x] 3.7 Update `contracts/openapi/registry.yaml` with the new App resource and schemas
- [x] 3.8 Unit tests for handlers; integration tests against ephemeral Postgres + OpenFGA

## 4. System-managed `_unassigned` App

- [x] 4.1 Implement workspace-bootstrap hook that creates the `_unassigned` App atomically with the new Workspace
- [x] 4.2 Reject `PATCH` / `DELETE` / `archive` on any App whose `slug=_unassigned` with `403 system_managed_app`
- [x] 4.3 Reject `POST /v1/workspaces/{ws}/apps` if `slug=_unassigned` (only the system can create it)
- [x] 4.4 Block public writes that target `_unassigned` as `app_id` on new OpenSpecs (writes from the migration job are allowed via a service principal)
- [x] 4.5 Tests for read-only invariants and bootstrap hook

## 5. OpenSpec backbone integration

- [x] 5.1 Add `app_id` to OpenSpec record schema; update OpenSpec persistence to require `app_id` (feature-flagged via `forge.app_entity.enabled`)
- [x] 5.2 Validate `workspace_id == application.workspace_id` for any incoming OpenSpec
- [x] 5.3 Update `intent.committed.v1` event payload schema to include `app_id`
- [x] 5.4 Implement `POST /v1/specs/{id}:reparent` with `app#editor` on both source and target; emit `spec.reparented.v1`
- [x] 5.5 Backbone unit tests + contract tests for the new event shape

## 6. Intent Capture Wizard App scope step

- [x] 6.1 Add an `app_scope` step to the wizard state machine as the first step
- [x] 6.2 Implement the "extend existing App" branch (list workspace Apps, search, preview history)
- [x] 6.3 Implement the "create a new App" branch (inline form → App CRUD call)
- [x] 6.4 Implement the "decide later" branch (`draft.app_id = _unassigned` + sticky banner)
- [x] 6.5 Refuse commit while `draft.app_id` points to `_unassigned`; surface inline App scope step
- [x] 6.6 Update wizard completeness map to mark `app_id` as required
- [x] 6.7 Wizard E2E test: complete flow with existing App; complete flow with inline App creation; commit blocked with `_unassigned`

## 7. Alfred control plane updates

- [x] 7.1 Add `app_id` to `POST /v1/intent/start`, `/answer`, `/commit` request schemas; reject calls without it (`422 missing_app_scope`)
- [x] 7.2 Thread `app_id` through every decision-log entry and audit event for the duration of a dialogue
- [x] 7.3 Update RAG retriever to scope queries to the App's corpus by default, with workspace fallback when empty; log the effective scope on every retrieval
- [x] 7.4 Update Alfred tests for the new scoping rules

## 8. Downstream services: onboarding, asset registry

- [x] 8.1 Update `app-onboarding-service` request schema to require `app_id` XOR `app_proposal`
- [x] 8.2 Implement inline App creation when `app_proposal` is supplied (atomic with onboarding under one `correlation_id`)
- [x] 8.3 Update onboarding events to carry `app_id`; update idempotency key to `(workspace_id, app_id, repo_name)`
- [x] 8.4 Update ai-asset-registry to accept and persist `app_id`; reject cross-workspace App references on asset publication
- [x] 8.5 Add `?app_id=` filter to asset list endpoints; default behaviour excludes workspace-scoped null-app assets unless `include_workspace_scope=true`
- [x] 8.6 Implement archive cascade: on `app.archived.v1`, mark `app_id`-matching assets as `discoverable=false`; emit `asset.discoverability.changed.v1`

## 9. Migration job

- [x] 9.1 Build `cmd/spec-app-migration` CLI (Go) with subcommands `dry-run`, `confirm`, `execute`, `restore-from-audit`
- [x] 9.2 Implement orphan classification: spec is orphan iff no active deployment + no live onboarding + no runtime ref + not in pinned set/dashboard/Alfred conv within 90 days + lifecycle in {proposed, draft}
- [x] 9.3 Implement dry-run that produces CSV `migration-dry-run-{ws}-{ts}.csv` and uploads it to the workspace owner's inbox
- [x] 9.4 Implement owner-confirmation capture (signed token stored in `application_audit`)
- [x] 9.5 Implement execute mode: backfill `app_id` for retainable specs; hard-delete orphans after confirmation; copy purged record bodies into the 7-year audit retention bucket
- [x] 9.6 Emit `spec.reparented.v1` (one per backfilled spec) and `spec.purged.v1` (one per deletion)
- [x] 9.7 Implement `restore-from-audit` runbook command for wrongful-deletion recovery
- [x] 9.8 Integration tests against a captured prod-like dataset

## 10. Portal UI

- [x] 10.1 Add App picker to the global topbar (next to the Workspace picker), with empty-state copy when the workspace has only `_unassigned`
- [x] 10.2 Add App detail page at `/workspaces/{ws_slug}/apps/{app_slug}` with tabs: Specs / Runs / Deployments / Runtimes / Settings
- [x] 10.3 Render `_unassigned` group last with visually distinct styling; show migration banner when non-empty
- [x] 10.4 Update Workspace detail view to group specs under their parent App
- [x] 10.5 Add App creation modal (used by the wizard and from the App picker)
- [x] 10.6 Visual regression baseline (Playwright) for new screens

## 11. Rollout, feature flags and observability

- [x] 11.1 Add `forge.app_entity.enabled` feature flag (per-workspace), default `false`
- [x] 11.2 Add `forge.app_scope.wizard_step.enabled` flag (per-workspace), default `false`
- [x] 11.3 Dashboards: App-create rate, orphan count remaining, migration progress %, NOT-NULL readiness per workspace
- [x] 11.4 SLO: App CRUD p95 < 250ms; migration dry-run completes within 15 min per 10k specs
- [x] 11.5 Runbook: per-workspace cutover sequence (M0–M6 from design)
- [x] 11.6 Runbook: orphan-restore from audit retention

## 12. NOT NULL cutover

- [x] 12.1 Add per-workspace "backfill-complete" sentinel set by the migration job after asserting 0 NULL rows
- [x] 12.2 Build script that flips `openspec.app_id`, `onboarding_request.app_id`, `deployment.app_id`, `runtime_registration.app_id` to `NOT NULL` per workspace
- [x] 12.3 Run the flip per pilot workspace once the sentinel is set; verify with a smoke test
- [x] 12.4 After all pilot workspaces are migrated, enable feature flags globally and remove the per-workspace gates
