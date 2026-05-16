# llm-flow-node Specification

## Purpose
TBD - created by syncing change ai-flow-authoring. Update Purpose after archive.
## Requirements

### Requirement: `llm` step type added to the canonical enum

The canonical Go enum `pkg/workflow/ast.StepType` SHALL gain the value `StepLLM = "llm"`. Parser, lint, registry version-classification, and runtime executor SHALL recognise it. The TS `CanonicalStepType` in `portal/src/lib/ast-canvas-adapter` SHALL include `"llm"` and the parity test (D8 in design) SHALL pass.

#### Scenario: LLM step parses and validates

- **GIVEN** a workflow YAML with a step of `type: llm` and all required LLM fields
- **WHEN** the parser runs
- **THEN** the AST MUST contain a step with `Type=StepLLM`
- **AND** lint MUST NOT report `unknown_step_type` for it

### Requirement: Deprecated `prompt` step type aliases to `prompt-template`

The legacy `prompt` step type (previously the only prompt-related primitive) SHALL be preserved in the enum as a deprecated alias of `prompt-template`. On parse the DSL layer SHALL emit a `deprecated_step_kind` lint warning for any `prompt` step and persist the migrated `prompt-template` form on next save (PATCH bump, reason `migrate_prompt_to_prompt_template`). `prompt` is distinct from `llm`: `prompt-template` renders a template; `llm` performs a model call with tool-calling.

#### Scenario: Legacy `prompt` step migrates on save

- **GIVEN** a workflow with a step `{ id: refine, type: prompt, ref: ... }`
- **WHEN** an author opens the workflow in the editor and saves it
- **THEN** the persisted form MUST have `type: prompt-template`
- **AND** the version bump MUST be PATCH with reason `migrate_prompt_to_prompt_template`

### Requirement: LLM node binds to a versioned prompt template

The `llm` step type SHALL reference a versioned prompt template via `prompt_template: registry:prompt/<scope>/<name>@<semver>`. The template MUST be resolved via `prompt-template-service`. Floating references (`@latest`, `@main`, `@stable`) SHALL be rejected at publish time, consistent with the existing workflow-DSL rule.

#### Scenario: LLM node publishes with pinned template

- **GIVEN** a workflow with `steps: [{ id: classify, type: llm, prompt_template: registry:prompt/sdlc-product/email-classify@1.3.0 }]`
- **WHEN** the workflow is published
- **THEN** the publish MUST succeed and the registry MUST resolve the template to the exact stored version

#### Scenario: Floating prompt-template reference rejected

- **WHEN** a workflow declares `prompt_template: registry:prompt/foo/bar@latest`
- **THEN** publish MUST fail with `lint_failed` code `floating_reference_not_allowed`

### Requirement: LLM node selects a model via the model gateway

The `llm` step SHALL declare `model.ref` in the form `gateway:model/<model-id>@<channel>` where `<channel>` is one of `latest-stable`, `latest`, or a pinned version. The `model-gateway` SHALL resolve the reference at execution time to a concrete model + credentials per workspace. Per-node `model.overrides` (e.g., `temperature`, `max_tokens`, `top_p`) SHALL be passed through unchanged to the gateway.

#### Scenario: Model resolved via gateway at execution

- **GIVEN** an LLM step with `model.ref: gateway:model/claude-opus-4-7@latest-stable`
- **WHEN** the step executes
- **THEN** the runtime MUST call `model-gateway` to resolve to the active model id and credentials for the current workspace
- **AND** the call MUST inherit the workspace's policy/audit controls

#### Scenario: Override passed through

- **GIVEN** an LLM step with `model.overrides: { temperature: 0.0 }`
- **WHEN** the step calls the gateway
- **THEN** the gateway MUST receive `temperature=0.0` in the request

### Requirement: LLM node tool-calling auto-bound to in-scope MCPs

The `llm` step SHALL accept a `tools: [string]` array of MCP references (`registry:mcp/<name>@<version>`). At execution time the runtime SHALL register each MCP's tool schemas with the model call so the LLM can invoke them. Tools MUST resolve through `mcp-gateway` to inherit credentials, policy, and audit. Tools MUST be a subset of the workflow's pinned `selected_assets.mcps` when a pinned set is declared.

#### Scenario: LLM invokes an MCP tool during the step

- **GIVEN** an LLM step with `tools: [registry:mcp/email-tools@2.0.0]`
- **WHEN** the LLM emits a tool-call for `send_reply` during the step
- **THEN** the runtime MUST invoke `email-tools@2.0.0/send_reply` via `mcp-gateway`
- **AND** the call MUST be recorded in per-asset observability with the correct workflow execution id

#### Scenario: Tool outside pinned set refused

- **GIVEN** a workflow with `metadata.selected_assets.mcps: [email-tools@2.0.0]` and an LLM step with `tools: [registry:mcp/database@1.0.0]`
- **THEN** publish MUST fail with `lint_failed` code `tool_outside_pinned_set`

### Requirement: LLM node declares output schema

The `llm` step SHALL declare an `outputs` schema (object with typed fields). Downstream steps SHALL reference LLM outputs as `$steps.<llm-step-id>.<field>` and lint SHALL fail at publish when a downstream reference points to an undeclared field.

#### Scenario: Downstream step references declared output

- **GIVEN** an LLM step `classify` with `outputs: { category: enum[urgent, billing, general], confidence: number }` followed by a step `inputs: { is_urgent: $steps.classify.category == "urgent" }`
- **WHEN** the workflow runs and the LLM returns `{ category: "urgent", confidence: 0.93 }`
- **THEN** the downstream step MUST receive `is_urgent=true`

#### Scenario: Reference to undeclared output rejected

- **WHEN** a downstream step references `$steps.classify.draft` but `classify.outputs` does not declare `draft`
- **THEN** publish MUST fail with `lint_failed` code `dangling_step_field`

### Requirement: LLM node enforces tool-call budget

Each LLM step SHALL accept `max_tool_calls` (default 10). The runtime MUST refuse further tool calls once the budget is exhausted and MUST emit `workflow.llm.budget_exhausted.v1` with the step id.

#### Scenario: Tool budget enforced

- **GIVEN** an LLM step with `max_tool_calls: 3`
- **WHEN** the LLM attempts a 4th tool call within the same step
- **THEN** the runtime MUST return a budget-exhausted error to the LLM in place of the tool result
- **AND** the step MUST complete with `outputs.budget_exhausted=true`

### Requirement: LLM node observability

Each LLM step invocation SHALL record: the resolved model id, the prompt template id + version, the rendered prompt size, the response token count, every tool call (id, args, result, duration), the total step duration, and the estimated cost. Records SHALL flow to `per-asset-observability` and SHALL be queryable by workflow execution id.

#### Scenario: LLM step records full trace

- **WHEN** an LLM step completes (success or failure)
- **THEN** an observability record MUST exist with all required fields
- **AND** the record MUST be retrievable via `per-asset-observability` filtered by the execution id

### Requirement: LLM node respects workspace model whitelist

If a workspace declares a model whitelist (`workspace.allowed_models`), the LLM step SHALL refuse to publish if its `model.ref` resolves to a model outside the whitelist. The check SHALL run at publish time and at execution time.

#### Scenario: Publish blocked when model not whitelisted

- **GIVEN** a workspace with `allowed_models: [claude-haiku-4-5]` and an LLM step referencing `gateway:model/claude-opus-4-7@latest-stable`
- **WHEN** publish runs
- **THEN** the registry MUST refuse with `lint_failed` code `model_not_whitelisted`
