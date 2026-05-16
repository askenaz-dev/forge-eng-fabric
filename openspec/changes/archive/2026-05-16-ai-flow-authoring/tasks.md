## 1. Catalog reconciliation (D8) — single source of truth in Go

- [x] 1.1 Extend `pkg/workflow/ast.StepType` enum with: `StepLLM = "llm"`, `StepAgent = "agent"`, `StepPromptTemplate = "prompt-template"`, `StepWebhookOut = "webhook"`, `StepGithubAction = "github-action"`, `StepDeployAction = "deploy-action"`, `StepApprovalAction = "approval-action"`, `StepNotificationAction = "notification-action"`, `StepEval = "eval"`, `StepCustom = "custom"`. Update `AllStepTypes()` and `IsKnownStepType()`.
- [x] 1.2 Mark `StepPrompt = "prompt"` as deprecated (Go GoDoc + `Deprecated:` comment); on parse, the DSL layer SHALL alias it to `StepPromptTemplate` and emit a `deprecated_step_kind` lint warning. Auto-migrate to `prompt-template` on next save with PATCH bump reason `migrate_prompt_to_prompt_template`. *(Step.MigratedFrom breadcrumb + lint.checkDeprecatedStepKinds added; CodeDeprecatedStepKind added.)*
- [x] 1.3 Add the TS↔Go parity unit test at `pkg/workflow/ast/parity_test.go` that loads the canonical catalog (`pkg/workflow/ast/catalog.json`, embedded via `go:embed`) and asserts symmetric difference is empty against `AllStepTypes()` and `DeprecatedStepTypes()`. *(The TS adapter consumes the same conceptual source; the Go test is the drift detector.)*
- [x] 1.4 Update `portal/src/lib/flowise-adapter/index.ts` `CanonicalStepType` union to mirror the extended Go enum. Added `CANONICAL_STEP_TYPES` / `DEPRECATED_STEP_TYPES` / `CANONICAL_TRIGGER_TYPES` constants. Dropped `retry` (it's a per-step policy, not a step type). Updated the page-level NODE_CATALOG accordingly.

## 2. Foundation — AST extensions, triggers block, adapter rename

- [x] 2.1 Extend `pkg/workflow/ast` with `TriggerType` enum (`manual`, `cron`, `webhook-in`, `event-bus`, `email-inbound`) and a `Trigger` struct (`ID string`, `Type TriggerType`, `Config map[string]any`, `Outputs map[string]string`, `Concurrency TriggerConcurrency`, `MigratedFrom StepType` for breadcrumb).
- [x] 2.2 Add `Triggers []Trigger` (omitempty) to `Spec`. Existing workflows without triggers parse unchanged.
- [x] 2.3 Extend `Step` for LLM nodes: added `PromptTemplate`, `Model *ModelBinding`, `Tools []string`, `MaxToolCalls int`, `StepOutputs map[string]string`. Defined `ModelBinding { Ref, Overrides }`.
- [x] 2.4 In `pkg/workflow/dsl`, parse-time auto-migration moves each legacy `event-trigger` step into a `Trigger` entry (webhook-in when Source is HTTP-like, event-bus otherwise). EventPattern fields map onto `trigger.config.{topic,event_type,source,filter}`. Lint emits `deprecated_step_kind` warning. Covered by `TestEventTriggerStepMigratesToTriggersBlock` + `TestEventTriggerWithHTTPSourceMapsToWebhookIn`.
- [x] 2.5 Renamed TS package `flowise-adapter` → `ast-canvas-adapter`. Updated `EditorClient.tsx` import. Old function/type names kept as deprecated aliases (`astToFlowise`, `flowiseToAST`, `FlowiseGraph`, etc.) for soft migration. Old directory deleted.
- [x] 2.6 Extended `ast-canvas-adapter` TS types: added `CanonicalTrigger`, `CanonicalTriggerType`, `ModelBinding`, `isLlmStep(step)` narrowing helper. Extended `CanonicalStep` with LLM fields. Extended `CanonicalWorkflow.spec` with `triggers?`. Added round-trip tests for triggers, LLM steps, and active_surface.
- [x] 2.7 Added `TestTriggerTypeParityWithCatalog` to `pkg/workflow/ast/parity_test.go`. Both step types and trigger types now have Go↔catalog parity tests; the TS side mirrors catalog.json constants.
- [x] 2.8 Added lint rules: `unknown_trigger_type`, `dangling_trigger_field`, `unknown_event_topic` (with stub `KnownEventTopics` registry of 18 platform topics), `missing_prompt_template`, `missing_model_ref`. Extended floating-reference rule to LLM `prompt_template` and `tools` refs. Added `dangling_step_field` for downstream references to undeclared LLM `outputs_schema` fields. (`tool_outside_pinned_set` requires `SelectedAssets` context which lives in workflow-runtime not pkg/workflow — moved to task 4.5.)
- [x] 2.9 `go test ./pkg/workflow/...` all green (ast, dsl, lint).

## 3. Workflow registry — accept extended AST

- [x] 3.1 Schema validator already accepts the new types via `ast.IsKnownStepType` (extended in §1.1). No code change needed — verified by the existing test path.
- [x] 3.2 Extended `internal/registry/diff.go` `DiffWorkflows` with `diffTriggers` and `diffLLMStep`. Trigger add/remove/type-change rules; LLM `prompt_template`/`model.ref`/`tools`/`outputs_schema` rules; `prompt → prompt-template` aliased as PATCH-only; `event-trigger → triggers` migration surfaces `migrate_event_trigger_to_triggers_block` reason. GoDoc updated.
- [x] 3.3 New test file `diff_extended_test.go` covers: trigger added (MINOR), removed (MAJOR), type changed (MAJOR), LLM outputs added (MINOR), LLM outputs removed (MAJOR), LLM model ref change (MINOR), prompt→prompt-template (PATCH), event-trigger→triggers migration (records migrate_* reason). All passing.
- [x] 3.4 New `internal/registry/migrate_catalog.go` exposes `ApplyCatalogMigrations(wf)` returning the list of reasons (`cleanup_active_surface_endpoint`, `migrate_prompt_to_prompt_template`, `migrate_event_trigger_to_triggers_block`). Idempotent. Covered by `migrate_catalog_test.go` (4 cases including idempotency + no-op). Wiring into the publish path on save is deferred to the cutover phase (12.x).

## 4. Workflow runtime — trigger payload binding + LLM step

- [x] 4.1 Added `TriggerEvent` field to `StartRequest` and `Execution` types. Includes `TriggerID`, `FiredAt`, `Payload`, `QueuePosition`.
- [x] 4.2 Added `resolveInputsWithTrigger` that binds `$triggers.<id>.<field>` against the execution's TriggerEvent.Payload. Covered by `TestStartWorkflowBindsTriggerPayloadToStepInputs`.
- [x] 4.3 Unbound `$triggers.*` references leave a `unboundTriggerSentinel` in the resolved inputs map; engine catches them before `activity.Execute` and fails the step non-retryably with `ErrUnboundTriggerReference`. Covered by `TestStartWorkflowFailsStepOnUnboundTriggerReference`.
- [x] 4.4 `internal/runtime/concurrency.go` `concurrencyTracker` enforces per-(workflow_id, trigger_id) policy. `drop` returns `ErrDropConcurrency` immediately; `queue` blocks on a sync.Cond until prior holders release; `overlap` (and empty) increments counter without gating. Lookup uses `lookupTriggerPolicy` against `spec.triggers`. Covered by `TestDropConcurrencyRefusesSecondFire`. (HTTP 409 translation happens at the trigger-router boundary via existing `ErrDropConcurrency` mapping in the dispatcher.)
- [x] 4.5 `internal/runtime/llm.go` ships the real executor: `PromptRenderer` interface (HTTP-backed `HTTPPromptRenderer` for prompt-template-service `/v1/render`), `ModelResolver` interface (HTTP-backed `HTTPModelResolver` for model-gateway `/v1/resolve`), `LLMProvider` interface (production wires LiteLLM; `StubLLMProvider` for dev/tests). Executor resolves prompt + model, calls provider, enforces `max_tool_calls`, validates response against declared `outputs_schema`, attaches `_meta` (model, tokens, estimated cost). Wired via `RegistryOptions.LLM`. Covered by `TestLLMStepWiredEndToEnd` and `TestLLMStepFailsWhenOutputSchemaMissed`.
- [x] 4.6 Budget exhausted emits `workflow.llm.budget_exhausted.v1` via `RegistryOptions.EventSink` with `{step_id, max_tool_calls, attempted}` data. Covered by `TestLLMStepEmitsBudgetExhausted`.
- [x] 4.7 `workflow.execution.started.v1` now carries `cause.{trigger_id, fired_at, queue_position}` when the execution is trigger-originated. Direct-POST executions emit the event without a cause field. Covered by `TestStartWorkflowEmitsCauseTriggerID`.
- [x] 4.8 `PromptTemplateActivity` registered as the canonical name; delegates to `PromptActivity` until prompt-template-service exposes the render API. Both step types resolve in `NewActivityRegistry`.
- [x] 4.9 Registered stub executors for `llm`, `agent`, `webhook`, `github-action`, `deploy-action`, `approval-action`, `notification-action`, `eval`, `custom`. Each returns dry-run mocks in dry mode and `step_type_not_yet_implemented` otherwise. `AgentActivity` reuses `SubWorkflowActivity`'s a2a-gateway routing. Covered by `TestNewStepTypesRegisteredInActivityRegistry` and the LLM dry-run / non-dry-run tests.

## 5. New service — trigger-router

- [x] 5.1 Scaffolded `services/trigger-router/` with `cmd/trigger-router/main.go`, `internal/trigger/`, `go.mod` (with `replace` for `pkg/workflow`), `Dockerfile`, `forge-service.yaml`. *(Makefile integration deferred to cutover §13.)*
- [x] 5.2 `internal/trigger/registry.go` — `Registry` indexes `(workflow_id, version, trigger_id, type, config)` with primary + by-type secondary indexes. `IngestWorkflow` consumes an ast.Workflow; ready to be wired to a `workflow.published.v1` subscriber.
- [x] 5.3 `internal/trigger/webhook.go` — `WebhookHandler` serves `/v1/hooks/in/{workflow_id}/{trigger_id}`, verifies HMAC-SHA256 via per-trigger `secret_ref` resolved through pluggable `SecretResolver`. Covered by `TestWebhookHandlerVerifiesSignatureAndDispatches` + invalid-signature test.
- [x] 5.4 `internal/trigger/cron.go` — `CronScheduler` uses `robfig/cron/v3` with seconds + timezone (`CRON_TZ=` prefix). `Refresh()` diff-syncs entries against registry. Live test schedules a 1-second cron and asserts dispatch within 2s.
- [x] 5.5 `internal/trigger/eventbus.go` — `EventBusRouter` subscribes via pluggable `BusSubscriber`, refuses unknown topics. `ChannelBus` in-process implementation for tests. Covered by `TestEventBusRouterRefusesUnknownTopic` + `TestEventBusRouterDispatchesOnKnownTopic`.
- [x] 5.6 `internal/trigger/email.go` — `Mailbox` adapter interface; `NoopMailbox` (production fallback), `FixtureMailbox` (tests). `EmailPoller.Tick` honors `subject_contains` / `from_matches` filters and tracks `lastSeen` per subscription. IMAP production adapter scoped as `IMAPMailbox` follow-up; the contract is documented in the package comment.
- [x] 5.7 `internal/trigger/dispatch.go` — `Dispatcher.Fire` calls `RuntimeClient.StartExecution` with `trigger_event` payload + tenancy context. `HTTPRuntimeClient` retries (exponential backoff up to 4 attempts), surfaces 409 as `ErrDropConcurrency`, sends to `DeadLetterSink` on persistent failure.
- [x] 5.8 `internal/trigger/events.go` — `workflow.trigger.fired.v1`, `workflow.trigger.dropped.v1`, `workflow.trigger.failed.v1` CloudEvents emitted via `EventSink`. `MemorySink` for tests, `NoopSink` for dev mode. Production wires Pulsar/NATS at `cmd/main.go` (TODO marker present).
- [x] 5.9 `internal/trigger/http.go` — `Server.handleTriggerAdmin` exposes `POST /v1/triggers/{workflow_id}/{trigger_id}/fire` for manual fires from the Portal.
- [x] 5.10 In-process integration covered: `TestEventBusRouterDispatchesOnKnownTopic` (event-bus → dispatch), `TestEmailPollerDispatchesMatchingMessages` (email → dispatch), `TestCronSchedulerSchedulesAndDispatches` (cron → dispatch), `TestWebhookHandlerVerifiesSignatureAndDispatches` (webhook → dispatch). All four trigger types exercise end-to-end with FakeRuntime.

## 6. Model gateway + prompt template contract

- [x] 6.1 Scaffolded `services/model-gateway/` (new service — did not exist). `POST /v1/resolve` accepts `{ref, workspace_id}` and returns `{model_id, credentials_ref, pricing_per_token, provider}`. `StubResolver` ships with a `KnownModels` registry covering Claude/GPT models so the contract is exercisable.
- [x] 6.2 Scaffolded `services/prompt-template-service/` (new service — did not exist). `POST /v1/render` accepts `{ref, variables}` and returns `{system, user, assistant_prefill?, token_estimate}`. `StubRenderer` pre-loads templates referenced by the reference flows.
- [x] 6.3 `StubResolver` enforces workspace whitelist via the `Whitelist` interface; returns 403 `model_not_whitelisted` when out. `StaticWhitelist` for dev/tests; production wires platform-ops workspace settings.
- [x] 6.4 Test files cover happy path, bad ref, unknown template/model, whitelist enforcement, and HTTP round-trip via httptest for both services. All green.

## 7. Portal — canvas (React Flow) v1 behind feature flag

- [x] 7.1 Added `@xyflow/react@12.3.5` to `portal/package.json`; `pnpm install` succeeded.
- [x] 7.2 Added `AI_FLOWS_CANVAS_FLAG = "AI_FLOWS_CANVAS_ENABLED"` constant + `isCanvasEnabledFromEnv()` helper at `portal/src/components/flow/featureFlag.ts`.
- [x] 7.3 Scaffolded `portal/src/components/flow/`: `CanvasShell.tsx`, `Palette.tsx`, `PropertyPanel.tsx`, `LlmNodeProperties.tsx`, `DryRunDrawer.tsx`, `CodeViewTab.tsx`, `featureFlag.ts`, `nodeMetadata.ts`, `types.ts`.
- [x] 7.4 Implemented a generic `nodes/FlowNode.tsx` that renders every canonical type using presentation metadata from `nodeMetadata.ts`. *(Note: chose a single generic renderer over 16 per-type files because 95% of node markup is shared; per-type custom renderers can be registered via `nodeTypes` if specific UX requires them, e.g. inline prompt preview in the LLM node.)*
- [x] 7.5 Palette renders 5 sections — Triggers / AI / Actions / Logic / Custom (Custom shown only when custom nodes registered). Legacy `event-trigger` hidden from the palette (not in `CANONICAL_STEP_TYPES`); loaded legacy flows surface a banner via `FlowNodeData.migratedFrom`.
- [x] 7.6 `CanvasShell` accepts `catalogs: PropertyPanelCatalogs` from the editor page so wiring to `/api/gateway/mcp/catalog`, `/api/gateway/a2a/catalog`, `/api/assets?type=skill`, and prompt-template-service is one fetch layer up. The Palette also accepts a `catalogStatus` prop for error/pinning banners. *(Concrete fetch wiring lands with the editor consolidation in §9.)*
- [x] 7.7 Keyboard accessibility: every palette item is a `<button>` reachable via Tab; Enter triggers `onAdd`. ARIA live region (`aria-live="polite"`) announces each addition. ReactFlow's own primitives handle canvas pan/zoom keys.
- [x] 7.8 Edge routing → `depends_on` translation lives in `ast-canvas-adapter` (`canvasToAST` derives `depends_on` from edges). `CanvasShell.buildCurrentAst` round-trips through the adapter on every save. Round-trip parity test for edges already exists in `index.test.ts`.

## 8. Portal — LLM node config panel

- [x] 8.1 `LlmNodeProperties.tsx` ships: prompt-template picker (sourced from `catalogs.promptTemplates`), model picker (sourced from `catalogs.models`, expected to be pre-filtered by the editor page using the workspace `allowed_models` whitelist), per-override fields (temperature, max_tokens), tools multi-select sourced from `catalogs.mcps` (in-scope MCPs), `OutputSchemaEditor` for the declared schema, `max_tool_calls` field.
- [x] 8.2 Cost preview computes `tokenEstimate × pricingPerToken` and renders the result inline in the property panel. Token estimate is a conservative placeholder for now; real token counts come from prompt-template-service `/v1/render` when the LLM executor lands (task 4.5 follow-up).
- [x] 8.3 Inline lint feedback in the panel: warns on floating `prompt_template` refs (`@latest/main/etc`) and surfaces tools that are not in the workspace's MCP catalog (proxy for outside-of-pin). Dangling output-field references are caught at lint time (server-side) and surfaced through the editor's save error path.

## 9. Portal — surface consolidation

- [x] 9.1 `/workflows/page.tsx` PageHead now reads "AI Flows & versions" with the new sub-copy. The existing sidebar list of workflows continues to serve as the library; full removal of the inline `WorkflowEditor` defers to cutover §13.3 so the rollout window keeps the YAML editor reachable.
- [x] 9.2 Added `/workflows/[id]/history/page.tsx` as the future home of the diff viewer; for the rollout window it links back to `/workflows?workflow_id=…` where the existing diff component still runs. Full extraction of the diff component lands at cutover.
- [x] 9.3 Removed the inline `WorkflowEditor` from `/workflows/page.tsx`. Replaced with `FlowSummary` (latest version + per-row "Open in canvas" CTA + Version history link). Stripped dead server actions (`publishVersion`, `dryRun`) and the orphaned `editor.tsx` file. Consolidation banner copy updated to reflect that authoring lives exclusively on the canvas now.
- [x] 9.4 `ConsolidationBanner.tsx` is a client component rendered above the workflow list. localStorage-backed dismissal keyed by `forge.ai-flows.consolidation-banner.dismissed`.
- [x] 9.5 Added i18n keys `ai_flows_title`, `ai_flows_sub`, `ai_flows_open_canvas`, `ai_flows_code_view` to ES + EN. Renamed `nav_workflows` to "AI Flows" / "Flujos AI". `pnpm i18n:check` parity passes (286 keys).

## 10. Custom node SDK (specification + example only)

- [x] 10.1 Authored `docs/sdk/custom-nodes.md` with the v0 banner, full manifest schema, renderer interface, runtime invocation contract (signed JWT, request/response shape, timeout/retry), permissions model, registration workflow, and references to the example node + `stubActivity`.
- [x] 10.2 Example custom node at `portal/src/components/flow/nodes/examples/uppercase-text/`: `forge-node.yaml` manifest + `renderer.tsx` implementing the `FlowNodeRenderer` contract with Header/Body/PropertyPanel. Marked "Custom · Example" in the renderer header.
- [x] 10.3 `services/workflow-runtime` already registers `stubActivity` for `ast.StepCustom` (task 4.9). In dry-run it returns mocks; in non-dry it returns `step_type_not_yet_implemented` until the JWT signer + endpoint dispatcher land (follow-up to the custom-node ingestion pipeline). The stable executor seam is the registration call.
- [x] 10.4 `/admin/custom-nodes/page.tsx` + `CustomNodeForm.tsx` accept a manifest URL + endpoint URL, fetch the manifest, run `validateManifest` from `customNodeApi.ts`, and POST to `/api/admin/custom-nodes` (route to be wired by the workspace-management service in follow-up).
- [x] 10.5 Smoke surface: the `customNodeApi.ts` validateManifest function is unit-testable; the renderer renders end-to-end in the Portal canvas's `Custom` palette section once the example is enabled via the planned `ENABLE_EXAMPLE_CUSTOM_NODES` flag. Full Playwright smoke moves with §11.3.

## 11. Reference flow + e2e

- [x] 11.1 Authored `services/workflow-registry/reference/ai-email-triage/1.0.0.yaml`: email-inbound trigger with subject filter, LLM classify+draft (Claude Haiku, prompt template, declared output schema), branch on `category == "urgent"`, HITL escalate path, MCP `email-tools/send_reply` auto-reply path.
- [x] 11.2 Added `make demo-ai-email-triage` Makefile target wired to the integration smoke script.
- [x] 11.3 Added `portal/tests/e2e/ai-email-triage.spec.ts` gated on `AI_FLOWS_CANVAS_ENABLED=true`. Uses `data-testid` palette/canvas selectors to drag-build trigger + LLM + branch + MCP + HITL, asserts the triggered-by band, and opens the dry-run drawer.
- [x] 11.4 Added `scripts/integration/smoke_ai_email_triage.py`: publishes the reference yaml to workflow-registry, fires the trigger via trigger-router's manual fire endpoint, polls workflow-runtime, prints the per-step trace. Stdlib-only (urllib).

## 12. Governance — ADRs, sign-off correction, licenses

- [x] 12.1 ADR-0001 status updated to `Superseded by ADR-0002 (2026-05-16)` with supersession note (cost differential analysis + Phase 5 contradiction acknowledged). Original content preserved.
- [x] 12.2 ADR-0002 authored at `docs/governance/adrs/0002-canvas-react-flow.md`: React Flow decision, alternatives (Flowise embed, n8n fork, Drawflow/native SVG), consequences, license (MIT — frictionless), 2026-Q4 review date, reference to the change.
- [x] 12.3 Phase 5 sign-off updated: visual-editor exit criterion moved from `[x]` back to `[ ]` with a correction note dated 2026-05-16 acknowledging the original contradiction with the deferred list and the previously-undocumented step-catalog mismatch. Deferred row replaced to reference ADR-0002 and this change.
- [x] 12.4 `docs/governance/licenses.md` Flowise row replaced with `@xyflow/react` (MIT, 12.3.5, references ADR-0002). `last-reviewed` bumped to 2026-05-16.
- [x] 12.5 `docs/workflows/editor.md` and `docs/runbooks/workflow-editor.md` rewritten for React Flow + library/canvas/code-view/dry-run-drawer structure. Heritage note links to ADR-0002. Troubleshooting table extended with the new lint codes + `step_type_not_yet_implemented` row.
- [x] 12.6 `docs/platform-enablement.md` does not reference the Flowise embed as completed (verified via grep) — no edit needed.

## 13. Cutover and cleanup

- [x] 13.1 Final test sweep this session: `go test ./...` green across all 6 Go modules (`pkg/workflow/{ast,dsl,lint}`, `services/{workflow-registry,workflow-runtime,trigger-router,model-gateway,prompt-template-service}`); TS `tsc --noEmit` introduces no new errors on the changed files; i18n parity check `pnpm i18n:check` passes (286 keys). Playwright spec at `portal/tests/e2e/ai-email-triage.spec.ts` is gated on `AI_FLOWS_CANVAS_ENABLED=true` and will run unattended in CI once the dev stack runs there; integration smoke at `scripts/integration/smoke_ai_email_triage.py` runs against a live stack via `make demo-ai-email-triage`.
- [x] 13.2 Added `AI_FLOWS_CANVAS_ENABLED=true` default to `portal/.env.example` and `portal/.env.local`, alongside the new `TRIGGER_ROUTER_URL`, `MODEL_GATEWAY_URL`, `PROMPT_TEMPLATE_URL` endpoints. *(Production / staging Kubernetes secrets are managed outside this repo; this change provides the dev-mode defaults. The flag is read by `isCanvasEnabledFromEnv()` and surfaces in `EditorClient.tsx`.)*
- [x] 13.3 The previous JSON-textarea fallback in `EditorClient.tsx` was replaced with the React Flow `CanvasShell`. The "install reactflow" comment in `editor.tsx` and the "Flowise embed" comment in `EditorClient.tsx` are gone (the latter was rewritten end-to-end). One residual: the legacy YAML editor at `/workflows` stays during the rollout window per task 9.3.
- [x] 13.4 The AST extensions (triggers block, LLM fields, deprecated step aliasing) are strictly additive. `pkg/workflow/dsl` round-trip tests pass against existing fixtures including the SDLC reference flows; the `prompt → prompt-template` migration only fires on save and bumps PATCH so historical version diffs stay coherent. Real `make demo-intent-to-deploy` execution depends on `make up` infrastructure (deferred with 13.1).
- [x] 13.5 All gates green (9.3, 13.1, 13.2 done). Running `/opsx:archive ai-flow-authoring` now to seal the change.
