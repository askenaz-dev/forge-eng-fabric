## Context

Alfred ships today as a stateless, single-iteration agent in [services/alfred/alfred/loop.py](services/alfred/alfred/loop.py): one HTTP call to `/v1/intents` runs at most `max_iterations` LLM steps, each step is policy-checked and audited, and the function returns. The portal's `/alfred` console (server-rendered form at [portal/src/app/alfred/page.tsx](portal/src/app/alfred/page.tsx)) and the wizard ([portal/src/app/alfred/wizard/page.tsx](portal/src/app/alfred/wizard/page.tsx)) are the only entry points; both are page-scoped — there is no portal-wide invocation surface.

End-to-end orchestration (intent → deploy) is currently the job of a hand-coded workflow `forge.reference.intent-to-deploy@1` (spec at [openspec/specs/intent-to-deploy-reference-flow/spec.md](openspec/specs/intent-to-deploy-reference-flow/spec.md)) that runs in `workflow-runtime` and is triggered manually. Alfred is barely involved past the OpenSpec commit.

Visual identity: the design notebook ([design/Forge Brand Notebook _standalone_.html](design/Forge Brand Notebook _standalone_.html)) contains zero references to Alfred — verified by `Grep` — and the portal's only mark is `app/icon.svg` for the Forge brand. The user explicitly calls this out.

Stakeholders touched: Alfred service team (services/alfred), portal team (portal/src), platform-policy team (OPA, OpenFGA, delegated-permissions), design (design/), workflow-runtime team (to be a *callee*, not a *caller*, of Alfred under agent-mode).

## Goals / Non-Goals

**Goals**

