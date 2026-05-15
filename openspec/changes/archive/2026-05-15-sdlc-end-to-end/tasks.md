## 1. App entity `targets` map

- [x] 1.1 Add `targets` JSONB column on `application` with the documented defaults; ship migration that backfills existing Apps
- [x] 1.2 Validate target values on PATCH; reject unknown phase keys and unknown values; emit `app.updated.v1` with `before/after.targets`
- [x] 1.3 Surface `targets` in the App detail page (read for `app#viewer`, edit for `app#owner`)
- [x] 1.4 Update OpenAPI for App resource to include the `targets` schema

## 2. sdlc-architecture skills

- [x] 2.1 Scaffold `services/sdlc-architecture-skills` (Python/FastAPI) with the standard skill-server layout
- [x] 2.2 Implement `propose-adr` (MADR template, citation enforcement, decision-log append, ADR file persisted to App repo)
- [x] 2.3 Implement `evaluate-options` (≥2 options, ranked output with cited rationale, citation enforcer)
- [x] 2.4 Implement `check-openspec-alignment` (cross-check ADR vs OpenSpec requirements)
- [x] 2.5 Implement `generate-api-contract` (OpenAPI document, Spectral lint, PR commit)
- [x] 2.6 Implement `propose-data-model` (ER schema with sensitivity tags, persisted in `docs/data-model/`)
- [x] 2.7 Implement `lightweight-threat-model` (STRIDE-style report, persisted in `docs/threat-models/`)
- [x] 2.8 Build eval suites for each skill (30+ graded fixtures + adversarial fixtures); register T1 at publication
- [x] 2.9 Wire gates `adrs_published`, `api_contract_published`, `data_model_documented`, `threat_model_present`, `security_review_passed`, `openspec_updated` into the orchestrator

## 3. sdlc-design skills

