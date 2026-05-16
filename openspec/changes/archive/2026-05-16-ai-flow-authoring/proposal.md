## Why

The platform promised "n8n/Flowise-style" visual workflow authoring for non-technical users (see `ai-workflow-engine` requirement "Visual workflow editor in the Portal"), but what shipped is a YAML editor at `/workflows` and a JSON-textarea shell at `/workflows/editor`. The Flowise embed chosen in ADR-0001 was never installed (`flowise` and `reactflow` are absent from `portal/package.json`), the Phase 5 sign-off marked the box as done despite the deferred item "Live Flowise embed browser session", and two parallel surfaces now confuse authors. There is also a **silent catalog mismatch**: the Portal advertises 15 canonical node types, but the canonical Go AST in `pkg/workflow/ast/ast.go` only enumerates 8 (`skill`, `mcp`, `prompt`, `branch`, `loop`, `human-in-the-loop`, `sub-workflow`, `event-trigger`) and uses `prompt` where the Portal calls it `llm`/`prompt-template`. Triggers exist today only as a step kind (`event-trigger` with an `EventPattern`), not as a sibling block, and there is no opinionated **LLM node** bound to the model gateway with tool-calling. This change closes that gap end-to-end: it reconciles the canonical catalog, makes triggers a first-class authoring primitive, defines the LLM node, and ships an AI-Flow editor that is visually authorable, vendible, and extensible.

## What Changes

- **BREAKING** — Replace the Flowise embed decision (ADR-0001) with a custom canvas built on React Flow (`@xyflow/react`). Supersede ADR-0001 with a new ADR-0002. The Flowise adapter is renamed and refactored to a canvas-agnostic `ast-canvas-adapter`.
- **BREAKING** — Consolidate the two existing surfaces. `/workflows` becomes the AI-Flow library (list + version history + a YAML/code view tab); `/workflows/editor` becomes the visual canvas. The YAML-only editor at `/workflows` is no longer the primary authoring surface.
- **Reconcile the canonical step catalog.** Extend `pkg/workflow/ast.StepType` with the names the Portal already promises: `llm`, `agent`, `prompt-template`, `webhook` (outbound), `github-action`, `deploy-action`, `approval-action`, `notification-action`, `eval`, `custom`. The existing `prompt` step type is preserved as a deprecated alias of `prompt-template` (auto-migrated on save). The Go enum becomes the single source of truth; the TS catalog mirrors it.
- Introduce **triggers** as a first-class sibling block on `spec.triggers` (alongside `spec.inputs`, `spec.steps`). Initial trigger types: `manual`, `cron`, `webhook-in`, `event-bus`, `email-inbound`. The existing `event-trigger` step kind is preserved during the transition as a legacy shape: lint emits a deprecation warning at publish and the registry persists an auto-migration to the new `triggers` block (lossless because `EventPattern` maps cleanly onto `webhook-in`/`event-bus` trigger configs). The deprecated step kind is removed in a follow-up change.
- Introduce a new **trigger-router** service that subscribes to external sources (or hosts webhook receivers) and POSTs to `workflow-runtime` `/v1/executions` when a trigger fires.
- Specify the **LLM node** shape (prompt template binding, system message, tool-calling auto-bound to MCPs in scope, model selection via `model-gateway` / LiteLLM, declared output schema). Adding `StepLLM` to the canonical Go enum is part of this change — previously the Portal's TS catalog listed `llm` but the Go AST had no matching step type.
- Define the **custom node SDK** as a documented contract (interface, manifest format, packaging) without yet implementing community node ingestion. This keeps extensibility commitments explicit without expanding implementation scope. The `custom` step type is added to the Go enum so saved flows referencing custom nodes can persist.
- Add a reference workflow `forge.reference.ai-email-triage@1` exercising the full primitive set (email trigger → LLM classify+draft → branch → MCP webhook send) for use in demos and smoke tests.
- Correct the Phase 5 sign-off: move "visual editor (Flowise embed)" out of `[x]` exit criteria back into deferred until this change ships. Also flag the catalog mismatch as a previously-undocumented gap.
- Update the `workflow-visual-editor` spec to reference React Flow + the new node shapes; remove Flowise-specific requirements; keep all canvas-agnostic requirements (canonical AST persistence, live registry catalog, dry-run, version persistence, pinned-set behavior).
- Naming convention: user-facing copy refers to **"AI Flows"** (ES: "Flujos AI"). Internal/code identifiers keep `workflow`.

## Capabilities

### New Capabilities

- `automation-triggers`: First-class trigger primitives in the workflow AST plus the trigger-router service that subscribes to external sources and dispatches executions. Covers manual, cron, webhook-in, event-bus, and email-inbound trigger types.
- `llm-flow-node`: Concrete shape and runtime behavior for the `llm` step type, including prompt-template binding, tool-calling against in-scope MCPs, model selection via `model-gateway`, and declared output schema for downstream nodes.
- `custom-node-sdk`: Contract for community/customer-contributed node types — manifest format, interface, packaging, and lifecycle. Specification only; ingestion pipeline is out of scope for this change.

