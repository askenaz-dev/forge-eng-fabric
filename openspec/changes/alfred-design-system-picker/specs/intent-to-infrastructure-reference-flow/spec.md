## ADDED Requirements

### Requirement: Design System propagation through the workflow

`forge.reference.intent-to-infrastructure@1` SHALL load the App snapshot with a resolved `design_system_ref` at dispatch time and SHALL propagate that ref to every `sdlc-design` skill invocation (`generate-ui-blueprint`, `generate-component-stubs`, `accessibility-audit`). The workflow SHALL include the active `design_system_ref` in the payload of `sdlc.ui_blueprint.proposed.v1` and `sdlc.component_stubs.committed.v1` so downstream observability and `traceability-graph` can link intent → chosen Design System → generated design artifacts.

This requirement defines the **interface** that `sdlc-design` skills SHALL receive; it does NOT mandate what the skills DO with the ref. Today the skills are stubs and SHALL receive the ref and ignore it. When the umbrella's F1b change makes those skills LLM-driven, they SHALL consume the ref to pick the correct tokens.

#### Scenario: Workflow loads App snapshot with design_system_ref

- **GIVEN** an App `app-1` with `design_system_ref=desing-system-3@2.0.0`
- **WHEN** the workflow is dispatched for `app-1`
- **THEN** the workflow's initial App snapshot MUST carry `design_system_ref=desing-system-3@2.0.0`
- **AND** the dispatch event MUST log the resolved ref

#### Scenario: Design skills receive design_system_ref in invocation args

- **GIVEN** a running workflow for `app-1` with `design_system_ref=desing-system-3@2.0.0`
- **WHEN** the orchestrator invokes `generate-ui-blueprint`
- **THEN** the skill invocation MUST include `design_system_ref=desing-system-3@2.0.0` in its arguments
- **AND** the same MUST be true for `generate-component-stubs` and `accessibility-audit` invocations in the same run

#### Scenario: UI blueprint event payload includes design_system_ref

- **WHEN** the workflow emits `sdlc.ui_blueprint.proposed.v1`
- **THEN** the event payload MUST include a top-level field `design_system_ref` matching the App's resolved ref at dispatch
- **AND** subscribers MUST be able to read the field without parsing the blueprint document

#### Scenario: Component stubs event payload includes design_system_ref

- **WHEN** the workflow emits `sdlc.component_stubs.committed.v1`
- **THEN** the event payload MUST include a top-level field `design_system_ref` matching the App's resolved ref at dispatch

#### Scenario: Design_system_ref is invariant during a single workflow run

- **GIVEN** a workflow run for `app-1` started with `design_system_ref=desing-system-3@2.0.0`
- **WHEN** the App's `design_system_ref` is changed mid-run via the swap PR mechanism
- **THEN** the running workflow MUST continue using the dispatch-time `design_system_ref` (`desing-system-3@2.0.0`) for all design-skill invocations
- **AND** the change MUST take effect only on subsequent workflow runs

#### Scenario: Skipped App still receives a resolved ref

- **GIVEN** an App created via the Skip path (the user clicked Saltar in the picker)
- **WHEN** the workflow dispatches for that App
- **THEN** the App's resolved `design_system_ref` MUST be `ds-forge-default`'s underlying target (e.g., `desing-system-1@<latest_approved>`)
- **AND** the workflow MUST propagate that resolved ref to design skills exactly as if the user had picked the default explicitly
