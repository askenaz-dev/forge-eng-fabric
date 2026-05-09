# Phase 6 Sign-Off

Status: Approved for bootstrap (role-based). Named approvers pending replacement of the role-based row when ≥ 1 simulated remediation under guardrails and ≥ 1 evolution-loop OpenSpec proposal land against productive incidents.

> **Reference change**: [`phase-6-autonomous-ops`](../../openspec/changes/archive/2026-05-09-phase-6-autonomous-ops/).

## Scope Summary

Phase 6 makes Forge run autonomous-ops loops: healing actions execute simulated remediations under SRE/Security guardrails, and the evolution loop turns observed incidents into candidate OpenSpec changes for human review. Sign-off attests that the healing catalog, the simulated-remediation pipeline, and the evolution loop are operable end-to-end.

## Exit Criteria Checklist

- [x] Healing actions catalog defined in `services/healing-engine`.
- [x] Reversibility classification + pre/post-condition probes per action.
- [x] Simulation sandbox with NetworkPolicy/OpenFGA scopes matching the target Workspace.
- [x] Evolution loop emits `openspec.autonomous_loop.proposed.v1` and lands proposals in the Evolution Inbox — see [`portal/src/app/evolution`](../../portal/src/app/evolution/).
- [x] OpenSpec service supports `source: autonomous-loop` with reviewer-required state — see [`openspec_service/models.py`](../../services/openspec/openspec_service/models.py).
- [ ] ≥ 1 simulated remediation against a productive incident with guardrails verified — **deferred**, see [Deferred Items](#deferred-items).
- [ ] ≥ 1 evolution-loop proposal that updates an OpenSpec — **deferred**.

## Evidence Links

| Evidence | Type | Location |
|---|---|---|
| Healing engine tests | Test report | `services/healing-engine/...` |
| Evolution loop tests | Test report | `services/evolution/...` |
| Simulated remediation | Simulation record | TBD |
| Evolution-loop OpenSpec proposal | OpenSpec record | TBD |
| Synthetic incident harness | Script | `scripts/phase6_synthetic_incidents.py` |

## Deferred Items

| Item | Owner | Target | Tracker |
|---|---|---|---|
| Simulated remediation against a productive incident | SRE / Security | 2026-Q4 | follow-up |
| Evolution-loop proposal that lands as a real OpenSpec change | SDLC | 2026-Q4 | follow-up |
| Production-grade healing-action policy bundle | Security | 2026-Q4 | follow-up |

## Approvers

When role-based approvers are replaced by named approvers, a signed git tag `phase-6-signoff-<YYYYMMDD>` is created on the merge commit that updates this file.

| Role | Name | Date (ISO-8601) | Decision | Notes |
|---|---|---|---|---|
| Platform | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| SRE Lead | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Security Lead | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Pilot Workspace Owner | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |

## Final Decision

Approved for bootstrap. Production readiness depends on completing the [Deferred Items](#deferred-items).
