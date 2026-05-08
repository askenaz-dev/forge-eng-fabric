# Healing Levels (L1..L5)

The Forge healing engine operates at five graduated autonomy levels. Higher
levels imply more independence from human reviewers and stricter prerequisites
to promote actions into them.

## Levels at a glance

| Level | Name | What the engine does | Human review |
|------:|------|----------------------|--------------|
| L1 | Notify | Detects and alerts. Performs no action. | Implicit (informational only). |
| L2 | Suggest | Proposes a concrete healing action with rationale. | Required to act. |
| L3 | Act with approval | Creates an Approvals Inbox entry; on approval, executes the workflow within TTL. | Required per invocation. |
| L4 | Act autonomously | Executes the workflow within the envelope; emits audit + notify. | Not required per invocation; envelope is human-approved up front. |
| L5 | Act and rollback | Executes the workflow, verifies, and auto-rolls-back on verify failure. | Not required per invocation; reversibility is mandatory. |

## Defaults

- New assets / envelopes default to **L1 or L2** until a policy explicitly raises them.
- **L4 / L5** are never default in `prod`. They require:
  - eval suite ≥95% pass rate over the last 50 runs;
  - ≥20 successful L3 dry-run executions;
  - 30-day postmortem-free window;
  - dual approval (`platform-admin` + `security-approver`).
- **L5 in prod** is restricted to actions with `reversible: true` and
  `critical_blast_radius=false` (cache refresh, log level toggle, etc.).
  Destructive prod actions are never L5.

## Decision flow

```
incident → suggested_actions[] → resolve in catalog → consult envelope
   → kill_switch? ─ yes → L1 (Notify)
   → envelope cap? ─ yes → degrade to highest allowed
   → action.allowed_levels_by_env? ─ if smaller, degrade further
   → rate-limit budget? ─ exceeded → blocked_by_rate_limit
   → run path:
       L1 → notified
       L2 → suggested
       L3 → approval inbox + wait → execute on approval
       L4 → execute + audit + notify
       L5 → execute + verify → rollback on failure
```

The level applied is recorded in `healing_action_invocation.applied_level`.
The requested level (before degradation) is recorded in `requested_level`. The
diff between the two — and the reason — is published via
`healing.level_decided.v1`.

## Audit and observability

Every invocation emits a sequence:

1. `healing.triggered.v1` — incident matched, action picked.
2. `healing.level_decided.v1` — level applied (with reason).
3. `healing.executed.v1` — workflow result (executed / failed / waiting_approval).
4. `healing.rolled_back.v1` (L5 only) — issued when verify fails.
5. `healing.escalated.v1` — issued whenever an L4/L5 invocation fails after
   rollback.

Dashboards in Grafana ("Phase 6 — Autonomous Ops") aggregate:
- `healing_invocations_total{level}`
- `healing_invocations_outcome_total{outcome}`
- `mttr_seconds`
- `auto_rollback_rate`
- `kill_switch_activation_count`

## Promotion (D6.10)

To raise an action's allowed level in an env, call:

```
POST /v1/actions/promote
{
  "action_id": "restart-pod",
  "environment": "stage",
  "target_level": "L4",
  "platform_admin_ok": true,
  "security_ok": true
}
```

The engine checks `healing_action_promotion_stats` and rejects with
`ErrPromotionPrerequisites` if any prerequisite is unmet.

## Kill switch

`POST /v1/kill-switch` toggles either the global flag (`workspace_id` empty)
or a workspace flag. Active state degrades all in-flight decisions to L1.
The flag is cached in the engine for 30 seconds.

See [docs/healing/envelopes.md](../healing/envelopes.md) for envelope semantics.
