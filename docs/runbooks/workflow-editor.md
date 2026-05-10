# Workflow Visual Editor Runbook

> Decision: [ADR-0001 — Embed Flowise](../governance/adrs/0001-workflow-visual-editor.md)
> Last validated: 2026-05-09 (adapter and persistence shell; live Flowise embed pending)

The visual workflow editor lives at `/workflows/editor` in the Portal. It reads and writes the canonical workflow AST in `workflow-registry` via the [Flowise adapter](../../portal/src/lib/flowise-adapter/index.ts).

## Permissions

The editor refuses access to users who lack `workflow.author` on the active Workspace. To grant the permission:

```sh
# OpenFGA tuple
fga tuple write user:<sub> workflow.author workspace:<ws-id>
```

Or use the Portal's [Admin & Governance](../../portal/src/app/permissions/page.tsx) page to add the relation.

## Authoring a workflow

1. Navigate to `/workflows/editor?workspace_id=<ws>`. The page loads with the canonical node catalog in the left sidebar.
2. Drag nodes from the catalog onto the canvas. Each node binds to a Registry asset (Skill, MCP, Prompt Template) — assets in non-`approved` lifecycle state are visually marked and rejected on save.
3. Connect nodes by dragging from the source handle to the target handle. Edges become `depends_on` entries on the canonical AST.
4. Click **Save as new version**. The editor:
   - Round-trips the graph through `astToFlowise` → `flowiseToAST` to validate adapter parity.
   - POSTs the canonical YAML to `workflow-registry`.
   - The registry creates a new immutable version with monotonically increasing `version`.

## Opening prior versions

Use the `?version=<v>` query parameter:

```
/workflows/editor?workspace_id=<ws>&workflow_id=<id>&version=1.2.0
```

When the loaded version is not the latest, the editor renders **read-only**. The only mutating action is **Fork as new latest**, which copies the AST into a draft, increments the version, and reopens the editor in normal mode.

## Round-trip parity

The editor MUST preserve canonical AST semantics across `Flowise format ↔ canonical AST`. The parity test lives at [`portal/src/lib/flowise-adapter/index.test.ts`](../../portal/src/lib/flowise-adapter/index.test.ts) and is part of `pnpm test`.

Run the editor smoke after changes to the adapter, Portal workflow pages, `workflow-registry`, or `workflow-runtime`:

```sh
python scripts/integration/smoke_workflow_editor.py
```

The smoke builds a workflow DSL payload, saves a new immutable workflow version, exports the DSL, re-opens the saved version, and dry-runs it through `workflow-runtime`.

If you observe a save that mangles the workflow:

1. Run `pnpm test --filter flowise-adapter` and confirm the round-trip test still passes.
2. If it fails, the adapter is the regression source. Compare the failing fixture to recent changes in `index.ts`.
3. If it passes, the regression is in Flowise's native node format — file an upstream issue and pin to the previous Flowise version while the fix lands.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Page shows "Permission required" | User lacks `workflow.author` | Grant via OpenFGA or `/permissions` page |
| Save returns 4xx with "asset not approved" | One node references a non-`approved` Registry asset | Promote the asset (eval-gated) or replace with an approved version |
| Save returns 4xx with "breaking change requires major bump" | Workflow AST diff is breaking against the latest version | Bump the `metadata.version` major field |
| Editor renders read-only unexpectedly | Loaded version is not `latest_version` on the parent record | Reload without `?version=` to open the latest |

## Asset-state filtering

The catalog displays Registry assets bound to each node type, filtered by lifecycle state. Only `approved` assets can be saved into a workflow:

| State | Visible in catalog | Saveable in workflow |
|---|---|---|
| `draft` | No | No |
| `in_review` | Yes (marked) | No |
| `approved` | Yes | Yes |
| `published` | Yes | Yes |
| `deprecated` | Yes (marked) | Yes (existing only; new uses rejected) |

## License and upgrade cadence

Flowise is LGPL-2.1-or-later (see [licenses.md](../governance/licenses.md)). Modifications to Flowise itself are tracked in [`portal/flowise-mods/`](../../portal/flowise-mods/) and contributed upstream. The version is pinned in `portal/package.json`.

Quarterly upgrade tasks land via OpenSpec changes; emergency CVE patches are co-approved by Security and Frontend Engineering and bypass the quarterly cadence.

## Related

- [ADR-0001](../governance/adrs/0001-workflow-visual-editor.md)
- [Flowise adapter](../../portal/src/lib/flowise-adapter/index.ts)
- [`workflow-visual-editor` capability spec](../../openspec/specs/workflow-visual-editor/spec.md)
