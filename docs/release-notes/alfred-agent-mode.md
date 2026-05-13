# Release notes — Alfred agent-mode

This change promotes Alfred from a per-call wizard into a long-running,
plan-driven supervisory operator for the intent-to-deploy reference flow and
adds a portal-wide dock surface plus a first-class Alfred brand identity.

## What's new

- **Agent-mode sessions** — `POST /v1/agent-mode/sessions` opens a stateful
  Alfred session that plans, dispatches and supervises the
  `forge.reference.intent-to-deploy@1` workflow end-to-end while respecting
  the workspace's frozen autonomy preset. Sessions resume across HITL pauses,
  budget pauses and service restarts.
- **Portal dock** — a floating Alfred mark anchored to the bottom-right of
  every authenticated route. `Alt+A` summons it. SSE stream powers a live
  transcript. The dock is mutually exclusive with the command palette.
- **Alfred identity** — SVG mark family (`alfred-mark`, `alfred-mark-mono`,
  `alfred-mark-working`), CSS-only working animation, persona voice rules
  enforced by `scripts/lint-alfred-copy.mjs`. The standalone Brand Notebook
  picks up the new Alfred section via `make design-export`.

## Rollout phases

| Phase | Default | Notes |
| --- | --- | --- |
| 0 | `ALFRED_AGENT_MODE_ENABLED=false` everywhere | Code paths ship; surface stays gated. Existing tests pass unchanged. |
| 1 | Internal workspace dogfood | Workspace flag `alfred.dock_enabled` toggled on for `forge.internal` only. CI smoke test (`make demo-intent-to-deploy`) drives the default agent-mode path. |
| 2 | Opt-in tenants | Workspace admins flip the flag in `/admin/alfred`. Observability tile (`cost_per_session_p95`, success rate, HITL-pause rate) lands in the per-asset dashboard. |
| 3 | Default-on for new workspaces | Workspace creation template sets `alfred.dock_enabled=true`. Existing workspaces stay opt-in. |

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `ALFRED_AGENT_MODE_ENABLED` | `false` | Global feature flag. When `false`, the `/v1/agent-mode/*` surface returns 503 and the portal dock launcher is hidden via permission gate. |
| `WORKFLOW_RUNTIME_URL` | `http://localhost:8093` | Used by the executor's workflow dispatcher. |
| `ALFRED_AGENT_MODE_MODEL` | `gemini-1.5-pro` | Pinned model for plan generation and follow-up evaluation. Override per environment. |
| `ALFRED_PRESET_DIR` | `/var/lib/forge/alfred/presets` | Filesystem location of workspace preset + settings JSON. |
| `alfred.dock_enabled` (workspace) | `false` | Per-workspace JSON setting. Flipped from `/admin/alfred` in the portal. |

## Rollback

Set `ALFRED_AGENT_MODE_ENABLED=false` on the Alfred deployment and restart.
The launcher disappears from the portal, new session calls return 503, and
existing sessions complete naturally with their audit rows intact. No schema
rollback is required.

## Related artifacts

- Runbook: [docs/runbooks/alfred-agent-mode.md](../runbooks/alfred-agent-mode.md)
- Design notes: [openspec/changes/alfred-agent-mode-orchestrator/design.md](../../openspec/changes/alfred-agent-mode-orchestrator/design.md)
- Persona and motion: [design/alfred-identity/](../../design/alfred-identity/)
