# automation-triggers Specification

## Purpose
TBD - created by syncing change ai-flow-authoring. Update Purpose after archive.
## Requirements

### Requirement: Trigger types in the canonical AST

The workflow AST SHALL accept an optional `spec.triggers` block listing zero or more triggers. Each trigger SHALL declare an `id` (unique within the workflow), a `type`, a `config` object (type-specific), and an `outputs` schema describing the event payload available to downstream steps via `$triggers.<id>.<field>`. A workflow with zero triggers SHALL remain valid and SHALL be invoke-only (callable only via direct POST to `workflow-runtime`).

#### Scenario: Workflow with a single trigger validates

- **GIVEN** a workflow YAML with `spec.triggers: [{ id: cron-1, type: cron, config: { expression: "0 */6 * * *" }, outputs: { fired_at: timestamp } }]`
- **WHEN** the workflow is published
- **THEN** the registry MUST accept the version
- **AND** the AST MUST round-trip YAML ↔ AST losslessly

#### Scenario: Workflow without triggers stays invoke-only

- **GIVEN** a published workflow with `spec.triggers` absent
- **WHEN** an external client POSTs to `workflow-runtime/v1/executions` with `workflow_id` and `inputs`
- **THEN** the execution MUST start exactly as today
- **AND** no trigger-router subscription MUST be created for this workflow

### Requirement: Supported trigger types

The platform SHALL ship support for at least these trigger types in this change: `manual`, `cron`, `webhook-in`, `event-bus`, `email-inbound`. Each type SHALL have a documented `config` schema and `outputs` shape. Adding a new trigger type SHALL require an OpenSpec change.

#### Scenario: Unknown trigger type rejected

- **WHEN** a workflow declares `spec.triggers: [{ id: x, type: unsupported-kind }]`
- **THEN** publish MUST fail with `400 lint_failed` and code `unknown_trigger_type`

#### Scenario: Manual trigger fires from the Portal

- **GIVEN** a workflow with `spec.triggers: [{ id: m, type: manual, outputs: { actor_email: string } }]`
- **WHEN** an authorized user clicks "Run now" in the Portal and provides the actor identity
- **THEN** the trigger-router MUST POST `/v1/executions` to `workflow-runtime` with `trigger_event: { trigger_id: m, payload: { actor_email: <user.email> } }`
- **AND** the execution MUST be auditable as `actor=<user.email>`

### Requirement: Trigger-router service

The platform SHALL provide a `trigger-router` service that subscribes to external sources and dispatches workflow executions. The service SHALL:

- Maintain a registry of active triggers per `(workflow_id, version)` sourced from `workflow-registry` on publish.
- Host webhook receiver endpoints under `/v1/hooks/in/{workflow_id}/{trigger_id}` for `webhook-in` triggers.
- Schedule cron triggers via Temporal cron workflows.
- Subscribe to platform event-bus topics for `event-bus` triggers.
- Host pluggable email adapters (IMAP at minimum in this change) for `email-inbound` triggers.
- Translate each trigger firing into a `POST /v1/executions` call to `workflow-runtime` with a `trigger_event` payload containing `{trigger_id, fired_at, payload}` and tenancy context.

#### Scenario: Webhook trigger fires execution

- **GIVEN** a published workflow with `spec.triggers: [{ id: gh, type: webhook-in, config: { secret_ref: ws:secret:gh-hook } }]`
- **WHEN** an external system POSTs a valid signed payload to `/v1/hooks/in/<workflow_id>/gh`
- **THEN** the trigger-router MUST verify the signature using `ws:secret:gh-hook`
- **AND** MUST POST `/v1/executions` to `workflow-runtime` with `trigger_event.trigger_id=gh` and the request body as `payload`
- **AND** an unsigned or invalid request MUST be rejected with 401 without firing the execution

#### Scenario: Cron trigger fires execution on schedule

- **GIVEN** a workflow with `spec.triggers: [{ id: nightly, type: cron, config: { expression: "0 3 * * *", timezone: "America/Mexico_City" } }]`
- **WHEN** the scheduled time elapses in the configured timezone
- **THEN** the trigger-router MUST POST `/v1/executions` with `trigger_event.payload.fired_at` set to the scheduled instant

#### Scenario: Email-inbound trigger fires on matching message

- **GIVEN** a workflow with `spec.triggers: [{ id: support, type: email-inbound, config: { mailbox_ref: ws:mailbox:support, filter: { subject_contains: "[urgent]" } } }]`
- **WHEN** a new message arrives matching the filter
- **THEN** the trigger-router MUST POST `/v1/executions` with `trigger_event.payload` containing `subject`, `from`, `body`, `received_at`, and `message_id`
- **AND** a message that does not match the filter MUST NOT fire the execution

### Requirement: Concurrency policy per trigger

Each trigger SHALL declare a `concurrency` policy: `queue` (default — events buffered, executions serialized), `drop` (new events discarded while an execution is running), or `overlap` (each event starts a parallel execution).

#### Scenario: Default concurrency is queue

