## Why

The `design-system-catalog` capability (archived 2026-05-13) shipped the four built-in templates and the `intent-capture-wizard` spec already mandates a Design System step in the **/alfred/wizard** route (the slash-command power-user surface). That picker is implemented today in `portal/src/app/alfred/wizard/page.tsx` and `PATCH`es the App's `design_system_ref` after `POST /v1/workspaces/{ws}/apps` creates it. **Three concrete gaps remain that block the end-to-end intent-to-infrastructure promise:**

1. **The Friendly view (`/alfred?view=friendly`) — the default surface for non-technical users — has no design-system picker at all.** `portal/src/components/alfred/FriendlyView.tsx` contains zero references to `design_system`. A non-technical user landing on "Nueva App", which is the user's primary path into intent-to-infrastructure per the umbrella, never sees the catalog and silently inherits `ds-forge-default`. The Lovable-style picker the user shared in the design conversation needs to live HERE, not buried in the wizard route.
2. **The `intent-to-infrastructure-reference-flow` spec is silent on `design_system_ref` propagation.** Today the App carries the ref but `forge.reference.intent-to-infrastructure@1` does not contractually require the workflow to load it, surface it on its events, or pass it to the `sdlc-design` skills. When F1b (umbrella) makes those skills LLM-driven, there is no spec guarantee they will receive the user's choice. We need to nail this contract now, while the skills are still stubs, so F1b lands against a stable interface.
3. **`POST /v1/workspaces/{ws}/apps` does not accept `design_system_ref` in the create body.** The wizard works around this with POST-then-PATCH, which is two round-trips, two audit events, and a window where the App exists with `ds-forge-default` before the user's actual choice. Surfacing the picker in the Friendly view (gap 1) makes the atomic-create variant more important — the conversational flow shouldn't sandwich the choice between two backend calls.

Sequenced **after** `alfred-litellm-header-injection` because the Friendly view's intent capture leans on Alfred's reasoning loop, and any change to Alfred's UX that survives a multi-day rollout should inherit the corrected header injection (G1) and prompt-template-service migration (G2) from day one. There is no code dependency; only ordering to avoid landing UX work on top of a known-broken cost-attribution path.

## What Changes

- **Friendly view picker.** Add a Design System step to the "Nueva App" card's conversation in `portal/src/components/alfred/FriendlyView.tsx`. The step SHALL render after the user describes the intent in plain language and before the workflow dispatch confirmation. It SHALL list the four built-in templates with their `manifest.screenshots.light`/`manifest.screenshots.dark` and `manifest.use_case` copy in card form, expose an explicit **Skip** affordance (Saltar / Skip), and persist the selection on the to-be-created App. Card layout SHALL match the Lovable-style reference (large screenshot, name in Instrument Serif italic, use-case copy in Geist, single selection, NEW badge during the first 30 days post-launch). All strings via `portal/src/i18n/dictionary.ts` (ES default, EN fallback).
- **Explicit Skip semantics.** Skip is NOT the same as picking the default. Skip SHALL emit `app.design_system.user_skipped.v1` (new event) so we can measure the catalog's discoverability. Skip SHALL resolve to `ds-forge-default` at App-create time and stamp `design_system_chosen_explicitly=false` on the App's audit record. Picking a card explicitly SHALL stamp `design_system_chosen_explicitly=true`.
- **Atomic `POST` with `design_system_ref`.** Extend `POST /v1/workspaces/{ws}/apps` to accept an optional `design_system_ref` in the create body. When supplied, the service SHALL validate it resolves to an `approved` Design System visible to the App's tenant, set `app.design_system_ref` on creation, and include the resolved value in the single `app.created.v1` event. When omitted, the existing alias-resolution behavior (default `ds-forge-default`) SHALL continue unchanged. The wizard picker (existing) SHALL be migrated from POST+PATCH to the new atomic POST as part of this change.
- **Workflow propagation contract.** `forge.reference.intent-to-infrastructure@1` SHALL load the App snapshot with a resolved `design_system_ref` at dispatch time and propagate that ref to every `sdlc-design` skill invocation (`generate-ui-blueprint`, `generate-component-stubs`, `accessibility-audit`). The workflow's `sdlc.ui_blueprint.proposed.v1` and `sdlc.component_stubs.committed.v1` events SHALL include the active `design_system_ref` in their payload so downstream observability and traceability-graph can trace intent → chosen DS → generated artifacts.
- **Existing /alfred/wizard route stays.** No regression: the wizard's existing Design System step (mandated by `intent-capture-wizard` spec lines 179-222) keeps working for power users. The only refactor: switch its action from POST-then-PATCH to atomic POST, and surface the same explicit Skip semantics introduced for the Friendly view.
- **Tests.** Playwright E2E exercising Friendly view → "Nueva App" card → describe intent → land on picker → pick `desing-system-3` → confirm dispatch → assert `app.design_system_ref=desing-system-3@<v>`, assert workflow events carry the ref, assert no extra PATCH was issued. Plus an E2E for the Skip path: assert `app.design_system.user_skipped.v1` fires and the App's audit record carries `design_system_chosen_explicitly=false`. Plus Go service unit tests for the new POST body field (happy path, invalid ref, non-approved ref, omitted field).

