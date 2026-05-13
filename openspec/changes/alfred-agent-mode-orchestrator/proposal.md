## Why

Alfred today is a *wizard helper*: a user types an intent in `/alfred`, Alfred drafts or edits an OpenSpec, and then humans drive scaffolding, CI, approvals and deploy by hand or via the `forge.reference.intent-to-deploy@1` workflow. The platform's own pitch — *autonomous agent-led SDLC* — is only half-true. We need Alfred to become the **end-to-end orchestrator** that takes an intent all the way to a deployed application within the workspace's autonomy preset, with humans only stepping in at policy-required approval gates.

Alfred also has **no visual identity**: the design notebook (`design/Forge Brand Notebook _standalone_.html`) contains zero references to Alfred, the portal ships no Alfred mark (only `app/icon.svg` for the Forge brand), and the only entry point is the `/alfred` page deep in the sidebar. To make agent-mode discoverable from any route, Alfred needs a coherent identity (icon, persona, copy voice, motion) and a portal-wide launcher pinned to the bottom-right corner.

## What Changes

- **NEW**: An **agent-mode session** on Alfred that supervises the full intent-to-deploy path autonomously — plans the run, dispatches sub-agents/workflows, watches CI, requests HITL approvals at policy-required gates, and reports back to the user — within the workspace's `autonomy_policy` ceilings.
- **NEW**: A portal-wide **Alfred dock** anchored to the bottom-right corner of every authenticated route — a floating launcher that opens a slide-in console showing the active session's plan, live decision stream, pending approvals and final outputs (PR URL, deploy URL).
- **NEW**: An **Alfred visual identity** added to the design system — SVG mark (monogram + ember/bowtie motif consistent with the Forge family), persona voice rules, color tokens, motion spec for the dock open/close/working states, and content guidelines for Alfred-authored messages (citations, criticality badges, "what I did next" summaries).
- **NEW**: A session-stream API (`GET /v1/sessions/{id}/stream`, SSE) on Alfred that the dock consumes for live updates; replays from the decision log for late joiners.
- **MODIFIED**: `alfred-control-plane` gains an *agent-mode session* model on top of the existing single-intent loop — long-running, plan-driven, capable of multi-step execution across `sdlc-orchestrator → scaffolder → ci → deploy-orchestrator`, and resumable across HITL pauses. Existing `/v1/intents` and wizard surfaces stay unchanged.
- **MODIFIED**: `intent-to-deploy-reference-flow` becomes **Alfred-supervised by default** — Alfred owns the workflow trigger, the per-step decision log, and the surfacing of the mandatory pre-prod HITL gate inside the dock (in addition to the approvals queue). The workflow contract itself does not change; only the operator.
- **MODIFIED**: `portal-shell` hosts the dock as a persistent shell affordance (fixed bottom-right, above the toast rail), gated by an `alfred:invoke` permission and a workspace-level feature flag, summonable via keyboard shortcut, focus-trapped when open, and respectful of the existing collapsed/responsive shell rules.
- **MODIFIED**: `portal-design-system` registers Alfred as a named brand surface — mark, color tokens, motion tokens, persona copy rules, and do/don't placement for the dock.
- **MODIFIED**: `delegated-permissions` adds `alfred:agent-mode.run` as a coarse action class so workspace admins can gate who/what may launch agent-mode sessions (defaults to `requires_approval` for `manual-prod`, `autonomous` for `full-autonomy`, etc.).
- **BREAKING**: None. The existing `/alfred` console, the wizard, single-intent loop and `forge.reference.intent-to-deploy@1` workflow keep working. Agent-mode is additive and feature-flagged off until the workspace admin opts in.

## Capabilities

### New Capabilities

- `alfred-agent-mode`: Long-running, plan-driven Alfred session that orchestrates intent-to-deploy end-to-end with policy-respecting autonomy, HITL pauses at required gates, a session SSE stream consumed by the portal dock, and resumability across pauses and disconnects.
- `alfred-dock`: Portal-wide bottom-right floating launcher and slide-in console for invoking Alfred from any authenticated route, showing live agent-mode session state, accepting follow-up intents, and routing the user to the underlying OpenSpec / PR / deploy URL.
- `alfred-identity`: Alfred's named brand surface inside the Forge design system — SVG mark, persona voice rules, color and motion tokens, content guidelines for agent-authored messages, and placement do/don'ts.

