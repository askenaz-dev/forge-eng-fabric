# AI-Flow Visual Editor Runbook

> Decision: [ADR-0002 — React Flow + Custom Canvas](../governance/adrs/0002-canvas-react-flow.md)
> (supersedes [ADR-0001 — Embed Flowise](../governance/adrs/0001-workflow-visual-editor.md))
> Last validated: 2026-05-16 (canvas + LLM property panel + trigger-router dispatch)

The AI-Flow visual editor lives at `/workflows/editor` in the Portal.
Library + version-history at `/workflows`. It reads and writes the
canonical workflow AST via the [`ast-canvas-adapter`](../../portal/src/lib/ast-canvas-adapter/index.ts).

## Permissions

The editor refuses access to users who lack `workflow.author` on the
active Workspace. To grant the permission:

```sh
fga tuple write user:<sub> workflow.author workspace:<ws-id>
```

Or use the Portal's [Admin & Governance](../../portal/src/app/permissions/page.tsx)
page to add the relation.

## Authoring a flow

1. Navigate to `/workflows/editor?workspace_id=<ws>`. The page loads
   with the palette in the left sidebar (Triggers / AI / Actions /
   Logic / Custom).
2. Drag a Trigger into the trigger band. Configure
   `mailbox_ref`, `expression`, `topic`, etc. in the property panel.
3. Drag nodes from the AI / Actions / Logic palette sections onto the
   canvas. Each node binds to a Registry asset (Skill, MCP, Prompt
   Template, Agent) — assets in non-`approved` lifecycle state are
   visually marked and rejected on save.
4. Connect nodes by dragging from the source handle to the target
   handle. Edges become `depends_on` entries on the canonical AST.
5. Select an LLM node and pick prompt template / model / tools / output
   schema in the right-rail property panel. The cost preview updates
   as you change values.
6. Click **Dry run**. The drawer shows the trace from
   `workflow-runtime` with mock I/O per step.
7. Click **Save**. The editor:
   - Round-trips the canvas graph through `astToCanvas` → `canvasToAST`
     to validate adapter parity.
   - POSTs the canonical AST to `workflow-registry`.
   - The registry creates a new immutable version with a SemVer bump
     classified by `internal/registry/diff.go`.

## Opening prior versions

Use the `?version=<v>` query parameter:

```
/workflows/editor?workspace_id=<ws>&workflow_id=<id>&version=1.2.0
```

When the loaded version is not the latest, the editor renders
**read-only**. The only mutating action is **Fork as new latest**.

## Round-trip parity

The editor MUST preserve canonical AST semantics across `canvas ↔
canonical AST`. The parity test lives at
[`portal/src/lib/ast-canvas-adapter/index.test.ts`](../../portal/src/lib/ast-canvas-adapter/index.test.ts).

The Go-side step-type and trigger-type parity tests at
[`pkg/workflow/ast/parity_test.go`](../../pkg/workflow/ast/parity_test.go)
catch any drift between the canonical Go enum and the TS adapter.

Run the editor smoke after changes to the adapter, Portal flow pages,
`workflow-registry`, `workflow-runtime`, or `trigger-router`:

```sh
make demo-ai-email-triage
```

The smoke publishes the reference flow, fires a synthetic email
trigger via trigger-router, and prints the dry-run trace from
workflow-runtime.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Page shows "Permission required" | User lacks `workflow.author` | Grant via OpenFGA or `/permissions` page |
| Editor shows "AI Flows canvas is not enabled" notice | `AI_FLOWS_CANVAS_ENABLED` flag off | Set the env var to `true` in the portal pod / `.env.local` |
| Save returns 4xx with `lint_failed: unknown_trigger_type` | Trigger type not in canonical catalog | Use one of `manual`, `cron`, `webhook-in`, `event-bus`, `email-inbound` |
| Save returns 4xx with `lint_failed: unknown_event_topic` | Event-bus subscribed to unregistered topic | Register topic in platform event catalog or use a known one |
| Save returns 4xx with `lint_failed: dangling_trigger_field` | Step expression refers to a trigger output not declared | Add the field to `trigger.outputs` schema |
| Save returns 4xx with `lint_failed: floating_reference_not_allowed` on `prompt_template` | LLM step uses `@latest`/`@main`/etc | Pin to exact SemVer |
| Save returns 4xx with `lint_failed: missing_prompt_template` | LLM step has no `prompt_template` set | Pick a template in the property panel |
| Save returns 4xx with `lint_failed: model_not_whitelisted` | LLM model not in workspace's `allowed_models` | Add the model to the workspace whitelist or pick a different one |
| Step runs with `step_type_not_yet_implemented` | One of the catalog-reconciled step types is registered with a stub executor | See design.md §D8 priority table — production executor lands in follow-up |
| Editor renders read-only unexpectedly | Loaded version is not `latest_version` on the parent record | Reload without `?version=` to open the latest |

## Asset-state filtering

The catalog displays Registry assets bound to each node type, filtered
by lifecycle state. Only `approved` assets can be saved into a flow.

| State | Visible in catalog | Saveable in flow |
|---|---|---|
| `draft` | No | No |
| `in_review` | Yes (marked) | No |
| `approved` | Yes | Yes |
| `published` | Yes | Yes |
| `deprecated` | Yes (marked) | Yes (existing only; new uses rejected) |

## License and upgrade cadence

`@xyflow/react` is MIT (see [licenses.md](../governance/licenses.md)).
Pinned in `portal/package.json` at `12.3.5`. Quarterly upgrade tasks
land via OpenSpec; emergency CVE patches are co-approved by Security
and Frontend Engineering and bypass the quarterly cadence.

## Related

- [ADR-0002](../governance/adrs/0002-canvas-react-flow.md)
- [`portal/src/lib/ast-canvas-adapter/`](../../portal/src/lib/ast-canvas-adapter/index.ts)
- [`workflow-visual-editor` capability spec](../../openspec/specs/workflow-visual-editor/spec.md)
- [`automation-triggers` capability spec](../../openspec/specs/automation-triggers/spec.md)
- [`llm-flow-node` capability spec](../../openspec/specs/llm-flow-node/spec.md)
- [`custom-node-sdk` capability spec](../../openspec/specs/custom-node-sdk/spec.md)
- [`docs/sdk/custom-nodes.md`](../sdk/custom-nodes.md)
