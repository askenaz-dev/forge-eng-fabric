## ADDED Requirements

### Requirement: Design System selection step in the Nueva App flow

The Friendly view's **Nueva App** card SHALL render a Design System selection step AFTER the user describes the intent in plain language and BEFORE the workflow dispatch confirmation. The step SHALL fetch the catalog via `GET /v1/design-systems`, render each `lifecycle_state=approved` entry visible to the caller's tenant as a selectable card showing the entry's `manifest.screenshots.light`, `manifest.screenshots.dark`, `name`, and `manifest.use_case` copy. The step SHALL expose an explicit **Saltar** / **Skip** action distinct from "Continue with current selection". The card titles SHALL render in Instrument Serif italic and the use-case copy in Geist, matching the Friendly view's typography contract. Every string SHALL come from `portal/src/i18n/dictionary.ts` (ES default, EN fallback). The rendered text SHALL NOT show raw `asset_id` or `version` strings — only the catalog `name` and `use_case`, in keeping with the Friendly view's "no raw IDs" rule.

#### Scenario: Picker renders after intent capture in Nueva App

- **GIVEN** a user on the Friendly view with `workspace.member` who selects the Nueva App card and types "Quiero una app para registrar pedidos"
- **WHEN** Alfred acknowledges the intent and the conversation advances to the design-system step
- **THEN** the panel MUST fetch the catalog and render at least the four built-in templates (`desing-system-1..4`) as selectable cards with both light and dark screenshots
- **AND** the panel MUST render a Saltar / Skip action at the bottom of the step, visually distinct from the primary Continue action

#### Scenario: Card layout uses friendly typography

- **WHEN** the step renders
- **THEN** each card MUST render the catalog entry's `name` in Instrument Serif italic
- **AND** the `use_case` copy in Geist
- **AND** the raw `asset_id` / `version` strings MUST NOT appear in the rendered text

#### Scenario: Continue picks the focused card and creates the App atomically

- **GIVEN** the user has focused `desing-system-3` on the picker
- **WHEN** the user clicks Continue
- **THEN** the host MUST issue `POST /v1/workspaces/{ws}/apps` with `design_system_ref=desing-system-3@<latest_approved>` and `design_system_chosen_explicitly=true` in the body
- **AND** NO subsequent `PATCH /v1/apps/{id}` MUST be issued for the design system field
- **AND** the conversation MUST advance to the workflow dispatch confirmation step

#### Scenario: Skip creates the App with default and emits the skipped event

- **GIVEN** the user is on the design-system step
- **WHEN** the user clicks Saltar / Skip
- **THEN** the host MUST issue `POST /v1/workspaces/{ws}/apps` with `design_system_chosen_explicitly=false` and WITHOUT a `design_system_ref` in the body
- **AND** the platform MUST resolve `ds-forge-default` server-side and emit `app.created.v1` with the resolved ref
- **AND** the platform MUST additionally emit `app.design_system.user_skipped.v1` with `{app_id, workspace_id, tenant_id, principal, correlation_id}`

#### Scenario: Step is gated on Nueva App only

- **GIVEN** a user on the Friendly view who selects the Mejorar card (extending an existing App)
- **WHEN** the conversation runs through to dispatch
- **THEN** the design-system step MUST NOT appear
- **AND** the existing App's `design_system_ref` MUST remain unchanged

### Requirement: Catalog load failure falls back to default with clear copy

The Design System step SHALL handle catalog-load failures (network error, registry 5xx, empty response, slow response) by rendering a friendly fallback message and offering the user a one-click "Continue with the default" action that maps to the Skip flow (resolves to `ds-forge-default`, emits `app.design_system.user_skipped.v1`). A 5-second skeleton loader SHALL precede the empty-catalog fallback to avoid flashing the error state during normal latency.

#### Scenario: Catalog returns empty within 5s

- **WHEN** the Friendly view requests the catalog and the registry returns an empty list within 5s
- **THEN** the step MUST render: "No pudimos cargar el catálogo de Design Systems. Continuaremos con el predeterminado de Forge." / "We couldn't load the Design System catalog. We'll continue with the Forge default."
- **AND** the only action MUST be a Continue button that maps to the Skip flow

#### Scenario: Slow catalog shows skeleton loader

- **WHEN** the catalog request is in flight for longer than 200ms but less than 5s
- **THEN** the step MUST render skeleton card placeholders matching the eventual card layout
- **AND** MUST NOT render the empty-catalog fallback until the 5s timeout elapses or the request errors
