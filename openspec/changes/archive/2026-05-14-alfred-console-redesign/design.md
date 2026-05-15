## Context

The Alfred Console is the primary way platform users interact with Alfred. Today it ships a single surface that mixes two very different user personas in the same shell:

- **Developers** want fast keyboard-driven access via slash commands (`/openspec new`, `/forge runtime list`), short identifiers visible in transcripts (`spec-7b3`, `app-1`), and direct access to JSON payloads.
- **Non-technical operators / business owners** want to express intent in plain language ("I want a time-off tracker for HR") and let Alfred do the rest. They don't know what an `openspec_id` is and shouldn't have to.

Two extra problems make the current state worse:

1. **Spec duplication**. Users routinely re-describe an intent that has already been captured (or even already implemented) and Alfred dutifully creates a new spec. We have no dedup pass between intent capture and persistence, and the resulting Registry/OpenSpec backbone gets polluted.
2. **Brand alignment**. The legacy `/openspec` command name reflects an internal tool, not the public-facing Forge brand. We want a single platform-agnostic `/forge` command.

We can solve all of this in one console redesign because they share a common root: the console is now anchored on an App (post `app-first-class-entity`), and we have a stable place to attach role-based defaults, a dedup pass and a renamed command.

Constraints:
- Backwards compatibility for `/openspec` for two minor versions. Removing it sooner would break every CI hook and docs page in tenant repos.
- The Friendly view MUST not require any new backend capability beyond the dedup endpoint; everything else (intent capture, agent mode, approvals) is reused.
- We must not regress the developer flow. Power users have muscle memory for the slash-command surface and we keep it intact in Advanced view.
- The platform's persona content rules (`PERSONA.md`) are already enforced for Alfred-authored messages; the Friendly view inherits and emphasises them.

Stakeholders: Portal team (console UI), Alfred team (backend), DX/CLI team (command rename), every pilot tenant (rollout). Workspace admins may want to override the role-based default (e.g., a tenant where all members should land on Friendly regardless of role).

## Goals / Non-Goals

**Goals:**
- Ship two views — Friendly (default for non-devs) and Advanced (default for devs) — sharing the same backend.
- Make the App switch obvious in both views; in Friendly it's a friendly App picker, in Advanced it's an App-aware slash-command context.
- Run a RAG dedup pass on every new intent and offer Extend / Create-new / Implement actions based on the match.
- Rename `/openspec` to `/forge` with a deprecation alias for two minor versions.
- Preserve all current developer affordances in the Advanced view.

**Non-Goals:**
- Re-designing the Alfred dock (the floating bottom-right launcher). The dock is reused as-is and gets a small copy refresh; deeper dock changes are out of scope.
- A net-new agent (no new specialized agents introduced here).
- Cross-tenant spec deduplication. The dedup query is scoped to the App (or, when no App is selected yet, the Workspace).
- Localisation beyond ES/EN. Both views ship in the existing two-language matrix.

## Decisions

### Decision 1 — Two views, one backend; the toggle lives in user settings

**Choice**: Friendly and Advanced are two React routes/sub-shells consuming the same backend (`POST /v1/intent/start`, `POST /v1/intent/answer`, agent-mode session APIs). The toggle is a per-user preference persisted server-side; an opt-in session-level override is available via a top-of-console switch. Role-based defaults apply on first sign-in (Friendly for `workspace.member`, Advanced for `workspace.developer` and above).

**Why**: keeping the backend single avoids a fork in the API surface, the audit shape and the agent-mode state machine. The toggle is the right granularity — it's a UI preference, not a permission. The role-based default gives sensible behaviour out of the box without forcing every user to pick a side on day one.

**Alternatives considered**:
- *Single adaptive view that hides developer affordances based on role*. Rejected — adaptive UIs are confusing ("where did the slash command go?") and conflate UX preferences with permissions.
- *Two backends*. Massively duplicates work and creates parity bugs.

### Decision 2 — Friendly view is anchored on three cards: Nueva App / Mejorar / Operar

