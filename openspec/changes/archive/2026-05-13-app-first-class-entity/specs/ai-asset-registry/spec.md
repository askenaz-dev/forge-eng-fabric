## ADDED Requirements

### Requirement: Assets carry their producing App scope

Every asset record produced by an App SHALL include `app_id` in addition to `workspace_id`. The Registry SHALL accept `app_id` on `POST /v1/workspaces/{ws}/assets`, on the lifecycle hooks and on every PATCH. Asset lists SHALL be filterable by `app_id`. When an asset is published from outside an App context (e.g., platform-level shared skills), `app_id` MAY be null and the asset MUST be flagged `scope=workspace`.

#### Scenario: Assets created with app scope

- **GIVEN** an authenticated user with `app#editor` on `app-1`
- **WHEN** the user calls `POST /v1/workspaces/ws-1/assets` with `app_id=app-1, type=agent-skill, ...`
- **THEN** the Registry MUST persist the asset with `app_id=app-1, workspace_id=ws-1`
- **AND** include `app_id` on the publication event payload

#### Scenario: Filter assets by App

- **WHEN** a caller calls `GET /v1/workspaces/ws-1/assets?app_id=app-1`
- **THEN** the response MUST list only assets whose `app_id=app-1`
- **AND** the response MUST NOT include workspace-scoped (null `app_id`) assets unless an explicit `include_workspace_scope=true` is passed

#### Scenario: Reject app_id from another workspace

- **WHEN** a caller calls `POST /v1/workspaces/ws-1/assets` with `app_id=app-2` where `app-2.workspace_id=ws-2`
- **THEN** the Registry MUST reject the request with `403 cross_workspace_app_reference`

### Requirement: App archival cascades to asset visibility

When an App is archived, the Registry SHALL mark every asset with `app_id=<archived>` as `discoverable=false` for new consumers, while preserving invocability for existing dependents. When the App is restored, discoverability SHALL be restored.

#### Scenario: Archive cascade

- **GIVEN** an App `app-1` with 3 approved assets
- **WHEN** an owner calls `POST /v1/apps/app-1:archive`
- **THEN** the Registry MUST set `discoverable=false` on the 3 assets within the same transaction (or compensate on failure)
- **AND** emit `asset.discoverability.changed.v1` for each
