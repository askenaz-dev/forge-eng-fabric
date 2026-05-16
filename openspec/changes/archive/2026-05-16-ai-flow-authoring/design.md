## Context

The platform's promise of "n8n/Flowise-style" visual authoring was made in `ai-workflow-engine` and `workflow-visual-editor`. ADR-0001 (2026-05-09) picked Flowise embed over n8n-fork and build-own. The adapter, persistence shell, gateway-catalog palette, runtime dry-run and version diff all landed. The Flowise embed itself did not — `@xyflow/react`, `flowise`, and `reactflow` are absent from `portal/package.json`, and `EditorClient.tsx` documents the gap in a code comment. Phase 5 sign-off marked the box `[x]` while the deferred items list "Live Flowise embed browser session" — i.e. the entire feature.

Today two surfaces exist:

- `/workflows` — YAML editor with ASCII graph preview, diff viewer, lint, dry-run (`portal/src/app/workflows/page.tsx` + `editor.tsx`).
- `/workflows/editor` — palette of gateway catalog assets + JSON textarea + version persistence (`portal/src/app/workflows/editor/page.tsx` + `EditorClient.tsx`).

**A silent catalog mismatch was discovered while reading the real Go AST (`pkg/workflow/ast/ast.go`):**

| Layer | What it enumerates |
|---|---|
| Portal TS catalog (`portal/src/lib/flowise-adapter/index.ts`) | `llm`, `mcp`, `skill`, `agent`, `prompt-template`, `human-in-the-loop`, `branch`, `loop`, `retry`, `eval`, `webhook`, `github-action`, `deploy-action`, `approval-action`, `notification-action` (15 types) |
| Portal page palette (`portal/src/app/workflows/editor/page.tsx`) | Same 15 types listed as `NODE_CATALOG` |
| Canonical Go AST (`pkg/workflow/ast.StepType`) | `skill`, `mcp`, `prompt`, `branch`, `loop`, `human-in-the-loop`, `sub-workflow`, `event-trigger` (8 types) |

This means several Portal-promised types have **no execution support in the runtime** today. In particular: `llm` does not exist in the Go enum (Go uses `prompt`); `agent`, `prompt-template`, `webhook`, `github-action`, `deploy-action`, `approval-action`, `notification-action`, `eval`, `custom` are all Portal-only. Triggers do already exist — but as a step kind (`event-trigger` with `EventPattern { Type, Source, Filter }`) rather than as a sibling block to `steps`. This change reconciles the two surfaces.

The user-stated vision: AI-orchestrated automations (email arrives → LLM classifies + drafts → conditional logic → microservice call), built visually by non-technical users, with all assets and credentials managed inside Forge. Three things are missing or unspecified after the reconciliation:

1. **Triggers as a first-class sibling block.** The current `event-trigger` step kind works for webhook/event-bus cases but conflates "what fires this flow" with "what runs in it" and gives no clean home for `cron`/`email-inbound`/`manual` triggers.
2. **The LLM node shape.** `llm` is named in the Portal catalog but has no Go enum, no parser support, no runtime executor, no specified shape (prompt-template binding, tool-calling contract, model-gateway integration).
3. **A real canvas.** Both existing surfaces are placeholder UX.

Stakeholders: Platform Architecture (owns AST/runtime), Frontend Engineering (owns Portal), Security (owns gateway/credentials), Product (owns the AI-Flow brand and the non-techie experience).

Constraints:

- The canonical AST in `pkg/workflow/ast` and the version-classification rules in `services/workflow-registry/internal/registry` are load-bearing for ~80 existing OpenSpec changes and the SDLC reference flows. Backward compatibility is required: existing `prompt` and `event-trigger` steps must remain parseable after this change.
- The platform speaks Spanish first (see `portal/src/i18n/dictionary.ts` and project CLAUDE.md). User-facing naming must land in ES with EN fallback.
- The Phase 5 sign-off cannot quietly be amended — the correction is part of this change's deliverables. The catalog mismatch likewise must be acknowledged in the correction note, not just silently fixed.

