# Spec Delta: workflow-runtime (ADDED)

## ADDED Requirements

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
