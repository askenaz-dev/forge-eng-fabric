# Spec Delta: healing-engine (ADDED)

## ADDED Requirements

### Requirement: Five-level healing model

The engine SHALL support 5 levels: L1 (Notify), L2 (Suggest), L3 (Act-with-approval), L4 (Act-autonomously), L5 (Act-and-rollback). Each invocation MUST record `level_decided` and rationale.

#### Scenario: L3 path requires approval

- **GIVEN** an envelope assigning L3 for action `restart-pod` in `env=stage`
- **WHEN** an incident triggers `restart-pod`
- **THEN** the engine MUST create an Approvals Inbox entry with diagnosis context
- **AND** wait for the approval signal up to TTL
- **AND** emit `healing.level_decided.v1{level=L3}` and `healing.triggered.v1{status=waiting_approval}`

#### Scenario: L5 auto-rollback on verify failure

- **GIVEN** action `refresh-cache` allowed at L5 in `env=prod` with `reversible=true`
- **WHEN** the action runs and post-action verify fails
- **THEN** the engine MUST invoke the inverse workflow (rollback)
- **AND** emit `healing.executed.v1{outcome=failed}` followed by `healing.rolled_back.v1`
- **AND** escalate to a human via `healing.escalated.v1`

### Requirement: Envelope enforcement

The engine MUST consult the matching envelope for `(capability, asset, env, criticality)`; actions exceeding `allowed_levels` MUST be degraded to the maximum allowed level.

#### Scenario: Degrade L4 to L3 when envelope caps at L3

- **GIVEN** envelope cap `allowed_levels=[L1,L2,L3]` for `env=prod`
- **WHEN** an action would default to L4
- **THEN** the engine MUST execute at L3 instead
- **AND** emit `healing.level_decided.v1{requested=L4, applied=L3, reason=envelope_cap}`

### Requirement: Kill switch precedence

When the kill switch is active (global or Workspace-scoped), the engine MUST degrade all actions to L1 (Notify-only) until disabled.

#### Scenario: Kill switch suppresses execution

- **GIVEN** the global kill switch is `active=true`
- **WHEN** any incident matches an action
- **THEN** the engine MUST emit `healing.triggered.v1{level=L1, suppressed=true, reason=kill_switch}`
- **AND** NOT invoke the underlying workflow

### Requirement: Action promotion gating

Promotion of an action to L4 or L5 in a given env MUST require: eval suite ≥95% on last 50 runs, ≥20 successful L3 dry-run executions, 30-day postmortem-free window, approval by `platform-admin` and `security-approver`.

#### Scenario: Reject premature promotion

- **GIVEN** an action with only 10 successful L3 runs
- **WHEN** promotion to L4 is requested
- **THEN** the engine MUST refuse with `412 promotion_prerequisites_unmet`
- **AND** report the missing prerequisites
