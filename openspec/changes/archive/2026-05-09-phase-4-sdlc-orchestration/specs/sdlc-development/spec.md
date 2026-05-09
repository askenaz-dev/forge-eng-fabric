# Spec Delta: sdlc-development (ADDED)

## ADDED Requirements

### Requirement: Development skills

The capability SHALL expose `implement-feature-tests-first`, `refactor-with-safety-net`, `apply-code-review-feedback` as registered skills.

#### Scenario: Feature implemented tests-first

- **GIVEN** an initiative in `phase=development` with a defined API contract
- **WHEN** Alfred invokes `implement-feature-tests-first`
- **THEN** the resulting PR MUST include tests added before implementation in the commit history
- **AND** tests MUST exercise the API contract endpoints
- **AND** the PR MUST pass `coverage` gate per criticality

### Requirement: Development gates

Gates `code_complete`, `lint_clean`, `unit_tests_passing`, `coverage` (per criticality) MUST be evaluated before progression to `qa`.

#### Scenario: Coverage threshold blocks progression

- **GIVEN** an initiative with `criticality=high` and coverage 78%
- **WHEN** progression is requested
- **THEN** gate `coverage` MUST fail (threshold 80%)
- **AND** emit `sdlc.phase.blocked.v1`
