# Spec Delta: sdlc-design (ADDED)

## ADDED Requirements

### Requirement: Design skills

The capability SHALL expose `generate-api-contract`, `propose-data-model`, `lightweight-threat-model` as registered skills.

#### Scenario: API contract generated and validated

- **GIVEN** an initiative in `phase=design`
- **WHEN** Alfred invokes `generate-api-contract`
- **THEN** an OpenAPI document MUST be produced and committed via PR
- **AND** linted against schema (Spectral or equivalent) with no errors
- **AND** linked to the OpenSpec

### Requirement: Design gates

Gates `api_contracts_defined`, `data_model_documented`, `threat_model_present` (for `criticality≥medium`) MUST be evaluated before progression to `development`.

#### Scenario: Threat model required at medium criticality

- **GIVEN** an initiative with `criticality=medium` lacking a threat model
- **WHEN** progression is requested
- **THEN** gate `threat_model_present` MUST fail
- **AND** Alfred MUST suggest invoking `lightweight-threat-model`
