# Spec Delta: ai-asset-registry-minimal (MODIFIED)

## MODIFIED Requirements

### Requirement: Workflow asset type

The Registry SHALL support asset `type=workflow` with sub-resources `version`, `eval_run`, `installation`.

#### Scenario: Workflow asset registered with versions

- **GIVEN** a published workflow `wf-1`
- **WHEN** queried via `GET /v1/assets/wf-1`
- **THEN** the response MUST list versions, latest eval runs, and installations across Workspaces
- **AND** include lifecycle state per version

#### Scenario: Eval-dataset asset type

- **GIVEN** a registered eval dataset `ds-7`
- **WHEN** queried
- **THEN** the response MUST include version history and trust level
- **AND** the dataset MUST be referenced by workflow eval runs that consumed it