## Goals / Non-Goals

**Goals:**

- Replace ADR-0001's Flowise decision with a custom canvas built on React Flow (`@xyflow/react`), captured in a new ADR-0002.
- Make `triggers` a first-class primitive in the workflow AST, runtime, and editor. Ship 5 trigger types (manual, cron, webhook-in, event-bus, email-inbound) routed by a new `trigger-router` service.
- Specify and implement the LLM node: prompt-template binding via `prompt-template-service`, tool-calling auto-bound to MCPs in the workflow's scope, model selection via `model-gateway`, declared output schema.
- Consolidate `/workflows` (library + history) and `/workflows/editor` (canvas) without dropping the YAML "Code view" capability.
- Ship a reference workflow `forge.reference.ai-email-triage@1` that exercises trigger + LLM + branch + MCP action, plus a Playwright e2e that drag-builds it and dry-runs it.
- Document the custom-node SDK contract (interface, manifest, packaging) without implementing the ingestion pipeline.
- Correct the Phase 5 sign-off and the license inventory.

**Non-Goals:**

- Implementing the community-node ingestion pipeline (registry, signing, distribution). The SDK is documented; the ingestion service is a follow-up.
- Replacing the canonical AST. The AST is extended, not rewritten.
- Building MCPs for the integrations listed in the email demo (Gmail, Slack, etc.). The reference flow uses a generic `webhook` MCP; production MCPs for popular SaaS are out of scope.
- Migrating existing workflows. Pre-existing workflows without a `triggers` block remain valid and invoke-only.
- Multi-canvas authoring (sub-flows, nested workflows). Stays a follow-up.
- Real-time multi-user collaborative editing on the canvas. Single-author-at-a-time with optimistic save.

## Decisions

### D1: Canvas library — React Flow (`@xyflow/react`)

**Decision:** Adopt `@xyflow/react` (MIT) as the canvas library. Implement custom node renderers in `portal/src/components/flow/nodes/`. Persist canonical AST via the renamed `ast-canvas-adapter`.

**Alternatives considered:**

