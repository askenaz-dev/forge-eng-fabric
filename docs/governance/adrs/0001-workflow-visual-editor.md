# ADR-0001: Workflow Visual Editor — Embed Flowise

**Status:** Superseded by [ADR-0002](./0002-canvas-react-flow.md) (2026-05-16)
**Date:** 2026-05-09
**Author:** Platform Architecture
**Reviewers:** SDLC Leads, Security, Frontend Engineering
**Review date:** 2026-Q4 (or sooner if Flowise upstream releases a major version with breaking changes)

> **Supersession note (2026-05-16):** The Flowise embed was never installed
> (`flowise` and `reactflow` absent from `portal/package.json`) and the
> Phase 5 sign-off acknowledged this gap in its deferred items list while
> marking the visual-editor exit criterion as done. The
> [ai-flow-authoring](../../../openspec/changes/ai-flow-authoring/) change
> records the post-mortem and adopts React Flow (`@xyflow/react`, MIT) as
> the canvas library. The adapter remains as `ast-canvas-adapter`; the
> 1-quarter cost differential that justified Flowise no longer exists
> because the adapter, the persistence shell, the gateway-catalog
> palette, the dry-run UX, and the version diff have all landed. See
> [ADR-0002](./0002-canvas-react-flow.md) for the new decision and
> rationale.

## Context

The `workflow-visual-editor` capability promises a non-technical authoring surface for workflows that compose Skills, MCPs, LLMs, HITL gates, and deploy actions. Three implementation paths were on the table when this decision was made:

1. **Embed Flowise** — open-source LLM-native flow builder under LGPL.
2. **Fork n8n** — open-source automation platform under fair-code license.
3. **Build own** — React Flow + custom node SDK on top of `workflow-registry`.

The platform already owns the canonical workflow AST and DSL (`pkg/workflow/ast`, `pkg/workflow/dsl`) plus a typed registry (`services/workflow-registry`). The editor only needs to be a thin presentation layer that translates between its native node format and our AST.

## Decision

**Embed Flowise.** Persist canonical AST in `workflow-registry` via a thin adapter that translates between Flowise's native node format and our AST. Contribute back the node catalog adapter to the Flowise community.

## Rationale

| Criterion | Flowise (chosen) | n8n fork | Build own |
|---|---|---|---|
| LLM-native primitives (LLM, Agent, Prompt Template, MCP) | Native | Add via custom nodes | Build from scratch |
| License compatibility with our distribution | LGPL — compatible (link, contribute mods) | Fair-code, restrictive for SaaS distribution | N/A |
| Engineering cost | ~1 quarter (adapter + embed) | ~2 quarters (fork mgmt + adapter) | 3+ quarters |
| Long-term maintenance | Track upstream + adapter | Track fork against upstream | Own everything |
| Time to first dogfood | Days | Weeks | Months |
| Match to Forge node catalog | High (LLM-first) | Medium (automation-first) | Custom-fit |

Flowise's node taxonomy maps cleanly onto our canonical catalog (LLM, MCP, Skill, Agent, Prompt Template, HITL Gate, Branch, Loop, Retry, Eval, Webhook, GitHub Action, Deploy Action, Approval Action, Notification Action). n8n's automation-first taxonomy would force awkward mappings for Skills and MCPs. Build-own's engineering cost does not pay for itself when an open-source LLM-native option exists.

## Consequences

**Positive**:
- Faster time-to-value: ~1 quarter to first dogfood vs 3+ quarters for build-own.
- We get upstream improvements (community node contributions, accessibility, i18n).
- The adapter contract is small (Flowise JSON ↔ canonical AST), so swapping the host later is cheap.

**Negative**:
- LGPL forces us to publish modifications to Flowise itself (not to our adapter or host code).
- Upstream version pinning becomes a quarterly maintenance task.
- Some Flowise UI elements (settings panel, marketplace) overlap with our own and must be hidden or rebranded.

**Mitigations**:
- Keep adaptations isolated in `portal/src/lib/flowise-adapter/`. Track license inventory in [`docs/governance/licenses.md`](../licenses.md).
- Pin Flowise to a known-good release in `portal/package.json`; quarterly upgrade tasks land in OpenSpec.
- Hide non-Forge Flowise UI via host-controlled CSS and prop-driven feature flags.

## Alternatives considered (and why rejected)

### n8n fork

- Pros: mature workflow runner; large community.
- Cons: Fair-code license complicates redistribution. Node taxonomy is automation-first, not LLM-native — Skills and MCPs don't map cleanly. Forking creates upstream-incompatibility debt.
- Verdict: rejected.

### Build own (React Flow + custom node SDK)

- Pros: perfect fit for our AST; full control over UX.
- Cons: 3+ engineer-quarters of effort. Accessibility, i18n, undo/redo, and other table-stakes features must be reimplemented.
- Verdict: rejected — does not pay for itself when an open-source LLM-native option exists.

### YAML-only authoring (no visual editor)

- Pros: zero engineering cost.
- Cons: Fails the non-technical user promise.
- Verdict: rejected.

## License tracking

Flowise is licensed under **LGPL-2.1-or-later**. Inventory entry in [`docs/governance/licenses.md`](../licenses.md). Modifications to Flowise itself (not our adapter or host) MUST be contributed upstream or made available on request, per LGPL requirements.

## Upgrade cadence

| Cadence | Trigger | Owner |
|---|---|---|
| Quarterly | Routine upstream sync | Frontend Eng |
| On-demand | CVE or security patch | Security + Frontend Eng |
| On-major | Flowise major-version release | Platform Architecture review before adoption |

## Review

This ADR is reviewed at **2026-Q4** or sooner if (a) Flowise releases a major version with breaking changes to the node format, or (b) we observe a >30% drift between Flowise's taxonomy and our canonical AST that requires adapter rewrites.

## References

- [`docs/platform-enablement.md` — Phase 5 enablement](../../platform-enablement.md#phase-5-workflow-marketplace)
- [`workflow-visual-editor` capability spec](../../../openspec/specs/workflow-visual-editor/spec.md)
- [`pkg/workflow/ast`](../../../pkg/workflow/ast/) — canonical AST
- [`services/workflow-registry`](../../../services/workflow-registry/) — versioned persistence
- [Flowise upstream](https://github.com/FlowiseAI/Flowise)
