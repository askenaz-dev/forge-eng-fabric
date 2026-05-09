# ai-workflow-engine Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Visual workflow editor in the Portal
The platform SHALL provide a visual workflow editor (style of n8n/Flowise) integrated into the Custom Portal. Users SHALL be able to design, edit, version and publish workflows visually.

#### Scenario: User builds a workflow visually and publishes it
- **WHEN** an authorized user composes a workflow visually with valid nodes and connections and publishes it
- **THEN** the workflow is persisted with a SemVer, validated, and registered in the Asset Registry as a `workflow` asset

### Requirement: Declarative DSL representation
Every workflow SHALL have a declarative DSL representation (e.g., YAML/JSON) equivalent to its visual form. The DSL SHALL be the canonical source for execution and Git-based versioning.

#### Scenario: Visual edit reflects in DSL and vice versa
- **WHEN** a workflow is modified in the visual editor or by editing the DSL
- **THEN** both representations remain in sync and produce the same execution graph

### Requirement: Catalog of node types
The engine SHALL support at least the following node types: **LLM call, MCP tool, Skill invoke, Agent invoke/delegate, Prompt Template, HITL gate, Branch, Loop, Retry, Eval, Webhook, GitHub action, Deploy action, Approval action, Notification**.

#### Scenario: Workflow uses HITL gate before deploy
- **WHEN** a workflow with an HITL gate node before a Deploy action is executed in a Workspace requiring approval for `deploy:staging`
- **THEN** execution pauses at the gate, an approval request is created, and resumes only after approval

### Requirement: Configuration in Portal, execution in runners
Workflow configuration SHALL occur in the Portal; execution SHALL occur in the Agentic Execution Platform's isolated runners with the same policy, telemetry and audit controls as other executions.

#### Scenario: Workflow execution inherits runner controls
- **WHEN** a workflow is executed
- **THEN** every node call goes through policy checks, secret brokering, telemetry and audit just like direct asset invocations

### Requirement: Versioning and rollback
Workflows SHALL be versioned (SemVer) and immutable once published. The platform SHALL support pinning a Workspace to a specific workflow version and rolling forward/back across versions.

#### Scenario: Rollback to previous workflow version
- **WHEN** a Workspace owner rolls back a workflow from version 2.0.0 to 1.4.2
- **THEN** subsequent executions in that Workspace use 1.4.2 and the change is audited

### Requirement: Optional durable execution via Temporal
The engine SHALL be able to run workflows on **Temporal** when durability/long-running guarantees are needed, while short workflows MAY run on lightweight runners.

#### Scenario: Workflow declared as durable runs on Temporal
- **WHEN** a workflow is annotated as `durable: true`
- **THEN** the engine schedules its execution on Temporal with checkpointing, retries and replay enabled

### Requirement: Workflows are not mandatory for every Alfred action
Alfred SHALL be able to execute actions directly without an explicit workflow. Workflows SHALL be reserved for repeatable, auditable, parameterizable or shareable processes across teams.

#### Scenario: Alfred executes an ad-hoc tool call without a workflow
- **WHEN** Alfred performs a one-off action allowed by policy
- **THEN** the action executes without requiring a Workflow asset, while still being audited and policy-checked

