# Alfred agent-mode — on-call runbook

Alfred agent-mode is a long-running, plan-driven supervisory session over the
[forge.reference.intent-to-deploy@1](../../services/workflow-registry/seeds/forge.reference.intent-to-deploy.v1.yaml)
reference workflow. This page is the operator's handbook for the surface.

## At a glance

| Item | Value |
| --- | --- |
| Service | `services/alfred` |
| Entry point | `POST /v1/agent-mode/sessions` |
| Live stream | `GET /v1/agent-mode/sessions/{id}/stream` (SSE) |
| Feature flag (global) | `ALFRED_AGENT_MODE_ENABLED` |
| Feature flag (workspace) | `alfred.dock_enabled` (per-workspace JSON setting) |
| Permission to start | `alfred:agent-mode.run` |
| Permission to cancel | `alfred:agent-mode.cancel` |
| Audit events | `alfred.agent_mode.*` (CloudEvents) |
| Storage tables | `alfred_agent_session`, `alfred_agent_step` |

## Start a session

The portal dock is the default surface. From any authenticated route:

1. Press `Alt+A` (`⌥A` on macOS) or click the floating Alfred mark in the
   bottom-right.
2. Enter the OpenSpec id and the intent text.
3. Click **Start agent**.

Direct API:

```bash
curl -X POST "$ALFRED_URL/v1/agent-mode/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"workspace_id":"<uuid>","openspec_id":"spec-1","intent":"…"}'
```

## Inspect the dock

The dock shows:

- The current plan checklist (each step's status as a glyph)
- A live transcript (durable; replayable via SSE `Last-Event-ID`)
- The follow-up composer
- Any artifact links Alfred surfaces (PR URL, deploy URL)

The launcher mark animates when a session is `running` or `planning`; the
animation is suppressed under `prefers-reduced-motion`.

## Approve or reject

When a session pauses at a policy-required gate, Alfred opens an approval in
the existing approvals queue **and** surfaces it inline in the dock. Approvers
can act in either surface; both paths produce the same audit record.

The dock badge transitions from "Working" to "Paused for approval".

## Resume after a pause

Approval grants are observed by the existing approvals listener. The
executor's `resume` helper is invoked automatically — no manual step is
needed. For an explicit replay or after a service restart, fire:

```bash
curl -X POST "$ALFRED_URL/v1/agent-mode/sessions/<id>/_resume" \
  -H "Authorization: Bearer $TOKEN"
```

A rejected approval transitions the session to `aborted`.

## Cancel a session

From the dock — click **Cancel session**. From the API:

```bash
curl -X POST "$ALFRED_URL/v1/agent-mode/sessions/<id>/cancel" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason":"customer asked us to roll back"}'
```

Cancellation respects `alfred:agent-mode.cancel`. In-flight steps are allowed
to finalize; subsequent steps are skipped.

## Common failure modes

### Session paused with `paused_for_budget`

The LiteLLM tenant budget is exhausted. Top up the tenant budget in
`services/finops` or wait for the next budget window. The executor resumes
automatically on the next probe success.

### Session aborted with `permission_denied`

Alfred lacks a delegated permission for the action class. Audit:

```bash
curl "$ALFRED_URL/v1/agent-mode/sessions/<id>/decisions" -H "Authorization: Bearer $TOKEN"
```

Look for the failed decision row — it carries the `action_class` and the
permission service's `reason`.

### Plan revision loops

If the same step fails repeatedly the executor keeps revising. The dashboard
`cost_per_session_p95` tile spikes. Cancel the session and review the failing
step's decision row before restarting with a tightened plan.

### SSE stalls in the dock

The proxy heartbeats every 15 s. If the dock shows stale state, hard-refresh
the page — the SSE client reconnects with `Last-Event-ID` and replays missed
events from the durable step log.

## Rollback

Set `ALFRED_AGENT_MODE_ENABLED=false` and restart the Alfred service. The
dock launcher disappears, new session calls return `503`, and existing
sessions complete naturally (no schema rollback required).

## Related pages

- [docs/runbooks/intent-to-deploy-demo.md](intent-to-deploy-demo.md) — the
  demo that drives the same path.
- [design/alfred-identity/PERSONA.md](../../design/alfred-identity/PERSONA.md)
  — voice and copy rules.
- [openspec/changes/alfred-agent-mode-orchestrator/design.md](../../openspec/changes/alfred-agent-mode-orchestrator/design.md)
  — the design decisions behind the surface.