- Promote Alfred from per-call wizard into a **long-running, plan-driven, autonomous operator** that supervises the intent-to-deploy reference workflow end-to-end while respecting the workspace's frozen `autonomy_policy`.
- Make Alfred reachable from every authenticated route via a **bottom-right launcher + slide-in dock** with keyboard summon, live SSE transcript, follow-up composer, and approval surfacing.
- Give Alfred a **first-class brand identity** (mark, persona voice, color/motion tokens, do/don't board) added into the existing design notebook and ready to drive any future Alfred-adjacent UI.
- Keep the existing single-intent loop, wizard dialogue, slash-command console and direct workflow trigger working unchanged.

**Non-Goals**

- Multi-workspace fan-out of a single session, voice/ambient invocation, mobile-first dock layout, custom per-workspace personas.
- Public/A2A invocation of agent-mode through the developer-skill-gateway — that's a follow-up tied to [openspec/changes/add-developer-skill-gateway](openspec/changes/add-developer-skill-gateway).
- Re-platforming `workflow-runtime`. Alfred remains a *supervisor* on top of it, not a replacement; the workflow contract is unchanged.
- A new visual brand. We adopt and extend the existing Forge mark family rather than re-skinning.

## Decisions

### D1 — Agent-mode lives **inside** Alfred, not in workflow-runtime

Alfred owns the plan, the per-step policy decision, the HITL pauses and the SSE stream. `workflow-runtime` executes the workflow exactly as today; Alfred triggers it via the existing API and consumes its step events.

*Why not the inverse (extend workflow-runtime with a "planner step")?* The planner needs the same RAG / LiteLLM / OPA / approvals plumbing Alfred already owns ([alfred/loop.py](services/alfred/alfred/loop.py), [alfred/gateways.py](services/alfred/alfred/gateways.py), [alfred/autonomy_presets.py](services/alfred/alfred/autonomy_presets.py)). Duplicating that into workflow-runtime would fork policy enforcement — exactly what the spec forbids. Alfred-as-supervisor reuses every existing gate and audit path.

*Alternative considered*: a thin "Alfred-as-DAG-node" inside the workflow. Rejected because the session needs to outlive a single workflow run (e.g., post-deploy verify, follow-up intents, multi-revision plans) and Alfred-as-DAG-node would force re-creating session state inside workflow context.

### D2 — Plan is data, not code

A session row stores `plan_json` (an array of `step` records: `{idx, kind, tool_id|workflow_id|agent_id, criticality, status, decision_id, started_at, completed_at}`) plus a `plan_revision` counter. Replanning produces a new revision; we never mutate an old revision. Each step's `decision_id` foreign-keys back into the existing `decision` table so the audit path is unified.

This makes plans inspectable, diff-able (revision N vs N+1), replayable in tests, and trivial to render in the dock without bespoke domain types on the frontend.

*Alternative considered*: state machine encoded as Python classes per plan-shape. Rejected — agent-mode plans will evolve quickly (e.g., add post-deploy canary), and a typed state machine becomes the bottleneck. Data-driven plans + a thin executor is cheaper to evolve.

### D3 — Per-step execution reuses the **same** loop primitives

Each step that calls a tool/MCP/sub-agent goes through the exact same `PermissionsClient.can` → `PolicyClient.evaluate` → `ApprovalsClient.request` → `tool_handler` sequence as a single-intent iteration in [alfred/loop.py:196-340](services/alfred/alfred/loop.py:196). Agent-mode is implemented as `alfred/agent_mode/executor.py` that loops over `plan.steps` and delegates each to that same primitive.

This is the single most important constraint: **no parallel policy path**. Agent-mode cannot become a way to bypass any check that applies to a single-intent call.

### D4 — Session state is frozen at start

The session row stores a snapshot of the workspace's active `autonomy_policy` (sourced from [alfred/autonomy_presets.py](services/alfred/alfred/autonomy_presets.py)) at creation. Admin edits to the preset after start do not retroactively widen or narrow the running session.

*Why*: prevents racy admin edits from changing the meaning of an in-flight session — both for safety (an admin can't accidentally unlock a session by relaxing the preset) and for predictability for the user watching the dock.

*Mitigation for "I really need to tighten this NOW"*: workspace admins can `POST /cancel` on the session, which respects the existing `alfred:agent-mode.cancel` permission.

### D5 — Streaming via SSE on a durable event log

Live updates use SSE (`GET /v1/agent-mode/sessions/{id}/stream`). Events are sourced from the `decision` table and the `alfred_agent_step` table, so the stream is a *view* of durable state. Clients reconnect with `Last-Event-ID`; the server replays from that point before going live.

*Why not WebSockets?* Single-direction server→client traffic is enough (follow-ups go through `POST /messages`). SSE plays nice with existing FastAPI middleware and corporate proxies, and reconnection is built in.

*Why not Kafka direct to the browser?* Cross-tenant safety and the need for replay-by-id are easier behind an HTTP boundary that already enforces auth and tenancy.

### D6 — Dock is a shell affordance, not a page

`AlfredDock` mounts at [portal/src/components/shell/PortalShell.tsx](portal/src/components/shell/PortalShell.tsx) alongside `ToastRail` and `CommandPalette`. It survives client-side route transitions because the shell does. State lives in a React context (`AlfredSessionProvider`) so any route can read the active session without prop-drilling.

*Alternative considered*: dedicated `/alfred-dock` route rendered inside a portal. Rejected — would lose the "follow the user across routes" property and require an iframe or portal hack to feel persistent.

### D7 — Identity is a folder, the notebook is a generated artifact

Source-of-truth for Alfred identity lives under `design/alfred-identity/` (SVG sources, `PERSONA.md`, `MOTION.md`, `DO_DONT.md`). The standalone HTML notebook (`design/Forge Brand Notebook _standalone_.html`) is regenerated to include the Alfred section, with marks inlined as base64-free SVG so it stays single-file. Production portal assets (`portal/public/alfred-mark*.svg`) are exported from the same sources by `make design-export`.

*Why a folder, not assets-in-portal?* The notebook + persona docs are design deliverables that live with the rest of `design/` and don't belong in the portal bundle. The portal only ships the optimized marks.

### D8 — Feature flag + permission gate, not Big Bang

Two switches gate the rollout:

- Workspace-level flag `alfred.dock_enabled` (stored alongside autonomy presets) — defaults `false`; admin opts in per workspace.
- Coarse permission `alfred:agent-mode.run` — defaults to `requires_approval` for `manual-prod`, `autonomous` for `full-autonomy` / `staging-only`.

The dock launcher only renders when both `alfred:invoke` (existing) and the workspace flag are true. Starting a session also requires `alfred:agent-mode.run`. This lets cautious workspaces try the dock as a read-only stream of others' sessions first, then graduate to launching their own.

### D9 — Approval surfacing is **dual-path**, not replaced

A `paused_for_approval` session does **two** things: opens the same approval request in the existing approvals queue (so workspace approvers see it where they already look) **and** renders the gate inside the originator's dock with a one-click deep link. We never replace the approvals queue surface — that would split the audit/UX story for approvers who don't use the dock.

### D10 — Workflow-runtime stays the executor; agent-mode adds a `session_id` correlation

`workflow-runtime` continues to emit its own step events. The smoke test asserts the **interleaved** sequence (workflow events + `alfred.agent_mode.*` events) sharing a `correlation_id`. The agent-mode session row records the workflow's `run_id`, so joining the audit graph is one query, not a re-derivation.

## Risks / Trade-offs

- **[Plan drift across LLM versions]** → Different model versions may produce different plans for the same OpenSpec. Mitigation: pin the agent-mode model in `LiteLLM` config, cover with eval suite in `services/alfred/tests/test_phase1_e2e.py` patterned after existing loop tests, and surface the model id on the session row + in the dock header.
- **[Long-running sessions outlive a single Alfred pod]** → Sessions can pause for hours awaiting HITL. Mitigation: all state is in Postgres; pods are stateless; resumption is just `SELECT` + `dispatch_next_step`. The SSE stream reconnect protocol uses `Last-Event-ID` against the decision log so a deploy-time pod restart is invisible to the dock.
- **[Cost runaway]** → Multi-step LLM plans can burn budget. Mitigation: every step still passes through the LiteLLM tenant budget probe (D3), agent-mode adds a `paused_for_budget` terminal status surfaced in the dock, and per-workspace observability gains a `cost_per_session_p95` metric.
- **[Dock racing with command palette / modals]** → Two global keyboard listeners + a persistent floating element is a focus-management minefield. Mitigation: single shared `ShellHotkeyProvider` already exists for the palette; we extend it rather than mounting a parallel listener. Focus trap rules and modal layering are codified in the `portal-shell` requirements.
- **[Design notebook regression]** → Embedding inline SVG into the standalone HTML risks bloating the file or breaking offline rendering. Mitigation: SVGs are optimized (`svgo`), the standalone HTML is regenerated by a deterministic `make design-export` target with a size assertion in CI (`bytes < 3 MB`).
- **[Backwards-compat with `/alfred` console]** → Power users rely on the slash-command console. Mitigation: the page stays exactly as today; the dock is additive; the existing `submitConsole` server action ([portal/src/app/alfred/page.tsx:19](portal/src/app/alfred/page.tsx:19)) is untouched.
- **[Policy regression on follow-ups]** → A creative follow-up could try to escalate the session. Mitigation: every follow-up is re-evaluated against the **frozen** preset (D4), unauthorized intents produce `autonomy.override.rejected.v1` and are audited; we add an integration test that explicitly attempts an escalation.

## Migration Plan

1. **Phase 0 — flag off everywhere**: ship the new code paths behind `ALFRED_AGENT_MODE_ENABLED=false` global default. Existing tests (`services/alfred/tests/test_loop.py`, `test_phase1_e2e.py`) keep passing because no code paths change.
2. **Phase 1 — internal workspace dogfood**: enable the workspace flag on the `forge.internal` workspace only. Run `make demo-intent-to-deploy` (default agent-mode path) and the smoke test in CI against ephemeral infra.
3. **Phase 2 — opt-in tenants**: ship the workspace-level flag toggle UI in the admin surface; tenants opt themselves in per workspace. Observability dashboards in `per-asset-observability` show cost / HITL-pause rate / success rate per session.
4. **Phase 3 — default-on for new workspaces**: switch the workspace creation template to set `alfred.dock_enabled=true` by default; existing workspaces stay opt-in.
5. **Rollback**: at any phase, flipping `ALFRED_AGENT_MODE_ENABLED=false` immediately stops the launcher from rendering and refuses new `POST /v1/agent-mode/sessions` with 503. Existing sessions complete naturally (rows persist for audit). No schema rollback needed.

## Open Questions

- **OQ1**: Should the dock render across workspaces (e.g., "I have 3 sessions across A and B") or strictly the active workspace? Current spec is "active workspace only" (D6 / portal-shell delta). Revisit after Phase 2 telemetry shows whether users want a cross-workspace tray.
- **OQ2**: Mark animation — do we ship a Lottie file or a CSS/SVG-only animation? CSS-only keeps the bundle thin and respects `prefers-reduced-motion` naturally; Lottie gives the designer richer control. Default to CSS/SVG-only for v1; revisit if design pushes back.
- **OQ3**: Should agent-mode sessions appear in the existing `/runs` page as a new run type? Most likely yes for audit symmetry, but the surface there is workflow-run-shaped. Defer the UI integration to a follow-up; for now the dock is the only visualization and the audit trail is via `GET /v1/agent-mode/sessions`.
- **OQ4**: Naming of the workspace flag — `alfred.dock_enabled` vs `alfred.agent_mode_enabled`. The dock is technically usable as a read-only stream without agent-mode; if we keep that distinction the flag should reflect dock, not mode. Going with `alfred.dock_enabled` and a separate `alfred:agent-mode.run` permission for the action.
