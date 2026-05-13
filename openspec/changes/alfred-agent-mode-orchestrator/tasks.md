## 1. Data model and migrations

- [ ] 1.1 Add `db/migrations/alfred/000X_alfred_agent_session.sql` creating `alfred_agent_session` (`id`, `workspace_id`, `openspec_id`, `correlation_id`, `originator_principal`, `model_id`, `plan_revision`, `plan_json`, `frozen_autonomy_policy`, `status`, `started_at`, `paused_at`, `resumed_at`, `completed_at`, `aborted_reason`, `workflow_run_id`).
- [ ] 1.2 Add `alfred_agent_step` (`id`, `session_id`, `idx`, `kind`, `tool_id`, `workflow_id`, `agent_id`, `criticality`, `decision_id`, `status`, `started_at`, `completed_at`, `outcome`) with FK to `decision`.
- [ ] 1.3 Add indices: `alfred_agent_session(workspace_id, status, started_at desc)`, `alfred_agent_session(originator_principal)`, `alfred_agent_step(session_id, idx)`.
- [ ] 1.4 Add `dock_enabled` boolean column to the workspace settings JSON used by `alfred/autonomy_presets.py` (or its successor table); default `false`.
- [ ] 1.5 Author OpenFGA tuples for the new action classes `alfred:agent-mode.run` and `alfred:agent-mode.cancel`; update the OpenFGA bootstrap fixture and the `tests/phase0-policy.yaml` test fixture.
- [ ] 1.6 Add the new action classes to the default workspace autonomy presets in `services/alfred/alfred/autonomy_presets.py` (`full-autonomy → autonomous`, `staging-only → autonomous`, `manual-prod → requires_approval`).

## 2. Alfred service — agent-mode core (`services/alfred/alfred/agent_mode/`)

- [ ] 2.1 Create `agent_mode/__init__.py` and `agent_mode/models.py` with `AgentModeSession`, `AgentModeStep`, `PlanRevision` pydantic models.
- [ ] 2.2 Create `agent_mode/planner.py` that builds an initial plan from a committed OpenSpec (RAG + LLM via the existing `LiteLLMClient`); plan schema matches D2.
- [ ] 2.3 Create `agent_mode/executor.py` that iterates `plan.steps`, dispatches each through the same `PermissionsClient → PolicyClient → ApprovalsClient → tool_handler` sequence as `loop.run_intent`; reuses `Store.append_decision` and `AIObserver`.
- [ ] 2.4 Implement plan replan-on-recoverable-failure: append a new `PlanRevision` row, increment `plan_revision`, continue from the inserted fix step.
- [ ] 2.5 Implement HITL pause: when policy returns `requires_approval` / `requires_dual_control`, open the approval via `ApprovalsClient.request`, persist `status=paused_for_approval`, emit `alfred.agent_mode.paused_for_approval.v1`, return control.
- [ ] 2.6 Implement resume on approval grant: subscribe to approvals stream (existing pattern in `alfred/loop.py`), call `executor.resume(session_id)` on grant; abort on reject.
- [ ] 2.7 Implement cancellation: respect `alfred:agent-mode.cancel`, stop dispatching, let in-flight step finalize, transition to `aborted`.
- [ ] 2.8 Implement budget-aware pause: probe LiteLLM tenant budget before each LLM-bound step; transition to `paused_for_budget` and emit `alfred.agent_mode.paused_for_budget.v1` on `over_budget`.
- [ ] 2.9 Implement frozen-preset enforcement: snapshot `autonomy_policy` on session row at start; all per-step ceilings consult the frozen copy.
- [ ] 2.10 Implement workflow step kind: dispatch `forge.reference.intent-to-deploy@1` via workflow-runtime API, persist `workflow_run_id`, consume the run's step events into the session's step rows.

## 3. Alfred service — API surface (`services/alfred/alfred/app.py` + new routers)

