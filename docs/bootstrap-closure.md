# Bootstrap closure — Forge Engineering Fabric

Phase 6 closes the bootstrap roadmap. Forge now operates an end-to-end loop:
detect → diagnose → heal → postmortem → evolve. This document captures the
hand-off from "bootstrap mode" to "productive operation".

## What's in production at closure

- **Phases 0–4** — tenancy, IAM, registry, observability, Alfred + RAG,
  OpenSpec backbone, MCPs, app onboarding, deploys, SDLC orchestration.
- **Phase 5** — workflow marketplace, advanced eval harness, asset-level
  observability, FinOps reports.
- **Phase 6** — incident detection, diagnosis pipeline, healing engine
  (L1..L5), incidents KB, postmortem generator, evolution loop, FinOps
  recommendations.

## Workshop

A retrospective workshop with all stakeholders is scheduled for the week of
sign-off. Agenda:

1. Roadmap recap — what shipped vs. what slipped.
2. Production telemetry — first month of healing decisions, MTTR, auto-rollback rate, postmortem cadence.
3. Top 3 pain points by Workspace pilot.
4. Decision: post-bootstrap roadmap as discrete OpenSpec changes (no
   monolithic "phase 7"). Owners are recorded per change.

Notes are committed to `docs/retrospectives/bootstrap-closure-retro.md`.

## Post-bootstrap roadmap

Future work is tracked as **standalone OpenSpec changes** under
`openspec/changes/`. Examples in flight or planned:

- KB cross-tenant anonymized sharing (consent-gated).
- L5 in prod allowance for explicitly low-blast-radius reversible actions.
- On-call rotation integration with Approvals Inbox.

These are explicitly **not** part of phase-6-autonomous-ops; they ship as
incremental, governed changes through the same OpenSpec → eval → deploy
pipeline that Forge now operates for any Tenant.

## Internal announcement template

> **Forge enters productive multi-tenant operation with autonomous ops.**
>
> Today Forge closes the bootstrap roadmap. Every team can now ship through
> the platform end-to-end, with Phase 6 autonomous-ops live in pilot
> Workspaces. Detection → diagnosis → healing → postmortem → evolution runs
> on every incident, with strict envelopes, kill switches, and promotion
> gates protecting production.
>
> What changes today:
> - All new assets default to L1/L2 healing. Promotions follow D6.10.
> - Postmortems are auto-generated; humans review and accept the
>   `autonomous-loop` proposals in the Evolution Inbox.
> - FinOps recommendations open draft PRs; teams approve via the standard
>   Approvals Inbox.
>
> Future improvements ship as OpenSpec changes — no more "phase" planning.
>
> — Platform team

## Sign-off ceremony

The sign-off table in
`openspec/changes/phase-6-autonomous-ops/sign-off.md` is filled in during the
workshop. Once every stakeholder has signed, the change is archived via:

```
/opsx:archive phase-6-autonomous-ops
```