**Choice**: the Friendly landing is three big cards. Each one routes into a specialised conversation flow with Alfred (`new_app`, `improve_app`, `operate_app`). The conversation is plain-language; identifiers are translated to friendly labels at render time.

**Why**: three is a comfortable number for a clean landing and matches the three operational jobs we hear most. Each card carries a one-liner about what it does ("Crea una app nueva desde cero" / "Improve an app you already have" / "Deploy, monitor or troubleshoot an existing app"). Selecting a card narrows Alfred's prompt template and pre-filters dedup candidates.

**Alternatives considered**:
- *Free-form intent box first, cards as fallback*. Rejected — non-technical users prefer guided affordances over a blank text box at the entry point.
- *Four cards (add "Settings" or "Help")*. Rejected — those belong in the user menu, not on the conversational entry point.

### Decision 3 — Dedup is a mandatory pass with a hard threshold

**Choice**: every intent (in either view) flows through `POST /v1/intent/match` *before* a draft spec is created. The endpoint runs a Milvus retrieval over the scoped corpus (App > Workspace) and returns the top-K candidates with cosine scores. If the top hit has `score >= 0.80`, the user MUST see the match dialog before any draft is persisted. Below 0.80, Alfred proceeds straight to draft creation. The 0.80 threshold is configurable per tenant via `tenant.spec_match.threshold` with a hard floor of 0.65; below 0.65 the system never surfaces a match.

**Why**: a hard threshold gives the dedup affordance a stable UX. Surfacing low-score matches would erode trust ("Alfred keeps proposing irrelevant specs"). The Milvus-backed retrieval already exists for Alfred's RAG; this is one extra query at intent capture, sub-100ms p95 with the current index sizes.

**Alternatives considered**:
- *Always show the top-K regardless of score*. Rejected — surfaces noise.
- *Two thresholds (low / high) with different copy*. Premature complexity; we can add a "see other similar specs" link below the dialog without two-tier copy.

### Decision 4 — Direct-to-architect on a committed match

**Choice**: when the matched spec is already `lifecycle_state in {approved, committed}`, the match dialog promotes the "Implementar" action to the primary CTA and demotes "Extender" / "Crear nuevo". Clicking "Implementar" calls `POST /v1/agent-mode/sessions {openspec_id, start_step="architect"}`, bypassing the wizard and putting Alfred directly into the architect step of the SDLC workflow. The agent-mode session emits the usual events.

**Why**: a committed spec is, by definition, ready to be implemented; the friction of "use the wizard to re-capture intent for an already-captured spec" is pure waste. The `start_step` hint slots cleanly into the agent-mode state machine without inventing a parallel API.

**Risk**: `start_step=architect` skips the dedup pass for that specific session start — that's the point, since dedup already happened. Audit captures the bypass with a `start_step` field on `alfred.agent_mode.session_started.v1`.

### Decision 5 — `/openspec` → `/forge` with a two-minor-version deprecation window

**Choice**: every CLI / palette / portal reference to `/openspec` is renamed to `/forge`. The `/openspec` alias keeps working for two minor releases after this change ships, emitting:
- a yellow deprecation toast in the Portal palette,
- a `WARNING` line in the CLI output,
- an `alfred.command.deprecated_alias.v1` audit event with the caller and the original input.

In the third minor release after this change, the alias is removed; calling `/openspec` returns `404 deprecated_command_removed` with a hint to use `/forge`.

**Why**: the platform team has explicit signal from tenants that they have CI hooks and docs referencing `/openspec`. Two minor versions ≈ ~3–4 months in the current cadence, which gives tenants time to update without rushing. The audit event lets us track adoption and pick a confident removal release.

**Alternatives considered**:
- *Hard cutover*. Rejected — too disruptive.
- *Permanent alias*. Rejected — leaves the brand-misaligned name in `git log` forever.

### Decision 6 — Friendly view never shows raw IDs

