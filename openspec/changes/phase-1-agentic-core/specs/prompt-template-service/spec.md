## ADDED Requirements

### Requirement: Versioned prompt templates with metadata
The platform SHALL provide a Prompt Template service where templates are versioned (SemVer), parameterized (typed variables), and carry: `owner_team`, `examples`, `recommended_model`, `cost_class`, `eval_suite`, `guardrails`, `change_history`, `lifecycle_state`, `trust_level`.

#### Scenario: Publish a new prompt template version
- **WHEN** an owner publishes a new prompt template version with metadata and at least one example
- **THEN** the service persists it as `proposed`, runs default evals and exposes it for invocation by Alfred and other agents

### Requirement: Variable schema validation
Variables SHALL be validated against a JSON schema before substitution. Invalid inputs SHALL be rejected with a clear error.

#### Scenario: Invalid variable rejected
- **WHEN** a caller invokes a prompt template with a variable violating the schema
- **THEN** the service rejects the invocation with 400 and emits a guardrail-trip event

### Requirement: Eval suite linked to lifecycle
Promotion to `approved` SHALL require eval suite results meeting the threshold for the template's trust level. Results SHALL be stored and visible in the Asset detail view.

#### Scenario: Promotion blocked by failing evals
- **WHEN** an owner tries to promote a template whose evals are below threshold
- **THEN** the promotion is rejected with the failing dimensions listed

### Requirement: Guardrails configurable per template
Each template SHALL declare guardrails (e.g., max tokens, content filters, allowed tools, output schema). Runtime SHALL enforce these guardrails.

#### Scenario: Output schema violation triggers guardrail-trip
- **WHEN** a model output violates the declared output schema
- **THEN** the runtime rejects the response, triggers a guardrail-trip event and either retries (per policy) or fails the call
