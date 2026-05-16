## Context

Three things are already true on `main`:

1. **The Design System catalog ships.** `design-system-catalog` archived 2026-05-13 with four built-in templates (`desing-system-1..4`), the `ds-forge-default` alias, `GET /v1/design-systems` listing endpoint in the registry service, swap PR mechanism, per-component overrides, sanity validator, and CloudEvents. `manifest.screenshots.{light,dark}` and `manifest.use_case` are required fields on every entry.
2. **The /alfred/wizard picker exists.** `portal/src/app/alfred/wizard/page.tsx` lines 137-166 (`selectDesignSystem` server action) and 404-487 (the render block) render the catalog as a radio-button grid when the user comes via the "create new App" branch. The wizard creates the App via `POST /v1/workspaces/{ws}/apps` first, then `PATCH`es `design_system_ref` after the user picks. The `intent-capture-wizard` spec (lines 179-222) mandates this behavior.
3. **The App entity has `design_system_ref`.** `services/application/internal/application/types.go:99` defines `DesignSystemRef` on the App aggregate; the swap service at `design_system_service.go` updates it via the swap PR mechanism; `application-entity` spec describes the field as optional.

Three things are **not** true on `main`:

1. **`portal/src/components/alfred/FriendlyView.tsx` contains zero references to `design_system`.** A non-technical user using the Friendly view's "Nueva App" card — the intended entry point for intent-to-infrastructure per the umbrella — never sees the catalog. The picker only exists for power users who explicitly navigate to `/alfred/wizard`.
2. **`intent-to-infrastructure-reference-flow` spec is silent on `design_system_ref` propagation.** The workflow contractually does not load the ref, does not propagate it to `sdlc-design` skill invocations, and does not emit it on `sdlc.ui_blueprint.proposed.v1` or `sdlc.component_stubs.committed.v1`. When F1b (umbrella) makes those skills LLM-driven, the interface they receive is undefined.
3. **`POST /v1/workspaces/{ws}/apps` does not accept `design_system_ref` in the create body.** The wizard works around this with POST-then-PATCH. Beyond the round-trip cost, this creates a brief window where the App exists with the alias-resolved default before the user's actual choice persists — observable to anyone subscribed to `app.created.v1` and to anyone reading the audit log out of order.

The user's design conversation (referenced as "the Lovable image") shows the picker as a prominent, card-based, in-conversation step with an explicit Skip button. The screenshot's previews look LLM-generated per intent (v0/Lovable pattern), but the user has explicitly scoped this change to **catalog screenshots only**; per-intent LLM previews are out of scope and deferred to a follow-up after F0b lands.

## Goals / Non-Goals

**Goals:**

- A non-technical user on the Friendly view's "Nueva App" card sees the catalog picker before workflow dispatch, picks consciously or skips, and the choice flows end-to-end into the App and the workflow execution.
- The `intent-to-infrastructure-reference-flow` spec contractually requires the workflow to load and propagate `design_system_ref` so F1b lands against a stable interface (not a moving target).
- The picker UX in both surfaces (Friendly view and /wizard) shares one React component and one set of i18n keys — no UX drift.
- The atomic `POST` variant eliminates the POST-then-PATCH window and the spurious `app.updated.v1` event the current wizard generates.
- Skip is distinguishable from "picked the default" in audit, so we can later measure catalog discoverability and decide whether to make the step blocking.

**Non-Goals:**

- AI-generated per-intent previews. Static catalog screenshots only.
- Adding new templates to the catalog (the 4 existing templates stay).
- Surfacing tenant-published custom Design Systems.
- Per-component override UI at intent capture.
- Post-deploy swap UI in the Portal.
- Any change to Alfred's LLM call path (that's `alfred-litellm-header-injection` and `alfred-via-model-gateway`).
- Implementing LLM-driven `sdlc-design` skills (that's F1b; this change only locks the contract those skills will fulfill).

## Decisions

### D1 — Picker lives in the Friendly view's "Nueva App" conversation, after intent capture

**Decision.** The picker step renders **after** the user has typed their intent into the Nueva App card's conversation and **before** the workflow dispatch confirmation. The Friendly view itself manages step ordering; the picker is one of the conversation's panels (not a separate route).

**Why.** The user's screenshot positions the picker as part of the in-flight conversation — the intent is already visible on the left, the picker on the right, and the conversation continues after. Rendering the picker before intent capture would be jarring (the user hasn't said what they want yet); rendering it after dispatch would be pointless (the workflow has already started without the ref). Mid-conversation matches the screenshot and the user's mental model.

**Alternatives considered:**

- Picker as a separate route the Friendly view redirects to. Rejected: breaks the conversational flow and loses chat context.
- Picker only at App creation time, separate from intent capture. Rejected: forces the user to think about design before they've articulated the intent — wrong order of cognitive load.

### D2 — Share one React component between Friendly view and /wizard route

**Decision.** Extract the picker into `portal/src/components/alfred/DesignSystemPicker.tsx`. Both `portal/src/components/alfred/FriendlyView.tsx` and `portal/src/app/alfred/wizard/page.tsx` consume it. The component is presentational (catalog data + selection state are props/callbacks); the host owns API calls.

