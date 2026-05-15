## ADDED Requirements

### Requirement: App carries a versioned Design System reference

Every App SHALL carry `design_system_ref`, a versioned pointer (`asset_id@version` or an alias like `ds-forge-default`) to a Design System asset. The reference SHALL resolve at build time when the App's portal bundle is generated. Apps SHALL NOT be creatable without an explicit `design_system_ref`; the wizard SHALL default to `ds-forge-default` and the App scaffolder SHALL fall back to `ds-forge-default` when no value is supplied.

#### Scenario: Create App with explicit Design System

- **WHEN** a caller calls `POST /v1/workspaces/{ws}/apps` with `design_system_ref=desing-system-2@1.0.0`
- **THEN** the platform MUST persist the App with that reference and resolve it to validate the asset exists and is `approved` at creation time

#### Scenario: Create App without Design System defaults to forge default

- **WHEN** a caller calls `POST /v1/workspaces/{ws}/apps` without `design_system_ref`
- **THEN** the platform MUST set `design_system_ref=ds-forge-default` and record the default in audit

#### Scenario: Reference to non-approved asset rejected

- **WHEN** a caller passes `design_system_ref=desing-system-2@2.0.0` and that version is in `lifecycle_state=proposed`
- **THEN** the platform MUST reject with `409 design_system_not_approved`

### Requirement: App carries optional per-component overrides

The App record SHALL include an optional `design_system_overrides` map: `{component_name: design_system_ref}`. The map MAY be empty. The map SHALL be readable on `GET /v1/apps/{id}` and mutable via `PATCH /v1/apps/{id}/design-system/overrides` (caller MUST have `app#owner`). Overrides SHALL be enforced at build time per the per-component overrides spec in `design-system-catalog`.

#### Scenario: Overrides default to empty

- **WHEN** a new App is created
- **THEN** `design_system_overrides` MUST equal `{}` unless an explicit map is supplied

#### Scenario: Owner sets overrides

- **GIVEN** an authenticated caller with `app#owner` on `app-1`
- **WHEN** the caller calls `PATCH /v1/apps/app-1/design-system/overrides` with `{button: desing-system-3@2.0.0}`
- **THEN** the platform MUST persist the override, validate `desing-system-3@2.0.0` is `approved` and reachable, and emit `app.design_system.override_changed.v1`

## MODIFIED Requirements

### Requirement: App CRUD API

The platform SHALL expose `POST /v1/workspaces/{ws}/apps`, `GET /v1/workspaces/{ws}/apps`, `GET /v1/apps/{id}`, `PATCH /v1/apps/{id}`, `POST /v1/apps/{id}:archive`, `POST /v1/apps/{id}:restore`, `DELETE /v1/apps/{id}`, **`POST /v1/apps/{id}/design-system:swap`**, **`PATCH /v1/apps/{id}/design-system/overrides`**. All endpoints SHALL enforce OpenFGA authorization against the App and its parent Workspace. Mutating endpoints SHALL emit the matching `app.*.v1` event and produce an immutable audit record.

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
