## 1. Friendly view shell

- [x] 1.1 Add the `/alfred` route with `?view=friendly|advanced` query handling and per-user resolution from `user.console_view_preference`
- [x] 1.2 Build the Friendly landing component with the three cards (Nueva App / Mejorar / Operar) in ES/EN
- [x] 1.3 Build the conversation panel scoped to each card's prompt template; persist scope on the dialogue context
- [x] 1.4 Implement the friendly label resolver (Apps, specs, workspaces, runs) with graceful fallback to italic placeholder
- [x] 1.5 Implement the friendly App switcher / static label header based on visible-App count
- [x] 1.6 Map known backend error codes to friendly copy via a lookup table; ship a generic fallback with disclosure
- [x] 1.7 Translations: ES default (es-CO) and EN (en-US) for all new copy
- [x] 1.8 Playwright e2e for the three card flows (golden path + permission-denied path)

## 2. Advanced view preservation + App scoping

- [x] 2.1 Move the existing console into the Advanced view; ensure no behaviour regression
- [x] 2.2 Add an App picker to the Advanced view top bar that scopes every subsequent slash command
- [x] 2.3 Wire `view=advanced` into every Alfred dialogue call originating from this view
- [x] 2.4 Update the developer keyboard shortcuts cheat sheet

## 3. View toggle + role-based default

- [x] 3.1 Add `user.console_view_preference` to the user-prefs schema; default null (resolved at first sign-in)
- [x] 3.2 Add `tenant.console_default_view` to tenant config
- [x] 3.3 Implement first-sign-in resolver: tenant default → role-based → fallback Friendly; persist the result
- [x] 3.4 Build the account-menu toggle and the session-level switch with "Save as my default" affordance
- [x] 3.5 Emit `alfred.console.view_toggled.v1` on every toggle (persistent and session)
- [x] 3.6 Test: first-time member, first-time developer, tenant override, toggle persistence, session-level only

## 4. Dedup endpoint and indexing

- [x] 4.1 Implement `POST /v1/intent/match` in the Alfred service, scoped to App (`app_id` set) or Workspace
- [x] 4.2 Wire to Milvus retrieval with k=5 default; return scores, lifecycle states and short summaries
- [x] 4.3 Add `tenant.spec_match.threshold` to tenant config with floor=0.65, default=0.80; reject writes below floor with `422 threshold_below_floor`
- [x] 4.4 Update the Alfred indexer to react to `spec.purged.v1` (remove from index), `spec.reparented.v1` (update metadata), `intent.committed.v1` (index)
- [x] 4.5 Update OpenAPI contract for the new endpoint
- [x] 4.6 Load tests: p95 < 100ms for retrieval against the pilot tenant corpus size

## 5. Dialogue API + view marker

- [x] 5.1 Update `POST /v1/intent/start` to accept `view`, `bypass_match`, `resume_spec_id`; default `bypass_match=false`
- [x] 5.2 Implement the dedup pass on intent start; return `spec_match` block when above threshold; no draft created in that case
- [x] 5.3 Update `POST /v1/intent/answer` to carry `view` through every turn; propagate to the LLM prompt and the audit event
- [x] 5.4 Wire the friendly persona rendering (label-only) when `view=friendly`
- [x] 5.5 Emit `alfred.intent.match_found.v1` and `alfred.intent.match_dismissed.v1`

## 6. Match dialog UI

- [x] 6.1 Build the match dialog component (renderable by Friendly view, Advanced view, wizard and dock)
- [x] 6.2 Implement the action ordering rule (Implementar primary when match is committed; Extender primary otherwise)
- [x] 6.3 Wire the Implementar action to `POST /v1/agent-mode/sessions` with `start_step=architect`; route to the session detail page
- [x] 6.4 Wire Extender to `POST /v1/intent/start` with `resume_spec_id`
- [x] 6.5 Wire Crear nuevo to `POST /v1/intent/start` with `bypass_match=true`
- [x] 6.6 Wire the "No, esto no es lo mismo" feedback button to emit `alfred.intent.match_dismissed.v1`
- [x] 6.7 Visual regression tests in both light and dark themes; ES and EN

## 7. Agent-mode session `start_step`

- [x] 7.1 Add `start_step` field to `POST /v1/agent-mode/sessions` request schema; validate against allowed values
- [x] 7.2 Implement spec-lifecycle gate: `start_step=architect` requires `spec.lifecycle_state in {approved, committed}` else `409 spec_not_ready_for_architect`
- [x] 7.3 Update the plan builder to produce a plan whose step 0 is the requested `start_step`
- [x] 7.4 Emit `alfred.agent_mode.session_started.v1` carrying `start_step`
- [x] 7.5 Test: discovery (default), architect (committed spec), unknown step, not-ready spec

## 8. Command rename `/openspec` → `/forge`

- [x] 8.1 Register `/forge` as the canonical command in the palette, the CLI and the docs
- [x] 8.2 Keep `/openspec` as an alias for two minor versions; map it to `/forge` at the command-router layer
- [x] 8.3 Emit `alfred.command.deprecated_alias.v1` on every `/openspec` invocation
- [x] 8.4 Render the yellow deprecation toast in the Portal palette
- [x] 8.5 Print the CLI deprecation warning to stderr
- [x] 8.6 Update docs: CLAUDE.md, README, runbooks, every code example referencing `/openspec`
- [x] 8.7 Schedule the third-minor-version removal task with the release calendar
- [x] 8.8 Hide both commands from the Friendly view palette; ensure `/` doesn't trigger the palette in Friendly view

## 9. Telemetry, dashboards and rollout

- [x] 9.1 Add `forge.alfred_console_v2.enabled` feature flag (per-tenant), default `false`
- [x] 9.2 Dashboards: Friendly vs Advanced ratio, match-found rate, match dismissed rate, false-positive ratio, `/openspec` alias usage
- [x] 9.3 SLOs: dedup retrieval p95 < 100ms; Friendly view first-paint < 1s
- [x] 9.4 Runbook: tenant rollout sequence (platform → 2 pilot tenants → global) with the threshold-calibration step between pilot and global
- [x] 9.5 Runbook: tenant exception for `force_keep_openspec_alias` if a tenant needs more time

## 10. Documentation

- [x] 10.1 Update CLAUDE.md repository-level guidance with `/forge`
- [x] 10.2 Update the user-facing docs site with Friendly view walkthrough and Advanced view reference
- [x] 10.3 Update the platform-enablement doc with the role-based default rules and the tenant override
- [x] 10.4 Update PERSONA.md if any persona rule needed clarification for label-only rendering