**Why.** Two divergent picker UIs would be a maintenance trap. One component, one i18n key set, one screenshot grid layout. Hosts differ on how the choice is committed: Friendly view holds it in conversation state and includes it in the eventual `POST /v1/workspaces/{ws}/apps`; /wizard route does the same after the refactor. Presentational/host split keeps the component portable.

**Alternatives considered:**

- Duplicate the picker JSX in both places. Rejected: certain drift.
- Hoist API call logic into the component. Rejected: ties the component to one transport assumption and complicates testing.

### D3 — Skip is a first-class action with its own event, not "picked the default"

**Decision.** The picker exposes an explicit Skip button (Saltar / Skip) distinct from the "Continue with current selection" action. When the user clicks Skip:

- The App is created with `design_system_ref=ds-forge-default` (alias resolution at create time, same as omitting the field).
- The App record carries `design_system_chosen_explicitly=false` in the audit log.
- The service emits `app.design_system.user_skipped.v1` with `{app_id, workspace_id, tenant_id, principal, correlation_id}`.

When the user picks any card explicitly (including the default) and clicks Continue, the App is created with `design_system_chosen_explicitly=true`. No skip event fires.

**Why.** Distinguishing skip from picked-default lets us measure the catalog's discoverability and decide later whether to make the step blocking. Today's wizard conflates the two (default-checked radio + no skip button), which means we can't tell whether users see the picker and shrug, or never see it.

**Alternatives considered:**

- No skip; the user must pick. Rejected: contradicts the user's explicit choice ("Opcional con default") and adds friction to a 20-40 min flow.
- Skip rendered as a small "no thanks" link instead of a button. Rejected: the screenshot shows it as a prominent action. Underweighting it would tell users skip is the wrong choice.

### D4 — Atomic POST with optional `design_system_ref` in body

**Decision.** Extend `POST /v1/workspaces/{ws}/apps` body schema:

```json
{
  "name": "...",
  "slug": "...",
  "description": "...",
  "owners": ["..."],
  "design_system_ref": "desing-system-3@2.0.0",       // NEW: optional
  "design_system_chosen_explicitly": true              // NEW: optional, defaults false if absent
}
```

When `design_system_ref` is supplied, validate it resolves to an `approved` Design System visible to the App's tenant (same validation the swap endpoint already performs). When omitted, resolve `ds-forge-default` server-side as today. Both paths emit a single `app.created.v1` event with the resolved ref. When `design_system_chosen_explicitly=false` and the picker host signals skip, additionally emit `app.design_system.user_skipped.v1`.

**Why.** Eliminates the POST-then-PATCH window (no spurious `app.updated.v1`, no observable inconsistent state). Keeps the field optional so existing callers (CLI, other portal flows) work unchanged. Validation reuses existing swap-validation code path.

**Alternatives considered:**

- Add a separate `POST /v1/workspaces/{ws}/apps/with-design-system` endpoint. Rejected: needless surface duplication.
- Make the field required. Rejected: breaks every existing caller and contradicts D3 (skip is valid).
- Keep POST-then-PATCH and just add Skip semantics. Rejected: the picker UX wins (single confirm), audit cleanliness (one event), and observability (no inconsistent window) all argue for atomic.

### D5 — Workflow propagation contract is interface-only, not implementation

**Decision.** `forge.reference.intent-to-infrastructure@1` SHALL load the App snapshot with a resolved `design_system_ref` at dispatch time, pass it to every `sdlc-design` skill invocation (`generate-ui-blueprint`, `generate-component-stubs`, `accessibility-audit`), and include it in `sdlc.ui_blueprint.proposed.v1` and `sdlc.component_stubs.committed.v1` event payloads under a new top-level field `design_system_ref`.

This change implements the **plumbing** (workflow loads the ref, passes it, emits it) but does NOT change what the skills do with it. Today the skills are stubs (per umbrella F1b); they receive the ref and ignore it. When F1b makes them LLM-driven, they consume the ref to pick the right tokens. The interface they will see is locked now.