- **Embed Flowise (ADR-0001's choice):** LGPL forces publishing mods to the fork; Flowise UI never matches Forge brand; upstream-pinning becomes quarterly maintenance; Flowise's mental model is "agent chains" not "automations with AI". Rejected.
- **Fork n8n:** Fair-code license complicates SaaS distribution; node taxonomy is automation-first and would force awkward mappings for Skills, MCPs, Agents. Rejected (same as ADR-0001).
- **Drawflow / LiteFlow / native SVG:** Smaller ecosystems, fewer accessibility primitives, no zoom/minimap/keyboard support out of the box. Rejected.
- **React Flow:** MIT, mature, accessibility primitives built-in, used in production by many AI tools, the canvas-only library — we keep full control of node rendering and side panels. **Selected.**

**Rationale:** When ADR-0001 was written, "build own" was 3 quarters of work. Two quarters of that — the adapter, the canonical node catalog, the persistence shell, the gateway-catalog palette, the dry-run UX, the version diff — are already done. The remaining gap is the canvas itself, which is precisely what React Flow provides. The cost differential that justified Flowise no longer exists.

### D2: Triggers as a sibling block to `steps`, not a step kind (with bridge for legacy `event-trigger`)

**Decision:** The canonical AST gains a top-level `spec.triggers: TriggerBlock[]` alongside `spec.inputs` and `spec.steps`. A trigger is **not** a step. Triggers fire executions; steps run inside them. The legacy `event-trigger` step kind is preserved as a deprecated shape: on parse, the DSL layer auto-migrates `event-trigger` steps into the new `triggers` block (`EventPattern` maps cleanly onto `webhook-in` or `event-bus` trigger configs) and emits a `deprecated_step_kind` lint warning. Publishes still succeed during the deprecation window; a follow-up change removes the legacy kind from the enum.

```yaml
apiVersion: forge.workflows/v1
kind: Workflow
metadata: { ... }
spec:
  inputs: []         # optional static inputs
  triggers:          # NEW — zero or more
    - id: email-in
      type: email-inbound
      config:
        mailbox: support@acme.com
        filter: { subject_contains: "[urgent]" }
      outputs:        # schema of the event payload available to steps via $triggers.<id>.<field>
        subject: string
        from: string
        body: string
  steps:
    - id: classify
      type: llm
      inputs:
        message: $triggers.email-in.body
      ...
```

**Alternatives considered:**

- **Trigger as a step kind (status-quo `event-trigger`):** Already in the AST. Reuses the step machinery but mixes "this fires the execution" with "this runs during execution". Confuses dependency analysis (a trigger doesn't depend on anything; everything depends on it implicitly), forces every cron/email/manual trigger to also be modeled as a step, and there is no clean place for `outputs` schema declarations distinct from step outputs. Rejected (but bridged for backward compatibility — see migration note above).
- **Triggers as workflow metadata:** Cleaner for invoke-only flows but doesn't expose the event payload to steps in a typed way. Rejected.

**Rationale:** Keeping triggers as a sibling block keeps the step graph free of the "what started this" concept. Steps reference trigger output via `$triggers.<id>.<field>`, identical in spirit to `$inputs.<field>`. Auto-migrating legacy `event-trigger` steps on parse means no existing published workflow breaks at the lint or runtime layer; the deprecation warning surfaces the migration to authors at next save.

### D3: Trigger routing — dedicated `trigger-router` service

**Decision:** A new Go service `services/trigger-router/` owns:
- A webhook receiver under `/v1/hooks/in/{workflow_id}/{trigger_id}` for `webhook-in` triggers.
- A cron scheduler (using existing Temporal cron-workflow support) for `cron` triggers.
- An event-bus subscriber for `event-bus` triggers.
- A pluggable adapter set for `email-inbound` (IMAP/Gmail/Outlook adapters as separate sidecar workers).
- A registry of active triggers per workflow version, sourced from `workflow-registry` on publish.

When a trigger fires, the router POSTs `workflow-runtime`'s `/v1/executions` with `{trigger_event: {...}}` and tenancy context.

**Alternatives considered:**

- **Embed trigger logic in `workflow-runtime`:** Conflates "start an execution" with "subscribe to external sources". Hurts testability and bloats the runtime. Rejected.
- **Per-trigger-type sidecar inside the workflow worker:** Forces the runtime to host webhook receivers and IMAP clients. Same conflation problem. Rejected.

**Rationale:** Triggers are an integration concern; the runtime is an execution concern. Separating them mirrors the mcp-gateway/runtime split and keeps each service deployable independently.

### D4: LLM node binds to existing services, no new primitives

**Decision:** The LLM node references existing services rather than introducing new ones:

```yaml
- id: classify
  type: llm
  prompt_template: registry:prompt/sdlc-product/email-classify@1.3.0   # prompt-template-service
  model:
    ref: gateway:model/claude-opus-4-7@latest-stable                   # model-gateway resolves
    overrides: { temperature: 0.2, max_tokens: 1024 }
  tools:                                                                # auto-bind to in-scope MCPs
    - registry:mcp/email-tools@2.0.0
    - registry:mcp/knowledge-base@1.5.0
  inputs:
    message: $triggers.email-in.body
  outputs:                                                              # declared schema for downstream nodes
    category: enum[urgent, billing, general]
    draft: string
    confidence: number
```

**Alternatives considered:**

- **Embed prompts inline:** No reuse, no versioning, no separation between authors and prompt engineers. Rejected.
- **Free-form `tools` array with arbitrary descriptions:** Bypasses the MCP gateway and its policy/audit controls. Rejected.
- **No declared output schema:** Forces downstream nodes to treat LLM output as opaque, defeating the visual editor's ability to map fields. Rejected.

**Rationale:** Reusing `prompt-template-service`, `model-gateway`, and `mcp-gateway` keeps the LLM node consistent with the platform's identity, audit, and credential story. Declared output schema unlocks the visual editor's field-mapping UX.

### D5: Custom-node SDK — specified now, ingested later

**Decision:** Define a node manifest schema, a TypeScript interface for node renderers, and a packaging convention. Publish it in `docs/sdk/custom-nodes.md` and ship a single end-to-end example node in `portal/src/components/flow/nodes/examples/`. Do **not** build the registry-ingestion pipeline in this change.

**Alternatives considered:**

- **Build the full ingestion pipeline now:** Doubles scope; the user said "leaves the door open, but not the focus now". Rejected.
- **Defer the SDK entirely:** Risks designing a node API that can't be opened up later. Rejected.

**Rationale:** Codifying the contract now is cheap and prevents tech debt; building the pipeline is a future change.

### D6: User-facing brand — "AI Flows" / "Flujos AI"

**Decision:** User-facing copy in the Portal refers to **"AI Flows"** (ES: **"Flujos AI"**). Internal code identifiers, service names, and APIs keep `workflow` to avoid a massive rename across 60+ services. The dictionary entries land in `portal/src/i18n/dictionary.ts`; routes stay `/workflows` and `/workflows/editor` (renaming routes would break inbound links).

**Alternatives considered:**

- **Rename everything to `flow`:** Touches every service, breaks external links, churns docs. Rejected.
- **Use "Workflows" everywhere:** Misses the AI-first positioning. Rejected.
- **Use "Automations":** Closer to n8n/Zapier but loses the AI emphasis. Rejected.

### D7: Surface consolidation — library / editor / code view

**Decision:**

- `/workflows` becomes the **library**: list of AI Flows in the workspace, filters, version history per flow, diff viewer. No editing here.
- `/workflows/editor` becomes the **canvas**: React Flow visual editor with palette (Triggers, AI, Actions, Logic), property panel, dry-run side drawer, and a "Code view" tab that shows the canonical YAML read/write.
- The current YAML editor in `/workflows/page.tsx` is removed; its diff viewer is moved to a `/workflows/[id]/history` sub-route.

**Rationale:** Two surfaces with overlapping responsibilities confuse authors. A library + an editor with a code-view tab is the standard pattern (n8n, Zapier, Make all do this).

### D8: Step type catalog reconciliation — Go enum is the source of truth, TS mirrors it

**Decision:** Extend `pkg/workflow/ast.StepType` with the names the Portal already promises: `llm`, `agent`, `prompt-template`, `webhook` (outbound), `github-action`, `deploy-action`, `approval-action`, `notification-action`, `eval`, `custom`. The existing `prompt` step type stays in the enum but is marked deprecated; on parse, the DSL layer auto-aliases `prompt` to `prompt-template` and emits a `deprecated_step_kind` warning. The Portal TS `CanonicalStepType` is reduced to mirror the Go enum exactly (no more 15-vs-8 mismatch). A unit test in `pkg/workflow/ast` enforces parity: every value in TS must exist in Go, and vice versa.

**Per-type implementation depth in this change:**

| Step type | Enum + parse | Lint | Runtime executor | Editor renderer + property panel |
|---|---|---|---|---|
| `skill`, `mcp`, `branch`, `loop`, `human-in-the-loop` | already done | already done | already done | new in this change |
| `prompt` → `prompt-template` alias | added | added | trivial wrapper around existing `prompt` executor | new |
| `sub-workflow` | already done | already done | already done | new |
| `llm` | **new** | **new** | **new** (see D4) | **new** |
| `agent` | **new** | new (asset-ref + active-surface validation) | reuse existing `a2a-gateway` invocation path | new |
| `webhook` (outbound) | **new** | new (URL + signing validation) | new — simple HTTP POST executor | new |
| `github-action` | **new** | new (action ref + inputs) | new — uses existing `mcp/github` gateway | new |
| `deploy-action` | **new** | new (target + policy) | reuse `deploy-orchestrator` | new |
| `approval-action` | **new** | new | reuse `human-in-the-loop` executor with a typed config | new |
| `notification-action` | **new** | new | new — fans out to email/slack via MCPs | new |
| `eval` | **new** | new (eval suite ref) | reuse `advanced-eval-harness` | new |
| `custom` | **new** | new (manifest validation) | **new** (see custom-node-sdk spec) | new |
| `event-trigger` | preserved | deprecation warning | auto-migrates to `triggers` block | hidden in palette; legacy flows surface a banner |

**Alternatives considered:**

- **Implement every executor fully in this change:** Would balloon scope by ~3 weeks. Several executors (`agent`, `deploy-action`, `eval`) reuse existing services; the new ones (`llm`, `webhook`, `notification-action`, `custom`) each cost real engineering. Rejected — see migration plan for the ordered cuts if scope must shrink.
- **Add step types lazily as the editor needs them:** Lets the TS/Go skew persist longer. Rejected — the whole point of D8 is to make the mismatch impossible to recreate.
- **Use a TS-only "presentation" type that maps to a smaller Go set:** Hides the gap rather than closing it; downstream tooling (registry diff, runtime executor selection) still has to make the same translation. Rejected.

**Rationale:** The Go AST is what executes. Anything the Portal pretends to support but Go cannot execute is a future production incident waiting to happen. The cost of catching up the enum is paid once; the cost of mismatch is paid every time someone drags an unsupported node onto a canvas and discovers it at runtime.

## Risks / Trade-offs

- **[Trigger-router becomes a SPOF for AI Flows]** → Mitigation: stateless service, horizontal scaling, per-tenant rate limits, dead-letter queue for failed event-to-execution translations. Trigger registry is replicated from `workflow-registry`.
- **[`@xyflow/react` major version churn]** → Mitigation: pin exact version in `package.json`, quarterly upgrade tasks via OpenSpec, abstract the canvas wrapper so a future swap stays adapter-isolated.
- **[Custom-node SDK lock-in once published]** → Mitigation: ship it as `v0.x` with an explicit "API may change" banner; freeze to `1.0` only after the ingestion pipeline lands and we have ≥3 example nodes built externally.
- **[LLM node + tool-calling has hidden cost/latency]** → Mitigation: tool-call budget per execution (`max_tool_calls: 10` default); cost estimates surfaced in the dry-run panel; tool calls audited and observable via per-asset observability.
- **[Email-inbound trigger has many adapter variants]** → Mitigation: ship only IMAP in v1; Gmail and Outlook adapters are follow-up changes. Document the adapter contract.
- **[Trigger-router subscribes to event-bus events that don't exist yet]** → Mitigation: the `event-bus` trigger type ships with an explicit "event topics must be declared in the workflow's `selected_events`" requirement; lint refuses publish if subscribed topics are unknown.
- **[Existing `/workflows` users discover surface moved]** → Mitigation: `/workflows` still exists but renders the library; a one-time banner on first visit explains the consolidation; the YAML "Code view" remains available inside the editor.
- **[Phase 5 sign-off correction may be politically sensitive]** → Mitigation: handle as a sign-off correction note with date and reason, not a silent edit. Captures the lesson that "deferred items list" should have been a hard gate.
- **[Ambiguity in "trigger fires while a previous execution is still running"]** → Mitigation: per-workflow concurrency policy in the trigger config (`concurrency: queue | drop | overlap`); default `queue`.
- **[Catalog reconciliation balloons scope]** → Mitigation: in this change, every new step type gets enum + parse + lint + editor renderer. Runtime execution lands per-type per the table in D8. If scope must shrink at cutover, the priority order is: `llm` → `webhook` → `notification-action` → `agent` (rest of the new types ship without executor, with a "not yet implemented" runtime error and a registry lint that refuses publishes of those types until their executor lands).
- **[Auto-migration of legacy `event-trigger` introduces silent data drift]** → Mitigation: the auto-migration runs on parse, not on persistence — the stored AST is unchanged until the next save. On save the registry persists the migrated form and records the bump with reason `migrate_event_trigger_to_triggers_block`. A read-only diff viewer shows authors what changed.

## Migration Plan

1. **Phase A — Foundation (no user-visible change yet):**
   - Add `@xyflow/react` dependency, scaffold canvas component under a feature flag `AI_FLOWS_CANVAS_ENABLED`.
   - Rename `flowise-adapter` → `ast-canvas-adapter`. Extend tests.
   - **Reconcile the step catalog (D8):** extend `pkg/workflow/ast.StepType` with the 10 missing types, alias `prompt` → `prompt-template`, add the parity unit test against the TS catalog.
   - Extend `pkg/workflow/ast` with optional `triggers` block. Existing workflows are unaffected (zero-trigger). Add the parse-time auto-migration that converts legacy `event-trigger` steps into the new `triggers` block.
   - Extend `pkg/workflow/dsl` lint with the new rules, plus `deprecated_step_kind` for `prompt` and `event-trigger`.
   - Extend `workflow-registry` version-classification with trigger and LLM-node deltas, plus a one-time data migration that strips `node.active_surface.endpoint` from existing persisted versions.

2. **Phase B — Runtime + trigger-router:**
   - `workflow-runtime`: accept `trigger_event` in `POST /v1/executions`.
   - Build `services/trigger-router`: webhook-in + manual + cron first; event-bus and email-inbound next.

3. **Phase C — Canvas v1 (behind flag):**
   - Build palette sections (Triggers, AI, Actions, Logic).
   - Build node renderers for: trigger, llm, mcp, agent, skill, branch, loop, human-in-the-loop, webhook-out, notification.
   - Build the property panel and the dry-run drawer.
   - Build the "Code view" tab.

4. **Phase D — LLM node end-to-end:**
   - Wire prompt-template-service lookup, model-gateway selection, MCP tool-calling.
   - Add LLM-specific config panel in the canvas.

5. **Phase E — Reference flow + e2e:**
   - Publish `forge.reference.ai-email-triage@1`.
   - Playwright e2e drags it together, dry-runs, asserts trigger payload reaches the LLM node and the LLM output reaches the webhook MCP.

6. **Phase F — Cutover:**
   - Flip `AI_FLOWS_CANVAS_ENABLED` to default-on.
   - Move `/workflows` to library mode; ship the one-time banner.
   - Move the diff viewer from `/workflows` to `/workflows/[id]/history`.
   - Update ADR-0001 (status: Superseded), publish ADR-0002.
   - Correct the Phase 5 sign-off.
   - Update license inventory.

**Rollback strategy:** Each phase is independently revertible. If Phase F (cutover) fails in production, flip `AI_FLOWS_CANVAS_ENABLED=false` and the editor falls back to the JSON textarea shell. The AST extensions (triggers block) are backward-compatible — they can stay in the schema even if the canvas is rolled back.

## Open Questions

- **Q1:** Should the email-inbound adapter ship with native Gmail OAuth in v1, or only generic IMAP? Generic IMAP is simpler; Gmail OAuth is what users will actually want.
- **Q2:** Where does the `trigger-router` live in the deployment topology — its own pod per tenant, or shared with per-tenant rate limiting? Affects HA and cost.
- **Q3:** Does the canvas need a "manual test fire" button at design time (firing a synthetic trigger event in the editor)? Strongly improves DX but adds scope.
- **Q4:** Should the LLM node's `tools` array auto-populate from the workflow's pinned `selected_assets.mcps`, or stay an explicit per-node selection? Auto-populate is more magical; explicit is more debuggable.
- **Q5:** Naming for the editor route — keep `/workflows/editor` or switch to `/flows`? Keeping avoids broken inbound links; switching aligns with the brand.
- **Q6:** Do we ship the custom-node SDK example node as a built-in (visible in the palette under "Examples") or as docs-only? Visible-in-palette helps discoverability but pollutes the production palette.
