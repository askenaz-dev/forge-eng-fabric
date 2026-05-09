# Spec Delta: policies-and-approvals (MODIFIED)

## MODIFIED Requirements

### Requirement: Autonomy envelope policy template

The policy engine SHALL provide template `autonomy-envelope` allowing per-(capability, asset_pattern, env, criticality) configuration of `default_level`, `allowed_levels`, `time_windows`, `max_actions_per_hour`.

#### Scenario: Envelope applied per request

- **GIVEN** envelope for `capability=sdlc-devops, asset_pattern=application/svc-*, env=prod, criticality=high`
- **WHEN** an action targeting `application/svc-foo` triggers in prod
- **THEN** the engine MUST consult that envelope first
- **AND** apply the constraints

### Requirement: Kill switch policy

The policy engine SHALL expose a `kill-switch` toggle global and per-Workspace; activation MUST require role `platform-admin` or `security-approver` and MUST be auditable.

#### Scenario: Kill switch activation logged

- **GIVEN** a `platform-admin` activates the global kill switch
- **WHEN** the action persists
- **THEN** the audit log MUST contain actor, timestamp, reason
- **AND** all healing engines MUST observe the change within 30s (cache TTL)

### Requirement: L5 reversibility constraint

Policy `require-reversible-for-l5` MUST be active and MUST refuse any L5 promotion or execution for actions with `reversible=false`.

#### Scenario: Reject L5 execution for non-reversible action

- **GIVEN** an action `delete-pvc` with `reversible=false` mistakenly assigned L5
- **WHEN** the engine evaluates
- **THEN** the policy MUST refuse with `412 reversibility_required_for_l5`
- **AND** degrade to the highest reversible level allowed
