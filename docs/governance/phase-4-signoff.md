# Phase 4 Sign-Off

Status: Approved for bootstrap (role-based). Named approvers pending replacement of the role-based row when eval-gated promotion evidence is attached to a live SDLC initiative.

> **Reference change**: [`phase-4-sdlc-orchestration`](../../openspec/changes/archive/2026-05-09-phase-4-sdlc-orchestration/).

## Scope Summary

Phase 4 makes Forge expose SDLC capabilities (product, architecture, design, development, QA, security, DevOps, SRE, FinOps) as governed Skills bound to capability-aware policies. Sign-off attests that the Skill registry, prompt templates, capability policies, and the reference workflow are operable end-to-end.

## Exit Criteria Checklist

- [x] Reference SDLC Skills registered in `services/registry` — see `skills/reference/agent-skills/` and the seed automation.
- [x] Eval reports per capability — see `docs/eval-harness/`.
- [x] Capability-bound policies in `services/policy-engine` — see [`policy-engine` spec](../../openspec/specs/policy-engine/spec.md).
- [x] Prompt templates seeded in `prompt-registry` — see `skills/reference/prompt-templates/`.
- [x] Reference workflow `forge.reference.intent-to-deploy@1` registered — see `services/workflow-registry/seeds/`.
- [x] ≥ 1 successful run of the reference workflow against staging GKE — see evidence below.

## Evidence Links

| Evidence | Type | Location |
|---|---|---|
| Skill registration | Registry list output | `make seed-registry` |
| Eval reports | JSON per capability | `docs/governance/evidence/phase-4/evals/` |
| Capability policies | Policy bundle | `services/policy-engine/bundles/` |
| Prompt templates | Registry list output | `services/prompt-registry` |
| Reference workflow | Registry record | `forge.reference.intent-to-deploy@1.0.0` |
| End-to-end local run | JSON report | `docs/governance/evidence/phase-4/demo-intent-to-deploy-local-20260510T033437Z.json` (`workflow_trigger_synthetic=false`) |
| End-to-end staging GKE run | JSON report | `docs/governance/evidence/phase-4/demo-intent-to-deploy-staging-gke-20260510T194106Z.json` (`workflow_trigger_synthetic=false`, ephemeral cluster deleted) |
| Reference workflow smoke | Smoke log | `docs/governance/evidence/phase-4/smoke-live.log` |
| Wizard transcript (non-engineer evaluator) | Transcript | `docs/governance/evidence/phase-4/wizard-nontechnical-transcript-20260510.md` |

## Deferred Items

| Item | Owner | Target | Tracker |
|---|---|---|---|
| Live SDLC initiative running through the orchestrator end-to-end | SDLC | 2026-Q3 | follow-up |

## Approvers

When role-based approvers are replaced by named approvers, a signed git tag `phase-4-signoff-<YYYYMMDD>` is created on the merge commit that updates this file.

| Role | Name | Date (ISO-8601) | Decision | Notes |
|---|---|---|---|---|
| Platform | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| SDLC Lead | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Security Lead | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Pilot Workspace Owner | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |

## Final Decision

Approved for bootstrap. Production readiness depends on completing the [Deferred Items](#deferred-items).
