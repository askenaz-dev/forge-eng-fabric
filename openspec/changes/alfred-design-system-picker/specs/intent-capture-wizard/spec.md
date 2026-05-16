## MODIFIED Requirements

### Requirement: Design System selection step in the wizard (new-App branch)

The Intent Capture Wizard SHALL include a **Design System** step that activates only on the "create a new App" branch of the App scope step. The step SHALL list the four built-in templates (`desing-system-1..4`) with their screenshots and use-case copy, render a live preview panel that mounts the chosen tokens onto a sample composition (a button stack, a KPI tile, a card with a run row), and persist the selection on the draft App as `design_system_ref`. The step SHALL default to `ds-forge-default`. If the user is on the "extend an existing App" branch, the step SHALL be skipped (the App already has a Design System).

The step SHALL share its rendering component with the Friendly view's Nueva App picker (single `DesignSystemPicker` component in `portal/src/components/alfred/`) so both surfaces show the same card layout, screenshot grid, and copy.

The step SHALL expose an explicit **Saltar** / **Skip** action distinct from "Continue with the focused selection". The App SHALL be created via a single atomic `POST /v1/workspaces/{ws}/apps` carrying `design_system_ref` and `design_system_chosen_explicitly` in the body; the wizard SHALL NOT issue a follow-up `PATCH /v1/apps/{id}` for the design system field. Skip SHALL emit `app.design_system.user_skipped.v1` in addition to the standard `app.created.v1`.

#### Scenario: Step appears only when creating a new App

- **GIVEN** a wizard session on the "extend an existing App" branch with `draft.app_id=app-1`
- **WHEN** the wizard advances past the App scope step
- **THEN** the Design System step MUST be skipped
- **AND** the draft MUST NOT contain a `design_system_selection_pending` flag

#### Scenario: Step shows four templates with previews

- **GIVEN** a wizard session on the "create a new App" branch
- **WHEN** the user reaches the Design System step
- **THEN** the step MUST display `desing-system-1`, `desing-system-2`, `desing-system-3`, `desing-system-4` as selectable cards with screenshots in both light and dark
- **AND** the preview panel MUST render a sample composition with the tokens of the currently focused template

#### Scenario: Default is ds-forge-default

- **WHEN** the user lands on the Design System step without an explicit choice
- **THEN** the step MUST highlight `desing-system-1` (resolved via the `ds-forge-default` alias) as the default
- **AND** the Continue action MUST be clickable from this state

#### Scenario: Explicit Continue creates the App atomically

- **WHEN** the user focuses `desing-system-3` and clicks Continue
- **THEN** the wizard MUST issue `POST /v1/workspaces/{ws}/apps` with `design_system_ref=desing-system-3@<latest_approved>` and `design_system_chosen_explicitly=true` in the body
- **AND** NO subsequent `PATCH /v1/apps/{id}` for the design system field MUST be issued

#### Scenario: Skip creates the App with default and emits the skipped event

- **WHEN** the user clicks Saltar / Skip on the Design System step
- **THEN** the wizard MUST issue `POST /v1/workspaces/{ws}/apps` with `design_system_chosen_explicitly=false` and WITHOUT `design_system_ref` in the body
- **AND** the platform MUST resolve `ds-forge-default` server-side and additionally emit `app.design_system.user_skipped.v1`

#### Scenario: Picker component is shared with the Friendly view

- **WHEN** an engineer inspects the wizard's Design System step rendering
- **THEN** the step MUST render via the shared `DesignSystemPicker` component at `portal/src/components/alfred/DesignSystemPicker.tsx`
- **AND** the same component MUST be the renderer used by the Friendly view's Nueva App design-system step
