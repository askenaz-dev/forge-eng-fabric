# Spec Delta: healing-action-catalog (ADDED)

## ADDED Requirements

### Requirement: Healing actions as governed assets

Each healing action MUST be a Registry asset (`type=healing_action`) declaring runbook, parameters, risk, allowed_levels_by_env, reversibility, eval suite, and an underlying workflow implementation.

#### Scenario: Action registered with eval suite

- **GIVEN** action `restart-pod`
- **WHEN** registered
- **THEN** the asset MUST include `runbook_url`, `parameters`, `risk`, `allowed_levels_by_env`, `reversible`, `eval_suite`, `implementation`
- **AND** absence of `eval_suite` MUST refuse registration

### Requirement: Reversibility required for L4/L5

Actions promoted to L4 or L5 in any env MUST declare `reversible: true`.

#### Scenario: Reject L4 promotion of non-reversible action

- **GIVEN** action with `reversible: false`
- **WHEN** L4 promotion requested
- **THEN** the registry MUST refuse with `412 not_reversible`

### Requirement: Implementation via workflow

Action implementations MUST reference a Phase 5 workflow with pinned version; ad-hoc inline scripts MUST NOT be allowed.

#### Scenario: Reject inline implementation

- **GIVEN** an action declaring inline shell script
- **WHEN** registered
- **THEN** the registry MUST refuse with `400 inline_implementation_not_allowed`