- **GIVEN** a trigger without an explicit `concurrency` field
- **WHEN** two events arrive while a prior execution is still running
- **THEN** both new events MUST be queued and executed in order after the prior execution completes
- **AND** the workflow-runtime MUST tag executions with `trigger_event.queue_position`

#### Scenario: Drop policy discards while running

- **GIVEN** a trigger with `concurrency: drop`
- **WHEN** an event arrives while a prior execution is still running
- **THEN** the event MUST be discarded with a `workflow.trigger.dropped.v1` event emitted to the bus

### Requirement: Trigger payload is typed and addressable in steps

Steps SHALL be able to reference fields of the trigger payload via the expression `$triggers.<trigger_id>.<field>`. The expression SHALL resolve against the declared `outputs` schema, and references to undeclared fields SHALL fail lint at publish time.

#### Scenario: Step references trigger payload field

- **GIVEN** a workflow with `triggers: [{ id: email-in, outputs: { from: string } }]` and a step `inputs: { author: $triggers.email-in.from }`
- **WHEN** the workflow runs with `trigger_event.payload.from="alice@acme.com"`
- **THEN** the step MUST receive `author="alice@acme.com"`

#### Scenario: Reference to undeclared trigger field rejected at publish

- **WHEN** a workflow has `$triggers.email-in.body` but the trigger declares `outputs: { from: string }` only
- **THEN** publish MUST fail with `lint_failed` code `dangling_trigger_field`

### Requirement: Event-bus topics declared upfront

A workflow using an `event-bus` trigger SHALL declare the subscribed topics in `spec.triggers[].config.topics`. Lint SHALL refuse publish when a topic is not registered in the platform's event catalog.

#### Scenario: Subscribing to unknown topic refused

- **WHEN** a workflow declares `spec.triggers: [{ id: e, type: event-bus, config: { topics: ["unregistered.topic.v1"] } }]`
- **THEN** publish MUST fail with `lint_failed` code `unknown_event_topic`

### Requirement: Trigger events emitted to bus

When a trigger fires (successfully or as `dropped`), the trigger-router MUST emit a CloudEvent to the platform bus: `workflow.trigger.fired.v1` on success, `workflow.trigger.dropped.v1` on drop, `workflow.trigger.failed.v1` when dispatching to the runtime fails. Each event MUST include `tenant_id`, `workspace_id`, `workflow_id`, `version`, `trigger_id`, and a correlation id.

#### Scenario: Successful fire emits observability event

- **WHEN** a trigger fires and the runtime accepts the execution
- **THEN** a `workflow.trigger.fired.v1` event MUST be emitted within 1 second
- **AND** the `traceability-graph` service MUST record the link between trigger and execution

### Requirement: Legacy `event-trigger` step auto-migrates to triggers block

The canonical AST has historically expressed event-driven entrypoints as a step kind `event-trigger` with an `EventPattern { Type, Source, Filter }`. On parse, the DSL layer SHALL auto-migrate each `event-trigger` step into an entry in `spec.triggers` of type `webhook-in` (when `EventPattern.Source` is an HTTP source) or `event-bus` (otherwise). The migration MUST be lossless: `EventPattern.Type` becomes `trigger.config.topic` for `event-bus` or `trigger.config.event_type` for `webhook-in`; `EventPattern.Filter` becomes `trigger.config.filter`. The migration MUST emit a `deprecated_step_kind` lint warning at publish time without blocking the publish, and the registry MUST record the migrated form as the persisted AST with reason `migrate_event_trigger_to_triggers_block`.

#### Scenario: Legacy event-trigger parses into new triggers block

- **GIVEN** a workflow YAML with a step `{ id: src, type: event-trigger, event_pattern: { type: "github.push.v1", filter: { repo: "acme/*" } } }`
- **WHEN** the parser runs
- **THEN** the in-memory AST MUST contain `spec.triggers: [{ id: src, type: event-bus, config: { topic: "github.push.v1", filter: { repo: "acme/*" } }, outputs: { ... } }]`
- **AND** the original step MUST be removed from `spec.steps`
- **AND** lint MUST emit `deprecated_step_kind` warning with step id `src`

#### Scenario: Migrated form persists on next save

- **GIVEN** a published workflow with a legacy `event-trigger` step
- **WHEN** an author opens it in the editor and saves it (without other changes)
- **THEN** the registry MUST persist the migrated `triggers` block form
- **AND** the version bump MUST be PATCH with reason `migrate_event_trigger_to_triggers_block`

### Requirement: Trigger types parity between Portal and runtime

The Portal trigger palette SHALL render exactly the trigger types supported by `trigger-router` and enumerated in the canonical AST. A unit test SHALL enforce parity between the TS `TriggerType` union (used by the canvas) and the Go `ast.TriggerType` enum: every value in one MUST exist in the other. Drift between the two SHALL fail CI.

#### Scenario: Adding a TS trigger type without Go support fails CI

- **GIVEN** a developer adds `slack-mention` to the TS `TriggerType` union without adding it to `ast.TriggerType`
- **WHEN** the parity test runs in CI
- **THEN** CI MUST fail with `trigger_type_parity_mismatch: slack-mention missing from Go enum`