- [ ] 3.1 Add `agent_mode/router.py` mounting under `/v1/agent-mode`; gate registration on `ALFRED_AGENT_MODE_ENABLED`.
- [ ] 3.2 Implement `POST /v1/agent-mode/sessions` (start) with full permission/policy stack and event emission.
- [ ] 3.3 Implement `GET /v1/agent-mode/sessions/{id}` (state) and `GET /v1/agent-mode/sessions/{id}/decisions` (decision list).
- [ ] 3.4 Implement `GET /v1/agent-mode/sessions/{id}/stream` (SSE) with `Last-Event-ID` resume, 15s heartbeat, replay from `decision` + `alfred_agent_step` tables.
- [ ] 3.5 Implement `POST /v1/agent-mode/sessions/{id}/messages` (follow-up intent) with frozen-preset re-evaluation and `autonomy.override.rejected.v1` emission on ceiling breach.
- [ ] 3.6 Implement `POST /v1/agent-mode/sessions/{id}/cancel` honoring `alfred:agent-mode.cancel`.
- [ ] 3.7 Emit CloudEvents on every state transition (`session_started`, `step_started`, `step_completed`, `plan_revised`, `paused_for_approval`, `paused_for_budget`, `resumed`, `completed`, `aborted`, `failed`); reuse existing CloudEvents helpers.
- [ ] 3.8 Update OpenAPI: extend `contracts/openapi/registry.yaml` (or the appropriate Alfred contract file) with the new surface; regenerate clients.

## 4. Alfred service — tests

- [ ] 4.1 Unit tests for `planner` over fixture OpenSpecs (happy path, security-impacting, prod-tagged).
- [ ] 4.2 Unit tests for `executor` covering: autonomous step success, policy deny, requires_approval pause/resume, requires_approval reject/abort, recoverable-failure replan, budget pause.
- [ ] 4.3 SSE integration test asserting replay-then-live behavior with `Last-Event-ID`.
- [ ] 4.4 Follow-up tests: in-ceiling follow-up accepted, ceiling-breaching follow-up rejected with `autonomy.override.rejected.v1`.
- [ ] 4.5 Frozen-preset test: admin tightens preset mid-session, in-flight session continues under original ceilings, next session picks up new ceilings.
- [ ] 4.6 End-to-end test (`tests/test_agent_mode_e2e.py`) that drives a canned OpenSpec through scaffold → PR → CI-green → HITL → deploy with mocked workflow-runtime and asserts the milestone event ordering from the spec.

## 5. Portal — dock component (`portal/src/components/shell/`)

- [ ] 5.1 Add `AlfredDock.tsx` with launcher button, slide-in panel, plan checklist, transcript, follow-up composer, artifact links; tokens from `--alfred-*` only.
- [ ] 5.2 Add `AlfredSessionProvider.tsx` exposing `useAlfredSession()` (active session id, status, SSE handle); mount inside `PortalShell.tsx` alongside `ToastRail` and `CommandPalette`.
- [ ] 5.3 Wire keyboard summon (`Alt+A` / `⌥A`) into the existing shell hotkey provider with palette-coexistence behavior.
- [ ] 5.4 Implement focus trap (use existing primitive if available; else lightweight tabbable cycle) with `Esc` restoring focus to launcher.
- [ ] 5.5 Implement responsive behavior: <1024px widens panel and hides launcher status text; mutually exclusive with command palette.
- [ ] 5.6 Implement permission gating: hide launcher when `alfred_invoke=false`; disable `Start agent-mode` when `alfred_agent_mode_run=false`.
- [ ] 5.7 Add portal SSE proxy at `portal/src/app/api/alfred/sessions/[id]/stream/route.ts` that forwards the Alfred gateway token server-side.
- [ ] 5.8 Add `portal/src/app/api/alfred/sessions/route.ts` (POST start) and `portal/src/app/api/alfred/sessions/[id]/messages/route.ts` (POST follow-up) and `…/cancel/route.ts`.
- [ ] 5.9 Extend `GET /api/permissions/me` (and `/api/sidebar/counts` as needed) to return `alfred_invoke` and `alfred_agent_mode_run` for the active workspace.
- [ ] 5.10 Add dock copy to `portal/src/i18n/dictionary.ts` under the `alfred.dock.*` key prefix in ES and EN, conforming to the persona lint rules.
- [ ] 5.11 Emit portal telemetry events `portal.alfred.dock_opened.v1` / `dock_closed` / `dock_session_started` / `dock_follow_up_sent` / `dock_navigated_to_artifact`.

## 6. Portal — design system surface