### Modified Capabilities

- `alfred-control-plane`: Adds the agent-mode session model (plan, steps, sub-agent dispatch, HITL pause/resume, SSE stream) alongside the existing single-intent loop. The dialogue API and wizard surface are unchanged.
- `intent-to-deploy-reference-flow`: Promotes Alfred from "trigger source" to **supervising operator** of the reference workflow — every step's policy/decision is logged against the Alfred session, and the pre-prod HITL gate surfaces in the dock as well as the approvals queue.
- `portal-shell`: Hosts the Alfred dock as a persistent shell affordance with keyboard summon, focus trap, permission gating and responsive rules (auto-collapses to icon-only below 1024px).
- `portal-design-system`: Registers Alfred mark, color tokens, motion tokens and persona copy rules so any future Alfred-adjacent UI (notifications, audit excerpts, run cards) renders consistently.
- `delegated-permissions`: Adds the `alfred:agent-mode.run` action class and its mapping in the default workspace autonomy presets.

## Impact

- **Code**:
  - `services/alfred`: new `alfred/agent_mode/` module (session model, plan state machine, sub-agent dispatcher, SSE stream); reuses `alfred/loop.py` for per-step LLM/policy/permission cycles; reuses `alfred/autonomy_presets.py` for ceilings.
  - `portal/src/components/shell/`: new `AlfredDock.tsx` mounted by `PortalShell.tsx` (below `ToastRail`), `useAlfredSession()` hook, `/api/alfred/sessions/[id]/stream` SSE proxy, hotkey wiring.
  - `portal/src/i18n/dictionary.ts`: dock and persona copy keys (ES/EN).
  - `design/`: new `design/alfred-identity/` folder containing the SVG mark sources, persona note (`PERSONA.md`), motion spec (`MOTION.md`), do/don't board (`DO_DONT.md`), and an updated entry in the Brand Notebook standalone HTML referencing the new mark and tokens.
  - `portal/public/`: `alfred-mark.svg`, `alfred-mark-mono.svg`, `alfred-mark-working.svg` (animated state).
- **APIs**:
  - `POST /v1/agent-mode/sessions` (start), `GET /v1/agent-mode/sessions/{id}` (state), `GET /v1/agent-mode/sessions/{id}/stream` (SSE), `POST /v1/agent-mode/sessions/{id}/messages` (follow-up intent), `POST /v1/agent-mode/sessions/{id}/cancel`.
  - Portal proxy: `/api/alfred/dock/*` to keep the gateway token server-side.
- **Database**: new `alfred_agent_session` (id, workspace_id, openspec_id, plan_json, status, started_at, paused_at, completed_at, correlation_id) and `alfred_agent_step` (session_id, idx, kind, tool_id, decision_id, status, started_at, completed_at). Reuses existing `decision` table via FK.
- **Events**: `com.forge.alfred.agent_mode.session_started.v1`, `…step_started.v1`, `…step_completed.v1`, `…paused_for_approval.v1`, `…resumed.v1`, `…completed.v1`, `…aborted.v1`.
- **Policy**: extends OPA bundles with `alfred:agent-mode.*` rules; `delegated-permissions` default presets updated; LiteLLM tenant budgets apply to agent-mode LLM calls.
- **Telemetry / audit**: agent-mode session is a first-class object in `per-asset-observability` (cost, latency, success rate, HITL-pause rate per workspace).
- **Compose / runtime**: no new services; Alfred container picks up the new module. Feature flag `ALFRED_AGENT_MODE_ENABLED` defaults to off; per-workspace toggle stored alongside autonomy presets.
- **Docs**: `docs/runbooks/alfred-agent-mode.md` (operator runbook), updates to `docs/runbooks/intent-to-deploy-demo.md` covering the dock-driven path.
- **Out of scope (deferred)**: multi-workspace session fan-out, voice/ambient invocation, mobile dock layout, custom per-workspace Alfred personas. The dock is desktop-first; tablet width auto-collapses it to its icon. Public/A2A agent-mode invocation through `developer-skill-gateway` is left to a follow-up change.
