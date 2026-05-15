## ADDED Requirements

### Requirement: Design System selection step in the wizard (new-App branch)

The Intent Capture Wizard SHALL include a **Design System** step that activates only on the "create a new App" branch of the App scope step. The step SHALL list the four built-in templates (`desing-system-1..4`) with their screenshots and use-case copy, render a live preview panel that mounts the chosen tokens onto a sample composition (a button stack, a KPI tile, a card with a run row), and persist the selection on the draft App as `design_system_ref`. The step SHALL default to `ds-forge-default`. If the user is on the "extend an existing App" branch, the step SHALL be skipped (the App already has a Design System).

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
- **AND** clicking "Continue" without changing the selection MUST persist `design_system_ref=ds-forge-default`

#### Scenario: Selection persists on the draft App

- **WHEN** the user picks `desing-system-3` and clicks "Continue"
- **THEN** the wizard MUST persist `design_system_ref=desing-system-3@<latest_approved_version>` on the draft App in the wizard state
- **AND** the App creation call invoked at commit time MUST carry that value

### Requirement: Live preview panel renders real tokens

The Design System preview panel SHALL fetch the tokens CSS sheet of the selected template (using its sha256-pinned URL from the asset manifest) and apply it to a sandboxed sample composition. The panel SHALL render the sample in both light and dark themes simultaneously (side-by-side). The panel SHALL NOT leak the tokens into the wizard's own chrome.

#### Scenario: Preview applies tokens in isolation

- **WHEN** the user focuses `desing-system-2`
- **THEN** the preview MUST apply `desing-system-2`'s tokens to a sandboxed composition (e.g., a shadow DOM or a scoped `style[scoped]`)
- **AND** the wizard's own chrome (sidebar, top bar, footer) MUST continue to render with the Portal's design system

#### Scenario: Preview renders both themes side-by-side

- **WHEN** the preview panel is open
- **THEN** the panel MUST show two columns labelled "Claro" / "Oscuro" with the same sample composition, each with the corresponding theme applied
