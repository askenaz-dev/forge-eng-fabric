# Phase 1 Sign-Off

Status: Approved for bootstrap (role-based). Named approvers pending replacement of the role-based row when integrated staging validation completes.

> **Reference change**: [`phase-1-agentic-core`](../../openspec/changes/archive/2026-05-09-phase-1-agentic-core/) (archived 2026-05-09).

## Scope Summary

Phase 1 establishes the agentic core: Alfred as the Control Plane Agent, the OpenSpec backbone for intent capture, the policy engine for autonomous-vs-approval decisions, the Registry promotion lifecycle with eval gates, and the SDLC orchestrator's foundational Skills (`create-user-stories`, `generate-test-cases`). Sign-off attests that the local-stack smoke covers the agentic surface end-to-end and that integrated staging validation is documented for the environment that has not yet been provisioned.

## Exit Criteria Checklist

Each item links to the corresponding archived change requirement, spec, or task.

- [x] Alfred Control Plane Agent dispatches actions through Tool Use — see [`alfred-control-plane` spec](../../openspec/specs/alfred-control-plane/spec.md).
- [x] OpenSpec backbone with Jira/Confluence linking — see [`openspec-backbone` spec](../../openspec/specs/openspec-backbone/spec.md).
- [x] Policy engine returns `autonomous`, `requires_approval`, `requires_dual_control` — see [`policy-engine` spec](../../openspec/specs/policy-engine/spec.md).
- [x] Registry T1 eval-gated promotion — see [`registry-baseline` spec](../../openspec/specs/registry-baseline/spec.md).
- [x] SDLC Skills `create-user-stories` and `generate-test-cases` registered — see [`sdlc-orchestrator` spec](../../openspec/specs/sdlc-orchestrator/spec.md).
- [x] Delegated permissions with scope/expiration — see [`permissions` spec](../../openspec/specs/permissions/spec.md).
- [x] Single `correlation_id` propagated across Alfred → tool → audit → Langfuse → Tempo — see [`telemetry-and-correlation` spec](../../openspec/specs/telemetry-and-correlation/spec.md).
- [x] Alfred Console `/openspec create` path with Jira/Confluence links — see `portal/src/app/alfred/page.tsx`.
- [x] Local E2E smoke passes — see `make smoke`.
- [ ] Integrated staging validation against real Jira / Confluence / Keycloak / OpenFGA / Langfuse / Tempo / GitHub — **deferred**, see [Deferred Items](#deferred-items).

## Local Evidence

- Alfred local E2E smoke covers OpenSpec creation intent, Jira/Confluence linked artifacts, a 7-day `openspec:write` delegated grant, policy behavior for `deploy:prod`, reference Skill invocation, schema validation, decision audit and Langfuse-compatible correlation IDs.
- Policy engine golden tests cover `deploy:prod` requiring approval and default autonomous behavior.
- Registry lifecycle tests cover T1 eval threshold rejection and production-relevant invocation blocking for `in_review` assets.
- Automated staging integration tests live in `services/registry/tests/test_integration_promotion.py` and write JSON evidence under `docs/governance/evidence/phase-1/<timestamp>/`.
- Portal build validates the Alfred Console `/openspec create` command path, including Jira and Confluence link fields.

## Integrated Evidence Checklist

- [ ] Alfred Console creates an OpenSpec in a real Workspace with bidirectional Jira story and Confluence page links.
- [ ] Alfred receives an active delegated permission with `scope=Workspace`, `action_class=openspec:write` and `expiration=7d`.
- [ ] Workspace policy is configured so `deploy:prod` returns `requires_approval` and non-prod actions remain autonomous.
- [ ] `create-user-stories` and `generate-test-cases` run against the OpenSpec, validate outputs against schemas and appear in audit and Langfuse.
- [ ] A T1 asset promotion is rejected with failing evals, then accepted with passing evals.
- [ ] A production-relevant invocation of an `in_review` asset is blocked and audited.
- [ ] A single `correlation_id` links the Alfred intent, decision log, tool calls, Langfuse trace and Tempo trace.

## Evidence Links

| Evidence | Type | Location |
|---|---|---|
| Local E2E smoke (Alfred) | JSON evidence | `docs/governance/evidence/phase-1/local-smoke-2026-05-09.json` |
| Policy engine golden tests | Test report | `services/policy-engine/tests/golden/` |
| Registry T1 promotion test | Test report | `services/registry/tests/test_promotion.py` |
| Staging integration test | Pending | **Deferred** — runs once staging is up |
| Portal Alfred Console build | CI run | `docs/governance/evidence/phase-1/portal-build.log` |

## Deferred Items

| Item | Owner | Target | Tracker |
|---|---|---|---|
| Integrated staging validation across real dependencies | SDLC + Platform | 2026-Q3 | `platform-gaps-closure` 6.4–6.7 |
| Reference workflow E2E run against staging GKE | SDLC | 2026-Q3 | `platform-gaps-closure` 8.2 |
| Wizard-driven intent capture (replaces slash-command surface for non-technical users) | SDLC + Portal | 2026-Q3 | `platform-gaps-closure` 3.1–3.12 |

## Approvers

When the role-based approval is replaced by named approvers, the table below is updated and a signed git tag `phase-1-signoff-<YYYYMMDD>` is created at the merge commit that updates this file.

| Role | Name | Date (ISO-8601) | Decision | Notes |
|---|---|---|---|---|
| SDLC Team Lead | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Platform Owner | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Security / IAM Lead | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| SRE Lead | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Pilot Workspace Owner | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |

## Final Decision

Approved for bootstrap using role-based approvers. Production readiness depends on completing the [Deferred Items](#deferred-items) and replacing the role-based row with named approvers; that update is the trigger for the signed `phase-1-signoff-<YYYYMMDD>` git tag.
