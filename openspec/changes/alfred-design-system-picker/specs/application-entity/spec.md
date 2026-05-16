## MODIFIED Requirements

### Requirement: App CRUD API

The platform SHALL expose `POST /v1/workspaces/{ws}/apps`, `GET /v1/workspaces/{ws}/apps`, `GET /v1/apps/{id}`, `PATCH /v1/apps/{id}`, `POST /v1/apps/{id}:archive`, `POST /v1/apps/{id}:restore`, `DELETE /v1/apps/{id}`, **`POST /v1/apps/{id}/design-system:swap`**, **`PATCH /v1/apps/{id}/design-system/overrides`**. All endpoints SHALL enforce OpenFGA authorization against the App and its parent Workspace. Mutating endpoints SHALL emit the matching `app.*.v1` event and produce an immutable audit record.

The `POST /v1/workspaces/{ws}/apps` body SHALL accept two optional fields: `design_system_ref` (string) and `design_system_chosen_explicitly` (boolean, defaults to `false` when omitted). When `design_system_ref` is supplied, the platform SHALL validate it resolves to a Design System with `lifecycle_state=approved` and visibility to the App's tenant (same validation path as `POST /v1/apps/{id}/design-system:swap`), set `app.design_system_ref` to the resolved value at creation, and include the resolved value in the `app.created.v1` event payload. When `design_system_ref` is omitted, the platform SHALL resolve the `ds-forge-default` alias server-side and set the resolved ref on the App. When `design_system_chosen_explicitly=false` (whether explicitly set or defaulted), the platform SHALL ALSO emit `app.design_system.user_skipped.v1` with `{app_id, workspace_id, tenant_id, principal, correlation_id}`. The audit record for App creation SHALL include both the resolved `design_system_ref` and `design_system_chosen_explicitly`.

#### Scenario: Patch updates name and description

- **GIVEN** an App `app-1` and a caller with `app#editor`
- **WHEN** the caller calls `PATCH /v1/apps/app-1` with `{name: "New Name"}`
- **THEN** the App MUST be updated, an `app.updated.v1` event MUST be emitted with the diff
- **AND** the audit record MUST list the prior and new values

#### Scenario: Archive prevents new specs

- **WHEN** an App is moved to `lifecycle_state=archived`
- **THEN** subsequent attempts to create a new OpenSpec with `app_id=<archived>` MUST be rejected with `409 app_archived`
- **AND** existing OpenSpecs MUST remain readable

#### Scenario: Delete refused while live artefacts exist

- **WHEN** a caller calls `DELETE /v1/apps/app-1` and `app-1` has at least one OpenSpec, deployment, runtime registration or onboarding request not in a terminal state
- **THEN** the platform MUST reject the deletion with `409 app_has_live_artefacts` and return the conflicting list

#### Scenario: Design System swap opens a PR

- **GIVEN** an App `app-1` with `design_system_ref=desing-system-1@1.4.0` and a caller with `app#owner`
- **WHEN** the caller calls `POST /v1/apps/app-1/design-system:swap` with `{target_ref: desing-system-3@2.0.0, reason: "Corporate refresh"}`
- **THEN** the platform MUST validate the target is approved, open a PR against the App's portal-bundle repository, and emit `app.design_system.swap_requested.v1`

#### Scenario: Atomic create with explicit design_system_ref

- **GIVEN** a caller with `workspace.member` and `workspace:create-app` on `ws-1`
- **WHEN** the caller issues `POST /v1/workspaces/ws-1/apps` with body `{name, slug, description, owners, design_system_ref: "desing-system-3@2.0.0", design_system_chosen_explicitly: true}`
- **THEN** the platform MUST validate `desing-system-3@2.0.0` is approved and visible to the workspace's tenant
- **AND** persist the App with `design_system_ref=desing-system-3@2.0.0`
- **AND** emit a single `app.created.v1` event containing the resolved ref and `design_system_chosen_explicitly=true`
- **AND** the audit record MUST list `design_system_chosen_explicitly=true`
- **AND** the platform MUST NOT emit `app.design_system.user_skipped.v1`

#### Scenario: Atomic create with omitted design_system_ref defaults to the alias

- **GIVEN** the same caller
- **WHEN** the caller issues `POST /v1/workspaces/ws-1/apps` with body `{name, slug, description, owners}` (no design_system_ref, no chosen_explicitly)
- **THEN** the platform MUST resolve `ds-forge-default` server-side
- **AND** persist the App with the resolved ref
- **AND** emit `app.created.v1` with the resolved ref and `design_system_chosen_explicitly=false`
- **AND** additionally emit `app.design_system.user_skipped.v1`

#### Scenario: Atomic create rejected for non-approved design_system_ref

- **WHEN** the caller issues `POST /v1/workspaces/ws-1/apps` with `design_system_ref="some-ds@1.0.0"` and the target's `lifecycle_state` is `proposed`
- **THEN** the platform MUST reject with `409 design_system_not_approved`
- **AND** the App MUST NOT be persisted
- **AND** no events MUST be emitted

#### Scenario: Atomic create rejected for design_system_ref invisible to tenant

- **WHEN** the caller issues `POST /v1/workspaces/ws-1/apps` with `design_system_ref="other-tenants-ds@1.0.0"` and the target is not `tenant_global` or shared with the caller's tenant
- **THEN** the platform MUST reject with `404 design_system_not_visible`
- **AND** the App MUST NOT be persisted

## ADDED Requirements

### Requirement: Design System selection audit on App creation

Every App creation SHALL record `design_system_chosen_explicitly` (boolean) on the App's audit record and SHALL include it in the `app.created.v1` event payload. The flag SHALL be `true` when the create-request body carried `design_system_chosen_explicitly=true` AND a valid `design_system_ref`. The flag SHALL be `false` when the body omitted the field, set it to `false`, or omitted `design_system_ref` (in which case the alias is resolved server-side). When the flag is `false`, the platform SHALL additionally emit `app.design_system.user_skipped.v1` so downstream observability can measure picker discoverability.

#### Scenario: Audit record carries the flag

- **WHEN** an App is created via the atomic POST (either explicit or skip path)
- **THEN** the App's immutable audit record MUST include `design_system_chosen_explicitly` as a boolean field
- **AND** the field MUST match the value emitted on `app.created.v1`

#### Scenario: Skip event observable from event bus

- **WHEN** an App is created with `design_system_chosen_explicitly=false`
- **THEN** subscribers of `app.design_system.user_skipped.v1` MUST receive an event with `{app_id, workspace_id, tenant_id, principal, correlation_id}` within the same transaction as the `app.created.v1` emission
- **AND** the two events MUST share the same `correlation_id`
