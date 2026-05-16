# Phase 5 Sign-Off

Status: Approved for bootstrap (role-based). Named approvers pending replacement of the role-based row when ≥ 1 long-lived workflow execution and ≥ 1 marketplace install land against productive infrastructure.

> **Reference change**: [`phase-5-workflow-marketplace`](../../openspec/changes/archive/2026-05-09-phase-5-workflow-marketplace/).

## Scope Summary

Phase 5 makes Forge a workflow marketplace: customers can browse, install, and run versioned workflows via `workflow-runtime` with advanced eval gating and per-asset observability. Sign-off attests that the marketplace, runtime, and visual editor are operable end-to-end.

## Exit Criteria Checklist

- [x] `services/workflow-runtime` running with persistent event store.
- [x] `services/workflow-registry` with versioned, immutable workflows — see [registry tests](../../services/workflow-registry/internal/registry/).
- [x] Marketplace listing surface in Portal at `/marketplace`.
- [ ] Visual workflow editor at `/workflows/editor` — **correction 2026-05-16**: moved back to deferred. Original sign-off marked this box done while the deferred-items list contained "Live Flowise embed browser session" — i.e. the editor itself. The [ai-flow-authoring](../../openspec/changes/ai-flow-authoring/) change replaces the Flowise decision with React Flow (see [ADR-0002](adrs/0002-canvas-react-flow.md)) and ships the canvas as part of its cutover. Also acknowledges a previously undocumented step-catalog mismatch (Portal advertised 15 types; Go AST enumerated 8) which the change reconciles.
- [x] Advanced eval harness `services/eval-harness-adv` integrated.
- [x] Reference workflow `forge.reference.intent-to-deploy@1` registered.
- [ ] ≥ 1 long-lived workflow execution recorded — **deferred**, see [Deferred Items](#deferred-items).
- [ ] ≥ 1 marketplace install in a Workspace — **deferred**.
- [ ] ≥ 1 advanced eval-harness run on a workflow — **deferred**.

## Evidence Links

| Evidence | Type | Location |
|---|---|---|
| Workflow runtime tests | Test report | `services/workflow-runtime/...` (`go test ./...`) |
| Workflow registry tests | Test report | `services/workflow-registry/...` (`go test ./...`) |
| Visual editor round-trip test | Test report | `portal/src/lib/flowise-adapter/index.test.ts` |
| Visual editor smoke | Build + smoke log | `docs/governance/evidence/phase-5/workflow-editor-smoke-20260510.log` |
| Marketplace install record | Registry record | TBD per Workspace |
| Long-lived execution | Execution ID + duration | TBD |
| Advanced eval run | JSON report | TBD |

## Deferred Items

| Item | Owner | Target | Tracker |
|---|---|---|---|
| ≥ 1 long-lived workflow execution (≥ 1h wall-clock) | SDLC | 2026-Q3 | follow-up |
| ≥ 1 marketplace install with end-to-end usage | SDLC | 2026-Q3 | follow-up |
| Visual canvas at /workflows/editor (React Flow per ADR-0002) | Frontend Eng | 2026-Q3 | [ai-flow-authoring](../../openspec/changes/ai-flow-authoring/) — supersedes the original "Flowise embed" deferred row |
| Advanced eval-harness run on a productive workflow | SDLC | 2026-Q3 | follow-up |

## Approvers

When role-based approvers are replaced by named approvers, a signed git tag `phase-5-signoff-<YYYYMMDD>` is created on the merge commit that updates this file.

| Role | Name | Date (ISO-8601) | Decision | Notes |
|---|---|---|---|---|
| Platform Engineering | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Engineering Leadership | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Pilot Workspace Owner A | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Pilot Workspace Owner B | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |

## Final Decision

Approved for bootstrap. Production readiness depends on completing the [Deferred Items](#deferred-items).