**Why.** Wiring the interface now is cheap (one Go struct field + one workflow step's invocation args + two event payload additions). Wiring it during F1b would conflate two concerns (interface design + implementation) and slow F1b down. Plus, the propagation lets us write the E2E test today against the stub skills: "ref selected in picker → ref present on workflow events". That E2E becomes the regression test F1b inherits.

**Alternatives considered:**

- Defer the propagation contract to F1b. Rejected: F1b's scope is the LLM realization; coupling it to interface design adds risk. Also blocks E2E testing of this change.
- Use a workflow-level variable instead of skill invocation args. Rejected: workflow-level state is harder to reason about and harder to validate via event payloads.

### D6 — Reuse Sequence: alfred-litellm-header-injection lands first

**Decision.** This change MUST land after `alfred-litellm-header-injection` is on `main`. No code dependency; only ordering. CI gate: the change's PR description SHALL reference the predecessor merge commit.

**Why.** Friendly view intent capture leans on Alfred's reasoning loop (Alfred drives the conversation in the Nueva App card). Any UX work shipped on top of the broken cost-attribution path would mean every intent submitted via the new picker contributes mis-attributed LiteLLM spend until F0b/F1a fully migrates Alfred to the gateway SDK. Waiting 1-2 days for header injection to land sidesteps the issue.

**Alternatives considered:**

- Ship in parallel; let the predecessor merge first by chance. Rejected: too easy to land out of order in a busy week.
- Ship before the predecessor. Rejected: same as parallel, plus we'd be the change that demonstrably worsens cost attribution.

## Risks / Trade-offs

- **Risk: the screenshot grid in the Friendly view's narrow conversation panel looks cramped.**
  Mitigation: Single-column card layout on narrow widths (<= 720px), two-column at the screenshot's referenced width. Screenshots scale to 100% of card width; max card width is 360px. Validate in dev tools against breakpoints before shipping.

- **Risk: users on slow connections see a flash of "Could not load catalog" while `GET /v1/design-systems` resolves.**
  Mitigation: Skeleton loader for the catalog cards; the existing "catalog empty" fallback (lines 418-422 of the current wizard) stays as the error state but only after a 5s timeout, not immediately.

- **Risk: `POST` validation rejecting a stale `design_system_ref` (e.g., a deprecated version) after the user picked it.**
  Mitigation: The catalog response from `GET /v1/design-systems` only returns `lifecycle_state=approved` entries today. The picker's selectable set IS the validation set. Edge case where the entry is deprecated between catalog fetch and POST: catch the 422, re-fetch catalog, re-render the picker with an error toast. Should be rare given the catalog is static for built-ins and tenant-published changes are slow.

- **Risk: emitting `app.design_system.user_skipped.v1` floods the event bus if every user skips.**
  Mitigation: One event per App creation that skips. The event volume is bounded by App creation rate, which is naturally low (tens per workspace per quarter at MVP). Not a concern.

- **Risk: existing `/alfred/wizard` E2E tests assume POST-then-PATCH and break.**
  Mitigation: Update or remove tests that assert PATCH ordering. The new atomic POST is observable via the single `app.created.v1` with `design_system_ref` populated; tests should assert that instead.

- **Risk: D5's interface-only contract is invisible until F1b realizes design skills.**
  Mitigation: The E2E test in this change asserts the propagation through the stub skills. Event payloads are observable; smoke test fails if `design_system_ref` is missing from `sdlc.ui_blueprint.proposed.v1`. When F1b lands, the existing test stays green if the contract is preserved.

- **Risk: shared `DesignSystemPicker` component coupling Friendly view to wizard route in ways that force one to compromise.**
  Mitigation: Strict presentational/host split (no API calls, no routing inside the component). If a divergence appears, fork is one new component, not refactoring both call sites.

## Migration Plan

1. Land `alfred-litellm-header-injection` (predecessor, separate change).
2. Land the application-service changes (POST body + validation + new event) behind no feature flag. Existing POST callers without the new field continue working (additive, not breaking).
3. Land the shared `DesignSystemPicker` component.
4. Migrate `/alfred/wizard` to atomic POST + new component. Verify existing wizard E2E green.
5. Add the picker to the Friendly view's "Nueva App" conversation. Ship behind `friendly_view.design_system_picker_enabled` GrowthBook flag for the first 7 days to allow rollback without code revert.
6. Update `intent-to-infrastructure-reference-flow` workflow to load + propagate + emit `design_system_ref`. Verify smoke test asserts the new payload field.
7. Update runbook (`docs/runbooks/intent-to-infrastructure-demo.md`) with a screenshot of the picker step.
8. After 7 days at 100% rollout, remove the flag.

**Rollback.** Steps 5-7 are flag-gated or additive; rollback is a flag flip or revert. Steps 2-4 are non-breaking refactors; revert is a code revert. No data migration to undo.

## Open Questions

- Does the Friendly view's "Nueva App" card already have intermediate state between "user types intent" and "workflow dispatch confirmed"? Or does it dispatch immediately on the first complete message? If the latter, we need to add an intermediate state (the picker) which is a slightly larger refactor than just inserting a panel. Investigate `FriendlyView.tsx` during implementation; if the conversation already routes through `POST /v1/intent/start`, the picker fits between that and the eventual `POST /v1/workspaces/{ws}/apps`.
- Should the NEW badge on the picker step persist for a fixed window (30 days post-launch) or until the user has seen it once? Engagement nuance; default to the simpler 30-day window unless analytics ask otherwise.
- How does the picker behave for the "Mejorar" card (extending an existing App)? It SHOULD be invisible there (the App already has a `design_system_ref`); the swap endpoint is the mechanism to change it. Confirm during implementation that the picker is gated on the new-App branch only, matching the existing wizard semantics.
- Does `app.design_system.user_skipped.v1` need to flow into `finops`/`ai-observability` for a "catalog discoverability" dashboard? Probably yes (small follow-up), but not required for this change to ship.