**Choice**: the Friendly view renders every entity by its human label. Internally, the React layer carries the IDs for API calls but never paints them. Error messages from OpenFGA / policy / backbone are translated through a lookup table to user-friendly copy (`403 missing_app_editor` → "You don't have permission to edit this app. Ask <owner_name>."). Unknown errors fall back to a single generic message + a "Show technical details" disclosure.

**Why**: hiding IDs is the single biggest UX win for non-tech users. The disclosure escape hatch keeps the surface debuggable when something goes wrong.

### Decision 7 — Role-based default on first sign-in, persisted thereafter

**Choice**: on first sign-in, the platform picks Friendly for users with `workspace.member` (or no workspace role beyond `workspace.viewer`), and Advanced for `workspace.developer` and above. The choice is persisted as `user.console_view_preference`. Subsequent sign-ins respect the persisted preference even if the role changes. Workspaces MAY enforce a tenant default via `tenant.console_default_view` (overriding role-based logic on first sign-in only).

**Why**: roles are a coarse but useful proxy at day zero. After that, users are best at picking their own preference. Tenant override is a Phase-6 ask but the wiring is trivial.

## Risks / Trade-offs

- **[Risk] Dedup misses obvious duplicates** because the threshold is too high. → Mitigation: tenant-tunable threshold with a 0.65 floor; track `match_dismissed` and `match_found` rates; calibrate after the first month of usage.
- **[Risk] Dedup surfaces false positives** that frustrate users. → Mitigation: include a "no, this is not the same" button in the match dialog that feeds back into the retrieval relevance training data (annotated for the next retrieval re-rank).
- **[Risk] Users stuck in Friendly mode hit a feature they need that's only in Advanced**. → Mitigation: every Friendly screen has a small "Switch to developer mode" link in the footer; the link is one click.
- **[Risk] `/openspec` removal breaks tenant CI scripts**. → Mitigation: long deprecation window, explicit audit-event-based usage tracking, an opt-in `force_keep_openspec_alias` tenant flag if a tenant cannot migrate in time (manual exception, not exposed in self-serve).
- **[Trade-off] Two views increase the test matrix** for the Portal. → Acceptable: most components are shared; only the landing + view-specific transcript styling differs. Playwright tests cover both views for each smoke flow.
- **[Trade-off] Match dialog adds one round-trip before intent draft**. → Acceptable: sub-100ms p95 for Milvus retrieval; the UX gain dwarfs the latency.

## Migration Plan

1. **M0 — Ship code dark-launched**: Friendly view, dedup endpoint, `/forge` alias all behind `forge.alfred_console_v2.enabled` (per-tenant).
2. **M1 — Pilot tenant enable**: turn on the flag for the platform tenant and two pilot tenants; gather telemetry on view toggle rates, match-found rates, false-positive reports.
3. **M2 — Adjust threshold**: re-calibrate the 0.80 threshold based on M1 data (expected to land somewhere in `[0.78, 0.85]`).
4. **M3 — Global enable**: flip the flag on for all tenants; deprecation toast for `/openspec` activates.
5. **M4 — Next minor release**: deprecation continues; usage of `/openspec` reported in tenant onboarding emails.
6. **M5 — Third minor release**: remove `/openspec` alias; CLI prints "Unknown command. Did you mean `/forge`?".

**Rollback**: per-tenant feature flag off restores the legacy single-view console; the dedup endpoint becomes a no-op for that tenant.

## Open Questions

- Should the "Operar" card start a conversation or open the existing observability/runs view directly? Recommendation: start a conversation, since users may want to *describe* an operational issue rather than click through dashboards. Confirm with two pilot users.
- The dedup retrieval today reads from Milvus; do we need to invalidate the index when a spec is purged by the orphan-deletion job from `app-first-class-entity`? Yes — the job SHALL emit `spec.purged.v1` which Alfred's indexer already consumes.
- Should Workspace owners be able to disable the Advanced view tenant-wide? Recommendation: no for Phase 5; revisit if requested.
