## ADDED Requirements

### Requirement: User-skipped event

The platform SHALL emit `app.design_system.user_skipped.v1` whenever an App is created without an explicit Design System choice. The event SHALL include `{app_id, workspace_id, tenant_id, principal, correlation_id, resolved_ref}` where `resolved_ref` is the alias-resolved Design System the App was created with (typically the value `ds-forge-default` points to at that moment). The event SHALL be emitted in the same transaction as `app.created.v1` and SHALL share its `correlation_id`. The event is informational; it does not gate any workflow.

#### Scenario: Skipped App emits the event

- **GIVEN** an App created via `POST /v1/workspaces/{ws}/apps` with `design_system_chosen_explicitly=false` (or omitted)
- **WHEN** the platform finalizes App creation
- **THEN** subscribers of `app.design_system.user_skipped.v1` MUST receive an event with `{app_id, workspace_id, tenant_id, principal, correlation_id, resolved_ref}`
- **AND** the event's `correlation_id` MUST match the `correlation_id` of the simultaneous `app.created.v1`
- **AND** the event's `resolved_ref` MUST equal the App's persisted `design_system_ref`

#### Scenario: Explicit choice does NOT emit the event

- **GIVEN** an App created via `POST /v1/workspaces/{ws}/apps` with `design_system_chosen_explicitly=true` and a valid `design_system_ref`
- **WHEN** the platform finalizes App creation
- **THEN** the platform MUST NOT emit `app.design_system.user_skipped.v1`
- **AND** subscribers MUST observe only the standard `app.created.v1`

#### Scenario: Event has no follow-on workflow side effect

- **WHEN** `app.design_system.user_skipped.v1` is emitted
- **THEN** no workflow MUST pause, branch, or take any action based on the event
- **AND** the event MUST exist solely for observability (catalog discoverability metric, audit trail)
