# workflow-runtime Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Durable execution on Temporal

Workflow executions MUST run on Temporal with `durable: true`; restarts of workers MUST NOT lose progress.

#### Scenario: Worker restart resumes execution

- **GIVEN** an execution paused at step `human-approval`
- **WHEN** the worker process restarts
- **THEN** Temporal MUST resume the execution from the last committed step
- **AND** the workflow MUST complete normally upon approval signal

### Requirement: Tenant isolation via namespaces

Each Tenant MUST have its own Temporal namespace; cross-namespace access MUST be denied.

#### Scenario: Cross-tenant signal denied

- **GIVEN** an execution in Tenant A namespace
- **WHEN** an actor in Tenant B attempts to send a signal
- **THEN** Temporal MUST refuse based on namespace ACL
- **AND** the orchestrator MUST emit `guardrail.trip.v1{reason=cross_tenant_signal}`

### Requirement: Retries and compensations

Steps MUST honor declarative retries; failures whose `compensate_with` is set MUST execute the compensation activity in saga-reverse order.

#### Scenario: Compensation runs on partial failure

- **GIVEN** a workflow with steps A, B, C where B has `compensate_with: rollback_B` and C fails
- **WHEN** C exhausts retries
- **THEN** the runtime MUST invoke `rollback_B`
- **AND** emit `workflow.compensated.v1` with sequence and outcomes

### Requirement: Step events to bus

Every step transition MUST emit a CloudEvent to the platform bus for observability and traceability.

#### Scenario: Step start/complete events

- **GIVEN** an execution with 4 steps completing successfully
- **WHEN** the execution finishes
- **THEN** 4 `workflow.step.started.v1` and 4 `workflow.step.completed.v1` events MUST be emitted
- **AND** the traceability service MUST record them as nodes/links

### Requirement: Accept trigger-originated executions

`workflow-runtime` SHALL accept executions originating from `trigger-router` via `POST /v1/executions` with an additional optional field `trigger_event: { trigger_id, fired_at, payload, queue_position? }`. When present, the runtime SHALL bind `$triggers.<trigger_id>` to the payload for step expression resolution. The existing direct-POST contract (without `trigger_event`) SHALL continue to work unchanged for invoke-only workflows.

#### Scenario: Execution started by trigger-router

- **GIVEN** trigger-router POSTs `/v1/executions` with `workflow_id`, `version`, and `trigger_event: { trigger_id: e, fired_at: ..., payload: { from: "alice@acme.com" } }`
- **WHEN** the execution runs a step with `inputs: { author: $triggers.e.from }`
- **THEN** the step MUST receive `author="alice@acme.com"`
- **AND** the runtime MUST emit `workflow.started.v1` with `cause.trigger_id=e` for traceability

#### Scenario: Direct-POST execution preserved

- **GIVEN** a client POSTs `/v1/executions` without `trigger_event`
- **WHEN** the workflow has zero triggers
- **THEN** the execution MUST start exactly as today and `$triggers` MUST be empty
- **AND** any step referencing `$triggers.<...>` MUST fail with `unbound_trigger_reference`

### Requirement: Queue semantics for trigger concurrency

The runtime SHALL honor per-trigger `concurrency` policy declared on the workflow: `queue` (default — serialize executions per trigger, exposing `trigger_event.queue_position`), `drop` (refuse to start while a prior execution is running), or `overlap` (start in parallel). For `drop` policy, the runtime SHALL respond `409 drop_concurrency` and emit `workflow.trigger.dropped.v1`.

#### Scenario: Queue serializes executions

- **GIVEN** a workflow with `concurrency: queue` and three trigger events arriving while one execution is in flight
- **WHEN** the events are accepted
- **THEN** they MUST execute in arrival order
- **AND** each execution MUST have a monotonically increasing `trigger_event.queue_position`

#### Scenario: Drop refuses concurrent

- **GIVEN** a workflow with `concurrency: drop` and an execution already running
- **WHEN** a new trigger event arrives
- **THEN** the runtime MUST respond `409 drop_concurrency`
- **AND** emit `workflow.trigger.dropped.v1`

### Requirement: LLM step execution

For steps of `type: llm`, the runtime SHALL: (a) resolve the prompt template via `prompt-template-service`, (b) resolve `model.ref` via `model-gateway`, (c) register declared `tools` with the model call (resolved via `mcp-gateway`), (d) enforce `max_tool_calls`, (e) validate the LLM response against the declared `outputs` schema, and (f) emit per-asset observability records per the `llm-flow-node` capability.

#### Scenario: LLM step end-to-end

- **GIVEN** an `llm` step with prompt template, model ref, two tools, and a declared output schema
- **WHEN** the step executes successfully
- **THEN** the runtime MUST have called `model-gateway` with the resolved model + credentials and the rendered prompt
- **AND** the runtime MUST have surfaced tool calls to the model via `mcp-gateway`
- **AND** the runtime MUST validate the response against `outputs` schema before passing control downstream
- **AND** an observability record MUST exist with model id, prompt template ref, token counts, tool call list, duration, and estimated cost

#### Scenario: LLM step exhausts tool budget

- **GIVEN** an `llm` step with `max_tool_calls: 3` and an LLM attempting a 4th tool call
- **WHEN** the budget exhausts
- **THEN** the runtime MUST return a budget-exhausted error to the LLM in place of the tool result
- **AND** emit `workflow.llm.budget_exhausted.v1` with the step id
