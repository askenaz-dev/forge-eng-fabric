# ADR-0002: AI-Flow Visual Editor — React Flow + Custom Canvas

**Status:** Accepted
**Date:** 2026-05-16
**Author:** Platform Architecture
**Reviewers:** Frontend Engineering, Security, Product
**Supersedes:** [ADR-0001](./0001-workflow-visual-editor.md)
**Review date:** 2026-Q4 (or sooner on a `@xyflow/react` major release with breaking changes)

## Context

When [ADR-0001](./0001-workflow-visual-editor.md) was authored (2026-05-09)
the decision matrix weighed:

1. Embed Flowise — 1 quarter of work.
2. Fork n8n — 2 quarters, fair-code license complications.
3. Build own (React Flow + custom node SDK) — 3+ quarters.

Flowise won on time-to-value alone. The decision was accepted but the
embed never landed — `@xyflow/react`, `flowise`, and `reactflow` are
absent from `portal/package.json`, and `EditorClient.tsx` carried a
"Flowise embed loaded dynamically once the npm dependency lands" comment
in production. Phase 5 sign-off marked the visual-editor exit criterion
done in the same change that listed "Live Flowise embed browser session"
as deferred — a contradiction now acknowledged in this supersession.

Meanwhile, two thirds of the build-own work has actually shipped under
other change names:

- The Flowise ↔ canonical AST adapter at `portal/src/lib/flowise-adapter/`
  (now renamed to `ast-canvas-adapter/`).
- The persistence shell against `workflow-registry`.
- The gateway-catalog palette wired to mcp-gateway / a2a-gateway.
- The dry-run UX against `workflow-runtime`.
- The version diff + bump classifier.

What remains is the canvas itself, which `@xyflow/react` provides
out-of-the-box. The cost differential that justified Flowise has
collapsed.

A separate factor: the AI-Flow product is positioned for **non-technical
users**, not engineers. Embedding Flowise means the AI-Flow editor will
never look like the rest of the Portal (`portal/src/i18n/dictionary.ts`,
`portal/src/components/shell/`). Brand control and Spanish-first copy
are easier on a canvas we own.

## Decision

Adopt **`@xyflow/react`** (MIT) as the canvas library. Implement custom
node renderers under `portal/src/components/flow/nodes/`. Persist
canonical AST via the renamed `ast-canvas-adapter`. Round-trip parity is
enforced by `portal/src/lib/ast-canvas-adapter/index.test.ts`.

## Rationale

| Criterion | React Flow (chosen) | Flowise (previous choice) | Fork n8n | Drawflow / native SVG |
|---|---|---|---|---|
| LLM-first node catalog | Custom — we define it | Yes | Automation-first | Custom |
| License compatibility | MIT — frictionless | LGPL — must publish fork mods | Fair-code — restrictive | MIT / various |
| Brand alignment | Total | None (Flowise UI) | None | Total |
| Accessibility primitives | Built-in | Built-in | Built-in | Roll-your-own |
| Engineering cost (now) | Remaining: canvas + node renderers | Embed + adapter mods | Fork mgmt + adapter | Canvas + everything |
| Upstream maintenance | Quarterly version pin | Quarterly + fork mods | Track fork against upstream | None |
| Match to Forge node catalog (16 types + 5 triggers) | Native | Approximate mapping | Awkward mappings | Native |

## Consequences

**Positive:**
- Forge brand applies to the editor: Tailwind + design tokens + Spanish-first.
- No LGPL distribution friction.
- The 16-type canonical catalog (extended in §D8 of the change design) is reified in code: `pkg/workflow/ast/catalog.json` + `portal/src/lib/ast-canvas-adapter/`.
- Trigger primitives, the new LLM node shape, and custom-node SDK all design without Flowise's mental model gravity.

**Negative:**
- We own more code — node renderers, palette, property panels, dry-run drawer, code-view tab.
- Quarterly `@xyflow/react` upgrade discipline required (low overhead — the library is API-stable).

**Mitigations:**
- Single `FlowNode` generic renderer in v1 keeps the code surface small; per-type renderers register through `nodeTypes` only when specific UX requires.
- Version pinned in `portal/package.json` at `12.3.5`; quarterly upgrade lands via OpenSpec.

## Alternatives reconsidered

### Embed Flowise (ADR-0001 status quo)

- Pros: less code to maintain.
- Cons: LGPL fork-mod publication; UX never matches Forge brand; mental model is "agent chains" not "automations with AI"; the cost differential evaporated when the supporting infrastructure landed.
- Verdict: superseded.

### Fork n8n

- Pros: mature workflow runner.
- Cons: Fair-code license complicates SaaS distribution; node taxonomy is automation-first; same fork-upstream debt as before.
- Verdict: rejected (same as ADR-0001).

### Drawflow / LiteFlow / native SVG

- Pros: minimal dependencies.
- Cons: roll-your-own accessibility, zoom, minimap, keyboard nav, edge routing.
- Verdict: rejected.

## License tracking

`@xyflow/react` is **MIT**. Entry tracked in
[`docs/governance/licenses.md`](../licenses.md). No publication
obligations.

## Upgrade cadence

| Cadence | Trigger | Owner |
|---|---|---|
| Quarterly | Routine `@xyflow/react` upstream sync | Frontend Engineering |
| On-demand | CVE or security patch | Security + Frontend Engineering |
| On-major | `@xyflow/react` major-version release | Platform Architecture review before adoption |

## Review

This ADR is reviewed at **2026-Q4** or sooner if `@xyflow/react`
releases a major version with breaking changes that affect the canvas
contract.

## References

- [ai-flow-authoring OpenSpec change](../../../openspec/changes/ai-flow-authoring/)
- [`pkg/workflow/ast/`](../../../pkg/workflow/ast/) — canonical AST
- [`portal/src/lib/ast-canvas-adapter/`](../../../portal/src/lib/ast-canvas-adapter/) — adapter
- [`portal/src/components/flow/`](../../../portal/src/components/flow/) — canvas
- [`docs/sdk/custom-nodes.md`](../../sdk/custom-nodes.md) — custom node SDK
- [React Flow upstream](https://reactflow.dev)