- [x] 3.1 Scaffold `services/sdlc-design-skills` (Python/FastAPI)
- [x] 3.2 Implement `generate-ui-blueprint` (Figma export JSON, references the App's Design System)
- [x] 3.3 Implement `generate-component-stubs` (React + Vue templates, tokens via the resolved Design System, no hard-coded values)
- [x] 3.4 Implement `accessibility-audit` (Axe-core integration, severity classification, gate verdict)
- [x] 3.5 Build eval suites: blueprints (≥30 fixtures), stubs (≥30), a11y (≥30 with adversarial accessibility regressions)
- [x] 3.6 Wire gates `ui_blueprint_present`, `component_stubs_committed`, `accessibility_audit_passed` into the orchestrator

## 4. sdlc-qa skills

- [x] 4.1 Scaffold `services/sdlc-qa-skills` (Python/FastAPI)
- [x] 4.2 Implement `generate-test-plan` (API contract coverage + happy/error paths + perf when criticality≥high)
- [x] 4.3 Implement `generate-e2e-tests` (Playwright suite, one spec file per plan section)
- [x] 4.4 Implement `triage-test-failures` (top hypotheses + affected files + proposed minimal patch, with safety eval)
- [x] 4.5 Add the reactive triage hook listening on `ci.failed.v1`; rate-limit to one report per PR per 10 min; auto-fix PR opens only for `targets.qa in {required, autonomous}` and only when safety eval passes
- [x] 4.6 Build eval suites for each skill including adversarial CI-failure fixtures
- [x] 4.7 Wire gates `integration_tests_passing`, `e2e_tests_passing`, `perf_budget_met` into the orchestrator

## 5. Healing engine L1/L2

- [x] 5.1 Implement the L1 detect pipeline against the four signal sources (Prometheus, `ci.failed.v1`, `deployment.failed.v1`, alert thresholds, incident creations)
- [x] 5.2 Emit `healing.detected.v1` with diagnosis report (reuse `diagnosis-pipeline`), candidate hypotheses with citations, candidate actions, blast-radius estimate
- [x] 5.3 Render L1 detection cards in the Approvals Inbox tagged `healing-l1`
- [x] 5.4 Implement L2 propose-fix: pick top hypothesis, generate code diff OR config diff, file-level diff renderer in the Approvals Inbox
- [x] 5.5 Implement the safety eval: protected paths, size budget (default 200 lines, tenant-configurable), no secret references
- [x] 5.6 Wire downgrade-to-L1 on safety eval failure with `healing.fix_downgraded.v1`
- [x] 5.7 Wire approval transition to L3 (existing flow); record rejection reason + feed back into the L2 training corpus
- [x] 5.8 Wire L1/L2 events into the Friendly view's "Operar" card for plain-language summaries
- [x] 5.9 Eval suite for `propose-fix` (≥30 graded fixtures + red-team adversarial patches)

## 6. sdlc-infrastructure capability

- [x] 6.1 Scaffold `services/sdlc-iac` (Go) with skill-server layout
- [x] 6.2 Implement `generate-terraform` for AWS, GCP, Azure provider modules
- [x] 6.3 Implement `generate-helm-values` per criticality tier (small/medium/large) matched to the platform sizing doc
- [x] 6.4 Implement `validate-iac`: `terraform fmt`, `terraform plan`, `helm lint`, `helm template`, `conftest test` against the policy bundle
- [x] 6.5 Implement `apply-iac`: open PR with `terraform plan` + `helm template` + validation report; NEVER direct-apply
- [x] 6.6 Implement the GitOps runner integration that performs the actual apply on merge
- [x] 6.7 Implement `break_glass=true` flow with dual approval (security-admin + platform-admin)
- [x] 6.8 Build eval suites for each skill (Terraform module quality, Helm values vs sizing doc, validator correctness)
- [x] 6.9 Wire gates `iac_generated`, `iac_validated`, `iac_applied` into the orchestrator (only when `targets.iac != skipped`)

## 7. SDLC orchestration: targets semantics

- [x] 7.1 Extend the orchestrator plan builder to consult `App.targets` and the optional per-spec `targets_override`
- [x] 7.2 Implement the merge rule: spec override may only tighten, never relax
- [x] 7.3 Implement phase outcomes per target value (required = fail on gate fail, optional = warn, opt-in = skip unless included, skipped = remove from plan)
- [x] 7.4 Emit `sdlc.phase.skipped.v1`, `sdlc.phase.warning.v1`, `sdlc.phase.blocked.v1` with the relevant target snapshot
- [x] 7.5 Insert the Infrastructure phase between DevOps and SRE in the phase ordering
- [x] 7.x Wire new gates: `api_contract_published`, `data_model_documented`, `ui_blueprint_present`, `component_stubs_committed`, `accessibility_audit_passed`, `iac_generated`, `iac_validated`, `iac_applied`, `dashboards_provisioned`, `log_pipeline_active`, `tracing_enabled`

## 8. Workflow runtime: DSL `targets` extension

- [x] 8.1 Extend the workflow DSL parser to accept `targets:` map references on each phase step
- [x] 8.2 Update the DSL lint to validate referenced phase keys and target values
- [x] 8.3 Update the AST tests to round-trip the new field
- [x] 8.4 Document the DSL extension in `pkg/workflow/dsl/README.md`

## 9. Reference workflow `forge.reference.intent-to-infrastructure@1`

- [x] 9.1 Author the workflow YAML under `workflows/forge/reference/intent-to-infrastructure@1.yaml`
- [x] 9.2 Register it in the workflow registry as `kind=reference`, tagged `forge`
- [x] 9.3 Ensure the step order matches the documented order (intent → architect → design → development → qa → security → devops → iac → deploy → sre → observability)
- [x] 9.4 Each step emits its documented event; correlation_id, app_id, openspec_id consistent across the run
- [x] 9.5 HITL inheritance: prod deploy and L2 healing fixes always pause for approval per policy
- [x] 9.6 Build `make demo-intent-to-infrastructure` Makefile target; produce JSON report at `build/demo-intent-to-infrastructure/<timestamp>.json`
- [x] 9.7 Smoke test in CI against ephemeral infra; assert the full milestone chain with consistent correlation
- [x] 9.8 Author `docs/runbooks/intent-to-infrastructure-demo.md`

## 10. Friendly view integration

- [x] 10.1 Wire the Friendly view's "Nueva App" card to start `forge.reference.intent-to-infrastructure@1` (after `alfred-console-redesign` ships)
- [x] 10.2 Wire the "Operar" card to surface healing L1/L2 events in plain language with friendly labels
- [x] 10.3 Visual regression baselines in both themes

## 11. Observability and rollout

- [x] 11.1 Feature flags: `forge.sdlc.architecture_skills.enabled`, `forge.sdlc.design_skills.enabled`, `forge.sdlc.qa_skills.enabled`, `forge.sdlc.iac.enabled`, `forge.healing.l1_l2.enabled`, `forge.workflow.intent_to_infrastructure.enabled` — per-tenant, default `false`
- [x] 11.2 Dashboards: per-phase skill invocation rate, gate pass/fail ratios, L1/L2 detection volume, L2 approve/reject ratio, IaC PR open/merge ratios
- [x] 11.3 SLOs: each skill p95 < 30s; reactive triage posted within 90s of `ci.failed.v1`; IaC validator < 60s
- [x] 11.4 Runbook: per-tenant rollout sequence (platform → 2 pilots → global)

## 12. Documentation

- [x] 12.1 Update `docs/platform-enablement.md` with the new phases and the `targets` matrix
- [x] 12.2 Author skill READMEs for each new skill (input/output schemas, eval baseline, T1 promotion criteria)
- [x] 12.3 Update `openspec/specs/sdlc-orchestration` purpose to reflect the new phase ordering after archive
- [x] 12.4 Update CLAUDE.md repository guidance with the new reference workflow and its make target

## 13. Registry sync for public-origin assets

- [x] 13.1 Split `IsPublic bool` in `pkg/artifact-store-adapter` Health into `IsPublicOrigin bool` and `IsPublicStorage bool`; update binding layer to reject only `IsPublicStorage=true`; add unit tests for both flag paths
- [x] 13.2 Add `origin_ref` (e.g., `npm:my-skill@1.2.3`), `is_public_origin bool` and `last_synced_at` columns to the registry asset row; ship migration that backfills existing rows with `is_public_origin=false`
- [x] 13.3 Implement the mirror-on-register flow: fetch bytes from origin URL, verify sha256 against origin-declared checksum, store via Tenant adapter, set `lifecycle_state=mirrored`, emit `asset.version.mirrored.v1`
- [x] 13.4 Add `lifecycle_state=mirrored` to the asset state machine; validate allowed transitions (`mirrored → approved`, `mirrored → rejected`); emit `asset.version.promoted.v1` on approve
- [x] 13.5 Add `auto_promote_policy` field on asset (`none` | `patch` | `minor`); major version bumps always require manual confirmation regardless of policy; surface the field in the asset settings UI
- [x] 13.6 Implement Sync Worker: periodic job (configurable per-Tenant, default weekly) that paginates registered public-origin assets, queries the npm/GitHub registry APIs for the latest version, compares to registry, triggers mirror+notify on drift; batch queries ≤100 req/min per origin; emit `registry.sync.completed.v1` with checked/drifted/mirrored counts
- [x] 13.7 Expose webhook receiver endpoints `POST /v1/registry/webhooks/npm` and `POST /v1/registry/webhooks/github`; validate payload signatures; trigger the mirror flow on new-release events; deduplicate against already-mirrored versions
- [x] 13.8 Add promotion confirmation UI in Portal: mirrored-version card on the asset detail page with origin_ref, version diff, last_synced_at, confirm/reject actions; wire auto-promote path to skip this card when policy applies
- [x] 13.9 Add **PUBLIC ORIGIN** badge to the asset catalog and asset detail pages (tooltip shows `origin_ref` + `last_synced_at`); gate on `is_public_origin=true`
- [x] 13.10 Wire `mcp-gateway` to compare live `tools/list` against registry cache on every new client session; emit `mcp.tool_list.drifted.v1` with before/after diff; update registry cache immediately; notify asset owner
- [x] 13.11 Feature flag: `forge.registry.public_origin.enabled` per-Tenant, default `false`; add to the rollout sequence in task 11.4
- [x] 13.12 Observability: add Sync Worker metrics (`sync_assets_checked`, `sync_assets_drifted`, `sync_mirrors_triggered`, `sync_duration_seconds`) to the dashboards in task 11.2
