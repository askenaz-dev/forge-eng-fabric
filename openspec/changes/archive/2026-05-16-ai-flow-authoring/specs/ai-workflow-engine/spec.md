## MODIFIED Requirements

### Requirement: Visual workflow editor in the Portal

The platform SHALL provide a visual workflow editor for AI Flows integrated into the Custom Portal. The editor SHALL be built on `@xyflow/react` (MIT) and SHALL follow the `workflow-visual-editor` capability for canvas behavior, palette structure, property panel, code view, and dry-run. Users SHALL be able to design, edit, version and publish AI Flows visually.

#### Scenario: User builds an AI Flow visually and publishes it

- **WHEN** an authorized user composes an AI Flow visually with valid triggers, nodes and connections and publishes it
- **THEN** the workflow is persisted with a SemVer, validated, and registered in the Asset Registry as a `workflow` asset
- **AND** any declared triggers SHALL be registered with `trigger-router` for dispatch

### Requirement: Catalog of node types

The engine SHALL support the canonical catalog organized into four families. The canonical Go enum `pkg/workflow/ast.StepType` SHALL be the single source of truth; the TS `CanonicalStepType` mirror SHALL be enforced by a parity unit test that fails CI on drift in either direction.

- **Triggers** (sibling block `spec.triggers`, not steps): `manual`, `cron`, `webhook-in`, `event-bus`, `email-inbound` (per `automation-triggers`).
- **AI** (steps): `llm` (per `llm-flow-node`), `agent`, `prompt-template`.
- **Actions** (steps): `mcp`, `skill`, `webhook` (outbound), `github-action`, `deploy-action`, `approval-action`, `notification-action`.
- **Logic** (steps): `branch`, `loop`, `human-in-the-loop`, `eval`. (Retry remains a per-step policy, not a step type.)
- **Extensibility** (step): `custom` per `custom-node-sdk`.
- **Deprecated** (preserved during transition, auto-migrated on save): `prompt` → `prompt-template`; the step-form `event-trigger` → entry in `spec.triggers`. Authors SHALL see a `deprecated_step_kind` warning at publish; the deprecated forms are removed in a follow-up change.

Per-type implementation depth in this change follows the table in `design.md` §D8. Adding a new built-in node type or trigger type SHALL require an OpenSpec change with capability impact.

#### Scenario: Workflow uses HITL gate before deploy

- **WHEN** a workflow with an HITL gate node before a Deploy action is executed in a Workspace requiring approval for `deploy:staging`
- **THEN** execution pauses at the gate, an approval request is created, and resumes only after approval

#### Scenario: Workflow combines trigger, AI, logic, and action

- **GIVEN** a workflow with an `email-inbound` trigger, an `llm` classify step, a `branch` on classification, and an `mcp` send-reply action
- **WHEN** an inbound email arrives matching the trigger filter
- **THEN** the runtime SHALL execute the steps in dependency order
- **AND** the LLM step SHALL execute per the `llm-flow-node` requirements
- **AND** the branch SHALL route to the correct downstream step based on the LLM output

#### Scenario: TS/Go catalog drift fails CI

- **WHEN** a developer adds a step type to the TS `CanonicalStepType` without adding it to the Go enum, or vice versa
- **THEN** the parity unit test SHALL fail in CI with `step_type_parity_mismatch` and the offending identifier

#### Scenario: Legacy `prompt` step still parses

- **GIVEN** a previously published workflow with a `prompt` step
- **WHEN** the workflow is read by the runtime or registry
- **THEN** parsing SHALL succeed, the step SHALL behave as `prompt-template`, and lint SHALL emit `deprecated_step_kind` with the step id

#### Scenario: Workflow uses HITL gate before deploy

- **WHEN** a workflow with an HITL gate node before a Deploy action is executed in a Workspace requiring approval for `deploy:staging`
- **THEN** execution pauses at the gate, an approval request is created, and resumes only after approval

#### Scenario: Workflow combines trigger, AI, logic, and action

- **GIVEN** a workflow with an `email-inbound` trigger, an `llm` classify step, a `branch` on classification, and an `mcp` send-reply action
- **WHEN** an inbound email arrives matching the trigger filter
- **THEN** the runtime SHALL execute the steps in dependency order
- **AND** the LLM step SHALL execute per the `llm-flow-node` requirements
- **AND** the branch SHALL route to the correct downstream step based on the LLM output