### Modified Capabilities

- `workflow-visual-editor`: Replace Flowise-specific requirements with React Flow + custom-canvas requirements. Add requirements for the trigger palette section and the LLM node configuration panel. Add explicit requirement for AI-Flow branding (user-facing naming).
- `ai-workflow-engine`: Update the "Catalog of node types" requirement to make trigger types first-class, separate from action steps. Update the visual-editor requirement to reference React Flow (canvas-agnostic wording).
- `workflow-dsl`: Extend the canonical AST to accept a `triggers` block alongside `inputs`/`steps`. Round-trip and lint requirements extend to triggers.
- `workflow-runtime`: Accept executions originating from the trigger-router with a `trigger_event` payload. Existing direct-POST executions continue to work.

## Impact

- **Code**:
  - `portal/package.json` — add `@xyflow/react` and remove the Flowise placeholder comments. Remove the `reactflow` suggestion in `editor.tsx`.
  - `portal/src/app/workflows/page.tsx` — refactor to library + version-history surface; move YAML editing into a "Code view" tab.
  - `portal/src/app/workflows/editor/page.tsx` + `EditorClient.tsx` — replace JSON textarea with React Flow canvas; add trigger palette section; add LLM node config panel.
  - `portal/src/lib/flowise-adapter/` → renamed to `portal/src/lib/ast-canvas-adapter/`. Round-trip tests stay; node shape extended for triggers and the new LLM shape. The TS adapter's `CanonicalStepType` is reduced to mirror the (now extended) Go `StepType` enum exactly.
  - `pkg/workflow/ast` — extend `StepType` enum with `llm`, `agent`, `prompt-template`, `webhook`, `github-action`, `deploy-action`, `approval-action`, `notification-action`, `eval`, `custom`. Keep `prompt` and `event-trigger` as deprecated aliases. Extend `Spec` with `Triggers []Trigger`. Add `Trigger` struct (`ID`, `Type`, `Config`, `Outputs`, `Concurrency`). Add LLM-specific fields to `Step` (or a typed LLM variant).
  - `pkg/workflow/dsl` — extend lint with the new rules; add an auto-migration step that converts `event-trigger` steps into the new `triggers` block on parse and emits a deprecation warning.
  - `services/workflow-registry` — accept the extended AST; version classification (MAJOR/MINOR/PATCH) extended to trigger and LLM-node changes.
  - `services/workflow-runtime` — accept `trigger_event` payloads; expose hook for trigger-router.
  - `services/trigger-router` — **new service**. Subscribes to event-bus, hosts webhook receivers, polls email/cron sources. Translates events into `workflow-runtime` execution POSTs with `trigger_event` payload.
  - `services/model-gateway` — expose a stable contract for the LLM node to bind models + credentials per workspace.
  - `services/mcp-gateway` — no API change; LLM node uses existing catalog for tool-calling.
  - `openspec/specs/workflow-visual-editor/spec.md`, `openspec/specs/ai-workflow-engine/spec.md`, `openspec/specs/workflow-dsl/spec.md`, `openspec/specs/workflow-runtime/spec.md` — modified per spec deltas.
  - New reference workflow: `services/workflow-registry/reference/ai-email-triage/1.0.0.yaml`.
- **APIs**:
  - New: `POST /v1/triggers` (trigger-router admin), `POST /v1/triggers/:id/fire` (manual test fire), trigger webhook receiver endpoints under `/v1/hooks/in/...`.
  - Extended: `POST /v1/workflows/:id/versions` accepts ASTs with `triggers` block.
  - Extended: `POST /v1/executions` accepts `trigger_event` field.
- **Governance**:
  - ADR-0001 (`docs/governance/adrs/0001-workflow-visual-editor.md`): status set to `Superseded by ADR-0002`.
  - New ADR-0002 (`docs/governance/adrs/0002-canvas-react-flow.md`): records the React Flow + custom canvas decision and rationale.
  - Phase 5 sign-off (`docs/governance/phase-5-signoff.md`): visual-editor exit criterion moved from `[x]` to deferred; reference to this change added.
  - License inventory (`docs/governance/licenses.md`): Flowise entry removed; React Flow (MIT) entry added.
- **Dependencies**:
  - Add: `@xyflow/react` (MIT) to `portal/package.json`.
  - Remove: any latent reference to Flowise in docs and code comments.
  - LGPL obligation tracked in licenses.md is removed.
- **Migration**:
  - Existing workflows without a `triggers` block remain valid (zero-trigger = invoke-only, current behavior).
  - The existing `/workflows` and `/workflows/editor` routes both stay functional during the transition; final routing consolidation happens at the end of the implementation plan.
- **Tests**:
  - Round-trip tests stay (renamed from `flowise-adapter` to `ast-canvas-adapter`); extended fixtures for triggers and the new LLM node shape.
  - New Playwright e2e: drag-build the `ai-email-triage` reference flow end-to-end and dry-run it.
  - New integration smoke: trigger-router fires an event → workflow-runtime executes.
