# platform-ops-service Specification

## Purpose
TBD - created by syncing change alfred-autonomous-platform-ops. Update Purpose after archive.

## Requirements

### Requirement: Dedicated service exposes every autonomous action

The platform SHALL run a service `platform-ops` that hosts every endpoint Alfred may invoke autonomously on the platform or on apps it monitors. Alfred (and any caller acting as `system:alfred`) SHALL have no generic shell, kubectl, psql, or docker tool exposed. All capabilities are expressed as narrow HTTP endpoints in this one service.

#### Scenario: Service exists and is the sole autonomous-action surface

- **WHEN** the FGA model is inspected for grants held by `system:alfred`
- **THEN** all autonomous-action capabilities SHALL resolve to endpoints under `https://platform-ops/v1/`
- **AND** no FGA grants SHALL exist for arbitrary code execution, raw kubectl, or unscoped database access

#### Scenario: Missing capability is proposed, not bypassed

- **WHEN** Alfred needs to perform an action without a covering endpoint
- **THEN** it SHALL invoke `POST /v1/code-fixes/open-pr` proposing a new platform-ops endpoint with its OPA policy entry and verification probe
- **AND** SHALL NOT attempt to perform the action via any other tool

### Requirement: Endpoint contract: input validation, OPA pre-check, action, verification, audit, response

Every mutating endpoint in `platform-ops` SHALL execute the following pipeline in order, atomically wrapped in audit transaction: (1) validate inputs against its declared schema; (2) call OPA `risk-classifier.rego` with the action descriptor; (3) if OPA decision permits, execute the action; (4) execute the declared `expected_outcome` probe; (5) if probe fails and action is reversible, attempt rollback; (6) write `audit_event` with action descriptor, OPA decision bundle hash, outcome, rollback record (if any); (7) respond with structured JSON including correlation_id and audit_event_id.

#### Scenario: Successful path

- **WHEN** an authorised caller invokes `POST /v1/services/registry/restart` with valid inputs
- **THEN** the endpoint SHALL validate inputs, call OPA, restart the service, wait for the declared health probe, and on success return 200 with `{audit_event_id, correlation_id, outcome:"verified"}`

#### Scenario: Verification fails and rollback succeeds

- **WHEN** an endpoint performs a reversible action whose post-probe fails
- **THEN** the endpoint SHALL execute the inverse action, write a single audit row with `outcome:"rolled_back"` and the rollback details, and respond 502 with structured error including the failed probe output

#### Scenario: OPA denies

- **WHEN** OPA returns `autonomy_decision = deny`
- **THEN** the endpoint SHALL return 403 without performing the action and SHALL write an audit row with `outcome:"denied_by_policy"` and the policy bundle hash

### Requirement: Required endpoint families

`platform-ops` SHALL expose at least the following endpoint families, with each endpoint declaring its action class, blast radius, reversibility, and expected-outcome probe in its OpenAPI metadata:

- `/v1/services/{name}/{restart|scale|cordon|uncordon}` — `mutate-runtime`, blast=single-service
- `/v1/migrations/{run|dry-run|rollback}` — `mutate-data`, blast=single-service
- `/v1/secrets/{key}/rotate` — `mutate-config`, blast=workspace
- `/v1/feature-flags/{key}/toggle` — `mutate-config`, blast=workspace
- `/v1/noise-rules/{propose|approve|revoke}` — `mutate-config`, blast=platform
- `/v1/circuit-breakers/{fingerprint}/reset` — `mutate-runtime`, blast=platform
- `/v1/code-fixes/open-pr` — `mutate-code`, blast=app
- `/v1/sandbox/{spawn|run|destroy}` — `mutate-infra`, blast=process (sandbox itself)
- `/v1/diagnostics/{probe|inspect|logs}` — `diagnostic`, no mutation

#### Scenario: Endpoint declares its OPA inputs

- **WHEN** any platform-ops endpoint receives a request
- **THEN** it SHALL construct an OPA input document containing `action_class`, `blast_radius`, `reversibility`, `scope.tenant_id`, `scope.workspace_id`, `scope.asset_id`, `actor`, `trigger_source` and pass it to the policy
- **AND** the OPA bundle hash SHALL be persisted on the audit row for that action

### Requirement: Inverse endpoint or compensating action for reversible classes

Every endpoint marked `reversibility ∈ {trivial, easy}` SHALL expose, or be served by, an inverse endpoint that can be invoked with the original `audit_event_id` to undo the action. Irreversible actions SHALL NOT advertise an inverse and SHALL require dual approval.

#### Scenario: Restart has trivial reversibility

- **WHEN** `POST /v1/services/{name}/restart` succeeds with `audit_event_id=X`
- **THEN** `POST /v1/services/{name}/restart?revert=X` SHALL be a no-op idempotent confirmation (restart already returned the service to running)
- **AND** the audit chain SHALL link both rows via `correlation_id`

#### Scenario: Migration rollback exists

- **WHEN** `POST /v1/migrations/run` succeeds for migration `0007_add_xyz`
- **THEN** `POST /v1/migrations/rollback` with the same migration id SHALL be available and produce a documented down-migration result

### Requirement: Service hosts the OPA bundle hash in audit and responses

Every response from a `platform-ops` mutating endpoint SHALL include `policy_bundle_hash` matching the value persisted on the audit row, enabling clients and audit reviewers to reconcile which policy version permitted the action.

#### Scenario: Bundle hash on response

- **WHEN** any mutating endpoint returns success
- **THEN** the response body SHALL include `policy_bundle_hash` (sha256, hex)
- **AND** the corresponding `audit_event` row SHALL contain the same hash