## Capabilities

### New Capabilities

(none) — all behavior extends existing capabilities.

### Modified Capabilities

- `alfred-console-friendly`: ADD a "Design System selection step in the Nueva App flow" requirement. The Friendly view's "Nueva App" conversation SHALL render the catalog picker after intent capture and before dispatch, with an explicit Skip affordance. The step SHALL never surface raw refs in the rendered text (only the catalog `name` and `use_case`), in keeping with the existing "Friendly view replaces raw IDs with human labels" requirement.
- `intent-capture-wizard`: MODIFY the existing "Design System selection step in the wizard (new-App branch)" requirement to (a) reference the new atomic POST contract and (b) introduce the explicit Skip affordance + `app.design_system.user_skipped.v1` event. Behavior parity with the Friendly view's picker (same catalog, same skip semantics, same audit stamp).
- `application-entity`: MODIFY the "Application aggregate" and "App CRUD API" requirements so `POST /v1/workspaces/{ws}/apps` accepts an optional `design_system_ref` in the create body, validates approval/visibility, and includes the resolved value in `app.created.v1`. ADD a "Design System selection audit" requirement for the new `design_system_chosen_explicitly` flag and the `app.design_system.user_skipped.v1` event.
- `intent-to-infrastructure-reference-flow`: ADD a "Design System propagation through the workflow" requirement. The workflow SHALL load the App's resolved `design_system_ref` at dispatch, propagate it to every `sdlc-design` skill invocation, and include it in `sdlc.ui_blueprint.proposed.v1` and `sdlc.component_stubs.committed.v1` event payloads.
- `design-system-catalog`: ADD a "User-skipped event" requirement formalizing `app.design_system.user_skipped.v1` alongside the existing change-event family (`app.design_system.changed.v1`, `app.design_system.override_changed.v1`, etc.).

## Impact

