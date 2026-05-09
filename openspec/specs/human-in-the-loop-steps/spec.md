# human-in-the-loop-steps Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Pause execution awaiting approval

A `human-in-the-loop` step MUST pause execution until an approver with the declared role decides; the pending decision MUST appear in the Approvals Inbox with full workflow context.

#### Scenario: Workflow waits for product-owner approval

- **GIVEN** a workflow with a `human-in-the-loop` step `approver_role=product-owner, timeout=24h`
- **WHEN** execution reaches the step
- **THEN** an Approvals Inbox entry MUST be created with previous-steps outputs and proposed-next-step inputs
- **AND** the workflow MUST stay in `waiting` state
- **AND** emit `workflow.step.waiting_human.v1`

### Requirement: Approver may modify inputs

Before approving, the approver MAY edit the inputs to be passed to the next step; modifications MUST be audited.

#### Scenario: Modified inputs recorded

- **GIVEN** an approver editing the proposed PR title
- **WHEN** they approve with the modification
- **THEN** the workflow MUST resume with the edited inputs
- **AND** the audit log MUST include both original and final inputs with diff

### Requirement: Timeout behavior configurable

The step MUST support `on_timeout` policy `{fail | proceed | escalate}`; default is `fail`.

#### Scenario: Escalate routes to next approver

- **GIVEN** a step with `on_timeout: escalate, escalation_role: engineering-manager, timeout=2h`
- **WHEN** 2h elapse without decision
- **THEN** the entry MUST be re-routed to `engineering-manager`
- **AND** emit `workflow.step.escalated.v1`
