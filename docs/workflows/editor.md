# AI-Flow visual editor

The Portal "AI Flows" module is the visual surface over the canonical
[workflow AST](dsl.md). It lives at `/workflows/editor` and consumes the
workflow registry (`WORKFLOW_REGISTRY_URL`) and runtime
(`WORKFLOW_RUNTIME_URL`) at runtime. The library + version-history surface
lives at `/workflows`.

> **Heritage:** ADR-0001 picked Flowise but the embed never landed. The
> editor is now built on `@xyflow/react` (MIT) per
> [ADR-0002](../governance/adrs/0002-canvas-react-flow.md). See the
> [ai-flow-authoring](../../openspec/changes/ai-flow-authoring/) change
> for the full reconciliation.

## What the editor gives you

- **Canvas (default tab)** — `@xyflow/react`-powered drag-and-drop with
  pan, zoom, minimap, keyboard navigation, edge routing.
- **Palette** with four sections — Triggers / AI / Actions / Logic (+
  Custom when custom nodes are registered). Drag onto the canvas or press
  Enter on a focused palette item to add.
- **Trigger band** above the canvas — shows the firing source. A flow
  with zero triggers renders `Triggered by: Manual invoke`.
- **Property panel** opens on node selection. For LLM nodes it surfaces
  prompt-template picker, model picker (workspace whitelist applied),
  per-override fields, tools multi-select, output schema editor,
  `max_tool_calls`, and an estimated cost-per-execution preview.
- **Code view tab** — renders the current AST as canonical JSON for
  direct editing. Invalid JSON disables save.
- **Dry-run drawer** — runs the current canvas state through
  workflow-runtime in `dry_run=true` mode and shows per-step inputs +
  outputs. No real Skill / MCP / LLM call is made.
- **Library at /workflows** — list of flows, version history per flow,
  diff viewer. Edits happen exclusively in the canvas.

## Authoring a flow

1. Navigate to `/workflows/editor?workspace_id=<ws>`. The page loads
   with the palette on the left and an empty canvas.
2. Drag a Trigger from the palette into the trigger band. Configure
   `mailbox_ref`, `expression`, etc. in the property panel.
3. Drag step nodes from AI / Actions / Logic. Connect by dragging from
   the source handle to the target handle — the edge becomes a
   `depends_on` entry on save.
4. Select an LLM node and pick a prompt template + model + tools in the
   right-rail property panel.
5. Click **Dry run**. The drawer shows the trace.
6. Click **Save**. The registry creates a new immutable version
   classified by `services/workflow-registry/internal/registry/diff.go`.

## Publish flow

1. Edit on canvas (and/or in code view); save promotes the current
   state to a new version.
2. The registry runs schema + lint, then classifies the diff:
   - Removed/required input → MAJOR
   - Removed step or step type changed → MAJOR
   - Trigger added → MINOR; removed → MAJOR; type changed → MAJOR
   - LLM `outputs_schema` field removed → MAJOR; added → MINOR
   - Added optional input/output, new step → MINOR
   - `prompt → prompt-template` alias → PATCH (`migrate_prompt_to_prompt_template`)
   - `event-trigger` step → triggers block → PATCH (`migrate_event_trigger_to_triggers_block`)
   - Description/owners only → PATCH
3. Either provide a version that satisfies the classification, or check
   `Auto-bump version`.

## Feature flag

The canvas is gated on `AI_FLOWS_CANVAS_ENABLED=true`. When the flag is
off, `/workflows/editor` renders a fallback notice and the existing
YAML editor at `/workflows` remains the authoring surface for that
tenant.

## E2E tests

The Playwright suite at `portal/tests/e2e/ai-email-triage.spec.ts`
drag-builds the AI Email Triage reference flow and runs a dry-run.
Gated on `AI_FLOWS_CANVAS_ENABLED=true` so it skips during the rollout
window.
