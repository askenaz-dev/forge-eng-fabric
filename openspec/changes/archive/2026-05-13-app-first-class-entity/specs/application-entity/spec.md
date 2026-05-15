## ADDED Requirements

### Requirement: Application aggregate

The platform SHALL model **Application** (henceforth "App") as a first-class entity sitting between Workspace and OpenSpec in the hierarchy `Tenant → Workspace → App → Specs[]`. Every App SHALL have a stable UUID `id`, a `slug` unique within its parent Workspace, a human `name`, a `description`, at least one `owner`, a `lifecycle_state` (`active|archived|deleted`), an optional `design_system_ref`, an optional `default_environments[]`, a `repo_links[]` array, a `runtime_links[]` array, a `workspace_id` and a `tenant_id`. Every OpenSpec SHALL reference exactly one App through `app_id`.

#### Scenario: App is created with all required fields

- **GIVEN** an authenticated user with `workspace.member` and `workspace:create-app` permission
- **WHEN** the user calls `POST /v1/workspaces/{ws}/apps` with `{name, slug, description, owners:[user-1]}`
- **THEN** the platform MUST persist an App with `id=<uuid>`, the supplied fields, `workspace_id=ws`, `tenant_id` derived from the workspace, `lifecycle_state=active`
- **AND** emit `app.created.v1` with the full App body and `correlation_id`
- **AND** seed an OpenFGA `app#owner` tuple for `user-1`

#### Scenario: Slug uniqueness scoped to workspace

- **GIVEN** an existing App `hr-portal` in `workspace=ws-1`
- **WHEN** a user attempts to create another App with `slug=hr-portal` in `ws-1`
- **THEN** the platform MUST reject the request with `409 app_slug_conflict`
- **AND** the same slug `hr-portal` SHALL still be creatable in `workspace=ws-2`

#### Scenario: OpenSpec requires app_id

- **WHEN** any caller attempts to persist a new OpenSpec without `app_id`
- **THEN** the platform MUST reject the persistence with `422 missing_app_scope`

### Requirement: App CRUD API

The platform SHALL expose `POST /v1/workspaces/{ws}/apps`, `GET /v1/workspaces/{ws}/apps`, `GET /v1/apps/{id}`, `PATCH /v1/apps/{id}`, `POST /v1/apps/{id}:archive`, `POST /v1/apps/{id}:restore`, `DELETE /v1/apps/{id}`. All endpoints SHALL enforce OpenFGA authorization against the App and its parent Workspace. Mutating endpoints SHALL emit the matching `app.*.v1` event and produce an immutable audit record.

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

### Requirement: System-managed `_unassigned` App per workspace

Each Workspace SHALL have exactly one App with `slug=_unassigned`, owned by the platform service identity. The `_unassigned` App SHALL be created automatically when the Workspace is created or, for existing Workspaces, by the migration job. It SHALL be read-only via the public CRUD API (PATCH, DELETE, archive SHALL all return `403 system_managed_app`). OpenSpecs MAY be re-parented out of `_unassigned` but new OpenSpecs SHALL NOT be assigned to `_unassigned` via the public API (the migration job is the only writer).

#### Scenario: Workspace bootstrap creates the unassigned bucket

- **WHEN** a new Workspace is provisioned
- **THEN** the bootstrap pipeline MUST create the `_unassigned` App with `lifecycle_state=active`, the platform service identity as owner, and `description="System-managed bucket for unassigned specs"`

#### Scenario: Manual modification of unassigned is refused

- **WHEN** any user calls `PATCH /v1/apps/{id}` where `slug=_unassigned`
- **THEN** the platform MUST respond `403 system_managed_app`

#### Scenario: Re-parenting out of unassigned is allowed

- **GIVEN** an OpenSpec `spec-7` currently scoped to `_unassigned`
- **WHEN** a workspace editor calls the re-parent API with a target `app_id=app-real`
- **THEN** the platform MUST update `spec-7.app_id=app-real`, emit `spec.reparented.v1` and audit the action

### Requirement: OpenFGA model fragment for App

The OpenFGA model SHALL include a new type `app` with relations `owner`, `editor`, `viewer`, `parent`. `parent` SHALL be a direct tuple to the parent `workspace`. `viewer` SHALL be computed as `direct | editor | owner | parent.viewer`. `editor` SHALL be computed as `direct | owner | parent.editor`. `owner` SHALL be direct only. Workspace tuples SHALL remain unchanged.

#### Scenario: Workspace viewer inherits app viewer

- **GIVEN** `user-1` is `workspace#viewer` of `ws-1` and `app-1.parent = ws-1`
- **WHEN** OpenFGA is queried for `app#viewer(app-1, user-1)`
- **THEN** the answer MUST be `allowed=true`

#### Scenario: Explicit app override grants editor without workspace access

- **GIVEN** `user-2` has no relation to `ws-1` but has a direct `app#editor` tuple on `app-1`
- **WHEN** OpenFGA is queried for `app#editor(app-1, user-2)`
- **THEN** the answer MUST be `allowed=true`
- **AND** `workspace#editor(ws-1, user-2)` MUST remain `allowed=false`

### Requirement: App lifecycle CloudEvents

The platform SHALL emit immutable CloudEvents on App lifecycle transitions: `app.created.v1`, `app.updated.v1`, `app.archived.v1`, `app.restored.v1`, `app.deleted.v1`. Each event SHALL carry the App's full body before and after the change (`before`, `after`), the acting principal, the `correlation_id` and the workspace context.

#### Scenario: Update event carries diff

- **WHEN** an App is updated via PATCH
- **THEN** the emitted `app.updated.v1` event MUST include `before` and `after` blocks reflecting the prior and current App body

### Requirement: Spec re-parenting event

When an OpenSpec is re-parented from one App to another, the platform SHALL emit `spec.reparented.v1` with `{spec_id, from_app_id, to_app_id, principal, reason, correlation_id}`. The migration job and the re-parent API SHALL be the only emitters.

#### Scenario: Migration emits re-parent event per spec

- **WHEN** the migration job re-anchors `spec-7` from `_unassigned` to `app-real`
- **THEN** exactly one `spec.reparented.v1` event MUST be emitted with `from_app_id=<unassigned id>`, `to_app_id=app-real`, `reason=migration`