- **Code**:
  - `portal/src/components/alfred/FriendlyView.tsx` — new Design System step in the "Nueva App" conversation.
  - `portal/src/components/alfred/DesignSystemPicker.tsx` — new shared component (used by both Friendly view and /wizard route after refactor) with card layout, Skip button, screenshot grid.
  - `portal/src/i18n/dictionary.ts` — new keys (`alfred_ds_step_title`, `alfred_ds_step_sub`, `alfred_ds_skip`, `alfred_ds_default_hint`, `alfred_ds_select_cta`, etc., ES default + EN).
  - `portal/src/app/alfred/wizard/page.tsx` — migrate from POST-then-PATCH to atomic POST; share component with Friendly view.
  - `services/application/internal/application/types.go` — add `DesignSystemRef` to create-request DTO (already on the App aggregate, missing on the create body).
  - `services/application/internal/application/service.go` — accept and validate `design_system_ref` at create time; emit `design_system_chosen_explicitly=true|false`; emit `app.design_system.user_skipped.v1` when the caller explicitly signals skip.
  - `services/application/internal/application/http.go` — bind the new field on the POST handler.
  - `services/application/internal/application/service_test.go` — new tests (atomic POST happy path, invalid ref, non-approved ref, omitted, explicit skip).
  - `services/sdlc-orchestrator/...` (workflow dispatch entry point) — load App snapshot with `design_system_ref` and pass it to design-skill invocations; include in `sdlc.ui_blueprint.proposed.v1` / `sdlc.component_stubs.committed.v1`.
  - `pkg/cloudevents/...` (or wherever the event schemas live) — register `app.design_system.user_skipped.v1`.
  - `tests/e2e/...` — Playwright spec covering Friendly view pick + skip flows.
- **APIs**:
  - `POST /v1/workspaces/{ws}/apps` body gains optional `design_system_ref` (additive, not breaking).
  - New event: `app.design_system.user_skipped.v1`.
  - No change to `GET /v1/design-systems` (catalog API).
  - No change to `PATCH /v1/apps/{id}` or swap endpoints.
- **Dependencies**:
  - Consumes existing `design-system-catalog` catalog API and `ds-forge-default` alias.
  - Sequenced after `alfred-litellm-header-injection` (ordering, no code dep).
  - Does not block on F0b/F0c/F1b — the workflow propagation contract is interface-only; F1b will fulfill it when design skills go LLM-driven.
- **Migration**: None. Existing Apps retain their `design_system_ref`. Existing wizard sessions in flight continue to work (POST-then-PATCH path still functions, will be removed after rollout window).
- **Tests**: New Playwright E2E + Go unit tests as above. Existing wizard E2E (if any) must remain green after the refactor.
- **Governance**: Change log entry references the umbrella `intent-to-infrastructure-gap-closure`, the archived `design-system-catalog` (2026-05-13), and the predecessor `alfred-litellm-header-injection`.

## Out of scope

- **AI-generated per-intent design previews** (the v0/Lovable pattern where each catalog entry shows a render of the user's specific intent in that DS). Static screenshots from `manifest.screenshots` only. Defer to a follow-up after `model-gateway-sdks` (F0b) lands and per-intent LLM cost attribution is reliable end-to-end.
- **Expanding the catalog beyond `desing-system-1..4`** (e.g., adding "Artistic Flair", "Clean Minimalism", "Professional Polish", "Elegant Dark", "High Density" as new built-in templates). The four existing templates are sufficient for MVP; additional templates are a separate design-authoring effort.
- **Surfacing tenant-published custom Design Systems in the picker.** The catalog API supports tenant-scoped entries; the picker MVP shows only `visibility=tenant_global` entries. Tenant-custom listing is a V2 follow-up.
- **Per-component overrides UI** at intent capture. The `PATCH /v1/apps/{id}/design-system/overrides` endpoint exists from the archived catalog change but is owner-only and post-deploy; surfacing it in intent capture would conflate flows.
- **Post-deploy swap UI in the Portal.** The `POST /v1/apps/{id}/design-system:swap` endpoint exists; building the swap UI is a separate App-settings change.
- **Changes to `design-system-catalog`'s catalog itself** (catalog contents, swap semantics, validator, override merger). This change only consumes the catalog and adds one new event type.
- **Changes to Alfred's LLM call path.** That work is `alfred-litellm-header-injection` (predecessor) and `alfred-via-model-gateway` (F1a). This change touches neither.
- **Implementing LLM-driven `sdlc-design` skills.** Those are F1b. This change only locks in the workflow's propagation contract so F1b lands against a stable interface.
