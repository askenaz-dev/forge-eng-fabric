# self-healing-and-resilience Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Five operational levels for healing actions
The platform SHALL support five configurable operational levels for healing actions: **Notify**, **Suggest**, **Act with approval**, **Act autonomously**, **Act and rollback**. The level SHALL be configurable by Workspace, environment, action class and asset trust level.

#### Scenario: Restart workload under "Act autonomously"
- **WHEN** an alert fires whose policy is `act_autonomously` for the action class `restart_workload`
- **THEN** Alfred restarts the workload, validates recovery, and audits the action

#### Scenario: Rollback validates and reverts on failure
- **WHEN** a healing action under `act_and_rollback` policy fails post-validation
- **THEN** Alfred reverts the action automatically, opens an incident, and notifies owners

### Requirement: Diagnosis using runbooks, OpenSpec and telemetry
On alerts/events, Alfred (or the SRE capability) SHALL diagnose using runbooks, the related OpenSpec, telemetry (logs/metrics/traces) and the knowledge base, before proposing or executing an action.

#### Scenario: Diagnosis cites sources used
- **WHEN** Alfred proposes a healing action
- **THEN** the proposal includes references to the runbook, OpenSpec, telemetry queries and KB sources used

### Requirement: Initial set of healing actions
The platform SHALL provide an initial catalog of healing actions: restart of workload, rollback of deployment, scale up/down, reapply configuration, retry job/pipeline, regenerate certificate/configuration when safe, create issue/incident, notify responsibles, draft postmortem.

#### Scenario: Scale up under load
- **WHEN** load metrics breach scale-up policy and the action is `act_with_approval`
- **THEN** Alfred opens an approval request to scale up; on approval, the action executes and validates

### Requirement: Postmortems and evolution loop
For every executed healing action and incident, the platform SHALL generate a draft postmortem and SHALL propose, via Alfred, follow-up changes back into the originating OpenSpec and/or related assets.

#### Scenario: Incident produces follow-up OpenSpec change
- **WHEN** a postmortem is finalized
- **THEN** Alfred drafts updates to the OpenSpec and/or related assets and opens them for review subject to policy

### Requirement: Audit and policy enforcement for healing actions
Every healing action — proposal, approval and execution — SHALL be policy-checked, audited and emit telemetry comparable to other agentic actions, including `correlation_id` linking to the originating alert/incident.

#### Scenario: Healing action denied by policy
- **WHEN** a proposed healing action is not allowed by the current policy
- **THEN** Alfred halts execution, records the policy denial, and routes the alert to a human responder

