## ADDED Requirements

### Requirement: Design skill implementations

The capability SHALL ship working implementations of `generate-ui-blueprint`, `generate-component-stubs` and `accessibility-audit` skills, registered in the Asset Registry with their eval suites. The skills SHALL be invokable through the standard skill-gateway and SHALL respect the App's resolved Design System reference.

#### Scenario: generate-ui-blueprint produces Figma-export-compatible JSON

- **GIVEN** an App `app-1` with `targets.design in {required, optional, opt-in}` and an approved API contract
- **WHEN** Alfred invokes `generate-ui-blueprint` against `app-1`
- **THEN** the skill MUST produce a UI blueprint document conforming to the Figma export JSON schema, persisted in the App's portal-bundle repo under `design/blueprints/<spec-slug>.json`
- **AND** the blueprint MUST reference the App's resolved `design_system_ref` for token consumption
- **AND** the skill MUST emit `sdlc.ui_blueprint.proposed.v1` with the blueprint path and the OpenSpec link

#### Scenario: generate-component-stubs uses the App's Design System

- **GIVEN** an App `app-1` with `design_system_ref=desing-system-2@1.0.0` and a UI blueprint
- **WHEN** Alfred invokes `generate-component-stubs`
- **THEN** the skill MUST produce React (or Vue, when the App stack is Vue) component stubs under `portal/src/app/(<app-slug>)/...` using only the canonical primitives bound to `desing-system-2`'s tokens
- **AND** the stubs MUST NOT hard-code colours/fonts; all visual values MUST come from CSS variables defined by the resolved Design System
- **AND** the skill MUST emit `sdlc.component_stubs.committed.v1` with the file list and the OpenSpec link

#### Scenario: accessibility-audit blocks on serious/critical violations at medium criticality

- **GIVEN** an App with `criticality=medium` and freshly committed component stubs
- **WHEN** `accessibility-audit` runs (Axe-core) against the rendered stubs
- **THEN** the report MUST classify findings by Axe severity
- **AND** the workflow gate `accessibility_audit_passed` MUST fail if any `serious` or `critical` finding is present
- **AND** the skill MUST emit `sdlc.accessibility_audit.completed.v1` with the full report and the gate verdict

## MODIFIED Requirements

### Requirement: Design skills

The capability SHALL expose `generate-api-contract`, `propose-data-model`, `lightweight-threat-model`, **`generate-ui-blueprint`**, **`generate-component-stubs`**, **`accessibility-audit`** as registered skills.

#### Scenario: API contract generated and validated

- **GIVEN** an initiative in `phase=design`
- **WHEN** Alfred invokes `generate-api-contract`
- **THEN** an OpenAPI document MUST be produced and committed via PR
- **AND** linted against schema (Spectral or equivalent) with no errors
- **AND** linked to the OpenSpec

#### Scenario: UI blueprint generated from approved contract

- **GIVEN** an App in `phase=design` with an approved API contract
- **WHEN** Alfred invokes `generate-ui-blueprint`
- **THEN** the skill MUST produce the blueprint document grounded in the contract and the App's Design System
- **AND** the blueprint MUST be linked to the OpenSpec's `linked_artifacts` under namespace `design:`

### Requirement: Design gates

Gates `api_contracts_defined`, `data_model_documented`, `threat_model_present` (for `criticality≥medium`), **`ui_blueprint_present`** (for Apps with `targets.design != skipped`), **`component_stubs_committed`** (for Apps with `targets.design in {required, autonomous}`), **`accessibility_audit_passed`** (for `criticality≥medium` and `targets.design != skipped`) MUST be evaluated before progression to `development`.

#### Scenario: Threat model required at medium criticality

- **GIVEN** an initiative with `criticality=medium` lacking a threat model
- **WHEN** progression is requested
- **THEN** gate `threat_model_present` MUST fail
- **AND** Alfred MUST suggest invoking `lightweight-threat-model`

#### Scenario: Component stubs gate fails on missing stubs

- **GIVEN** an App with `targets.design=required` and no committed component stubs
- **WHEN** progression to `development` is requested
- **THEN** gate `component_stubs_committed` MUST fail
- **AND** Alfred MUST surface the missing artefact and propose invoking `generate-component-stubs`
