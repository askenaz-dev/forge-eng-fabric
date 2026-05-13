## ADDED Requirements

### Requirement: Gateway-sourced invocations are first-class

The asset-observability service SHALL accept `com.forge.gateway.invocation.v1` and `com.forge.gateway.installed.v1` events from the Kafka bus and SHALL aggregate them into the existing per-asset metric series with `source=gateway`, alongside the existing `source=runtime` and `source=workflow` series. Queries that omit the `source` filter SHALL include both internal and gateway traffic.

#### Scenario: Mixed-source rollup

- **GIVEN** an asset that received 100 invocations from the workflow-runtime and 40 invocations through the gateway in the last hour
- **WHEN** a caller queries `GET /v1/assets/{id}/metrics?range=1h`
- **THEN** `totals.invocations` is `140`
- **AND** `by_source.gateway.invocations` is `40`
- **AND** `by_source.workflow.invocations` is `100`

#### Scenario: Source filter narrows results

- **WHEN** the same query is made with `?source=gateway`
- **THEN** `totals.invocations` is `40`

### Requirement: Install metric per asset

The service SHALL maintain an `installs` series per asset capturing total installs, active installs (last seen within 30 days), per-client breakdown and the latest installed-version distribution. Installs SHALL be derived from `com.forge.gateway.installed.v1` keyed by `(developer_sub, asset_id, client)`.

#### Scenario: Repeated install is one active install

- **GIVEN** the same developer installs `foo@1.0.0` then `foo@1.1.0` on Claude Code within the same week
- **THEN** the `installs.active` for `foo` increases by exactly 1
- **AND** `installs.by_version` records both versions but only `1.1.0` as the current install for that developer

### Requirement: Drift detection includes gateway traffic

Eval-drift detection SHALL consider gateway-sourced eval samples on equal footing with runtime samples. The drift alert threshold SHALL remain unchanged but its baseline SHALL be computed over the union of sources.

#### Scenario: Gateway-only regression triggers drift

- **GIVEN** an asset whose internal eval scores remain stable but whose gateway eval scores drop by more than the configured threshold over the last 24h
- **WHEN** the drift detector runs
- **THEN** a `drift_alert=true` is recorded on the asset's metrics
- **AND** the alert payload identifies `source=gateway` as the contributing series
