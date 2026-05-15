## ADDED Requirements

### Requirement: App carries SDLC `targets` map

Every App SHALL carry a `targets` JSONB map declaring the per-phase delivery policy for SDLC workflows. The map SHALL contain at minimum the keys `architect`, `design`, `development`, `qa`, `security`, `devops`, `iac`, `sre`, `finops`, `observability`. Allowed values per key are `required`, `optional`, `opt-in`, `skipped`. The platform SHALL initialise new Apps with the following defaults:

- `architect: required`
- `design: optional`
- `development: required`
- `qa: required`
- `security: required`
- `devops: required`
- `iac: opt-in`
- `sre: optional`
- `finops: opt-in`
- `observability: opt-in`

#### Scenario: Defaults applied on App creation

- **WHEN** a new App is created via `POST /v1/workspaces/{ws}/apps`
- **THEN** the platform MUST initialise `targets` with the documented defaults
- **AND** the App record MUST expose `targets` on every subsequent `GET /v1/apps/{id}`

#### Scenario: Patch updates targets and audits the change

- **GIVEN** an App `app-1` and a caller with `app#owner`
- **WHEN** the caller calls `PATCH /v1/apps/app-1` with `{targets: {iac: required}}`
- **THEN** the App MUST be updated with `targets.iac=required` (other keys preserved)
- **AND** an `app.updated.v1` event MUST be emitted carrying `before.targets` and `after.targets`
- **AND** the audit record MUST list the diff

#### Scenario: Invalid target value rejected

- **WHEN** a caller patches `targets.qa=auto` (not in the allowed set)
- **THEN** the platform MUST reject with `422 invalid_target_value` listing the allowed set

### Requirement: Per-spec target override

An individual OpenSpec MAY override the App-level `targets` for the duration of its workflow run via a `targets_override` block in the spec. The override SHALL be merged on top of the App-level map at workflow start time. The override MUST NOT make a phase *more* permissive than the App's policy ceiling — specifically, `required` SHALL NOT be relaxed to `optional` or `skipped` from an override; only the other direction is allowed.

#### Scenario: Spec override tightens a phase

- **GIVEN** an App with `targets.iac=opt-in` and a spec with `targets_override.iac=required`
- **WHEN** the workflow runs against this spec
- **THEN** the merged plan MUST treat `iac` as `required`
- **AND** the override MUST be recorded in the run's audit trail

#### Scenario: Override attempt to relax required rejected

- **GIVEN** an App with `targets.security=required`
- **WHEN** a spec submits `targets_override.security=optional`
- **THEN** the orchestrator MUST reject the workflow start with `409 cannot_relax_required_phase`
