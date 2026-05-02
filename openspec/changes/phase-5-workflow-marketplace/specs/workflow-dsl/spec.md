# Spec Delta: workflow-dsl (ADDED)

## ADDED Requirements

### Requirement: Canonical AST

The DSL parser MUST produce a canonical AST equivalent to the editor visual model; round-trip YAML ↔ AST MUST be lossless.

#### Scenario: Round-trip preserves semantics

- **GIVEN** a YAML workflow with nested branches and a human-in-the-loop step
- **WHEN** parsed to AST and serialized back to YAML
- **THEN** the resulting YAML MUST be semantically equivalent
- **AND** AST normalization MUST not alter ordering of dependent steps

### Requirement: Schema validation and lint

DSL submissions MUST be validated against JSON Schema and linted for unreachable steps, dangling dependencies, type mismatches, and cycles.

#### Scenario: Reject unreachable step

- **GIVEN** a DSL document with a step never referenced as `depends_on`
- **WHEN** linting runs
- **THEN** the lint MUST report `unreachable_step` with the offending id
- **AND** the publish flow MUST refuse with `400 lint_failed`

#### Scenario: Reject cycles

- **GIVEN** steps A → B → A in dependency
- **WHEN** linting runs
- **THEN** the lint MUST report `cycle_detected`
- **AND** publish MUST be denied
