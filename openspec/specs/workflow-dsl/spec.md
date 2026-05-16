# workflow-dsl Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Canonical AST

The DSL parser MUST produce a canonical AST equivalent to the editor visual model; round-trip YAML ↔ AST MUST be lossless. The canonical AST SHALL accept an optional `spec.triggers` block alongside `spec.inputs` and `spec.steps`. Each trigger SHALL have `id`, `type`, `config` (object), `outputs` (schema), and optional `concurrency` policy (`queue` | `drop` | `overlap`, default `queue`).

#### Scenario: Round-trip preserves semantics

- **GIVEN** a YAML workflow with nested branches and a human-in-the-loop step
- **WHEN** parsed to AST and serialized back to YAML
- **THEN** the resulting YAML MUST be semantically equivalent
- **AND** AST normalization MUST not alter ordering of dependent steps

#### Scenario: Round-trip preserves triggers block

- **GIVEN** a YAML workflow with two triggers (a `cron` and an `email-inbound`)
- **WHEN** parsed to AST and serialized back to YAML
- **THEN** the resulting YAML MUST contain both triggers with all fields intact
- **AND** trigger ordering MUST be stable across normalization

### Requirement: Schema validation and lint

DSL submissions MUST be validated against JSON Schema and linted for unreachable steps, dangling dependencies, type mismatches, and cycles. Lint MUST additionally:

- Reject unknown trigger types (`unknown_trigger_type`).
- Reject step references to undeclared trigger outputs (`dangling_trigger_field`).
- Reject `event-bus` triggers subscribing to unregistered topics (`unknown_event_topic`).
- Reject LLM steps with floating prompt-template references (`floating_reference_not_allowed`).
- Reject LLM steps with tool references outside the workflow's pinned MCP set (`tool_outside_pinned_set`).
- Reject downstream references to undeclared LLM output fields (`dangling_step_field`).

#### Scenario: Reject unreachable step

- **GIVEN** a DSL document with a step never referenced as `depends_on` and not consumed by any downstream step or output
- **WHEN** linting runs
- **THEN** the lint MUST report `unreachable_step` with the offending id
- **AND** the publish flow MUST refuse with `400 lint_failed`

#### Scenario: Reject cycles

- **GIVEN** steps A → B → A in dependency
- **WHEN** linting runs
- **THEN** the lint MUST report `cycle_detected`
- **AND** publish MUST be denied

#### Scenario: Reject dangling trigger field reference

- **GIVEN** a workflow with `triggers: [{ id: e, outputs: { from: string } }]` and a step `inputs: { body: $triggers.e.body }`
- **WHEN** linting runs
- **THEN** the lint MUST report `dangling_trigger_field` with offending step id and field name
- **AND** publish MUST be denied

#### Scenario: Reject LLM step with floating prompt template

- **GIVEN** an LLM step with `prompt_template: registry:prompt/foo/bar@latest`
- **WHEN** linting runs
- **THEN** the lint MUST report `floating_reference_not_allowed`
- **AND** publish MUST be denied