- [ ] 6.1 Add `--alfred-*` tokens to `portal/src/app/globals.css` for both `:root` and `[data-theme="dark"]`; verify AA contrast on `--alfred-ink` over `--alfred-paper` in both themes.
- [ ] 6.2 Add Alfred marks to the shared mark registry (or create one if absent) with stable ids `alfred-mark`, `alfred-mark-mono`, `alfred-mark-working`.
- [ ] 6.3 Implement the working-state animation as CSS-only on the `alfred-mark-working` SVG with `prefers-reduced-motion` fallback to a still mark.
- [ ] 6.4 Author the persona-lint script `scripts/lint-alfred-copy.mjs` that fails on emoji, exclamation marks, first-person plural and missing ES/EN pairs over `alfred.*` keys; wire into CI.

## 7. Design folder — Alfred identity

- [ ] 7.1 Create `design/alfred-identity/marks/alfred-mark.svg`, `alfred-mark-mono.svg`, `alfred-mark-working.svg` (24×24 viewBox, ember/bowtie motif consistent with the Forge `F.` mark).
- [ ] 7.2 Write `design/alfred-identity/PERSONA.md` (voice rules, ES/EN do/don't with ≥3 examples per rule, criticality glyph table).
- [ ] 7.3 Write `design/alfred-identity/MOTION.md` (dock-in/out easing, working-cycle, reduced-motion fallbacks).
- [ ] 7.4 Write `design/alfred-identity/DO_DONT.md` (≥6 do/don't pairs each with inline SVG illustrations).
- [ ] 7.5 Add `Makefile` target `design-export` that runs `svgo` over `design/alfred-identity/marks/*.svg`, writes optimized copies to `portal/public/alfred-mark*.svg`, and regenerates the Alfred section of the standalone notebook HTML.
- [ ] 7.6 Regenerate `design/Forge Brand Notebook _standalone_.html` so it contains the new `Alfred` section with inlined marks, tokens, persona summary and do/don't thumbnails; verify size remains <3 MB and the file stays single-file offline.
- [ ] 7.7 Add a CI check that fails if `design/alfred-identity/` is modified without rerunning `make design-export` (compare a checksum of the section against the notebook).

## 8. Policy and audit

- [ ] 8.1 Add OPA rules for `alfred:agent-mode.run` and `alfred:agent-mode.cancel` to the platform bundles; cover with rego unit tests in `contracts/openfga/tests/`.
- [ ] 8.2 Register the new CloudEvents (`alfred.agent_mode.*`) in the event schema registry and add audit ingestion paths in `per-asset-observability` so per-session cost/latency/HITL-pause metrics roll up.
- [ ] 8.3 Add a session-level dashboard tile (cost per session p95, success rate, HITL-pause rate) to the existing per-asset observability dashboard.

## 9. Reference flow integration

- [ ] 9.1 Update `forge.reference.intent-to-deploy@1` workflow metadata (description + tags) to note the Alfred agent-mode default operator path; no contract change.
- [ ] 9.2 Update the smoke test in CI to assert the interleaved milestone sequence under the agent-mode path (per spec scenario).
- [ ] 9.3 Update `make demo-intent-to-deploy`: default path drives through `POST /v1/agent-mode/sessions`; `NO_AGENT_MODE=1` falls back to direct workflow trigger; report JSON gains `session_id`.

## 10. Documentation and rollout

- [ ] 10.1 Author `docs/runbooks/alfred-agent-mode.md` covering: start a session, inspect the dock, approve/reject, resume after pause, cancel, common failure modes, on-call mitigations.
- [ ] 10.2 Update `docs/runbooks/intent-to-deploy-demo.md` to cover both agent-mode-default and `NO_AGENT_MODE=1` paths, with screenshots of the dock at each phase.
- [ ] 10.3 Add a portal admin surface for the per-workspace `alfred.dock_enabled` toggle (reuse the autonomy-preset admin page).
- [ ] 10.4 Ship a release note describing Phase 0 → Phase 3 rollout (per the design migration plan) and the `ALFRED_AGENT_MODE_ENABLED` env var + workspace flag semantics.
- [ ] 10.5 Verify rollback by flipping `ALFRED_AGENT_MODE_ENABLED=false` in a staging environment and asserting: launcher disappears, new session calls return 503, existing sessions complete and remain queryable.
