## ADDED Requirements

### Requirement: Every mutating action declares an expected outcome

Each action invoked through `platform-ops` SHALL carry an `expected_outcome` document describing a deterministic probe whose success is the post-condition of the action. The probe SHALL be one of: HTTP probe (URL + matchers on status/body/headers), TCP probe (host:port reachable), SQL probe (query + expected row predicate), shell-equivalent (command + expected exit code, executed inside a sandbox), Prometheus probe (PromQL + expected value range). An action with no valid `expected_outcome` SHALL NOT be eligible for autonomous execution.

#### Scenario: Action with explicit HTTP probe

- **WHEN** Alfred invokes `POST /v1/services/registry/restart` with `expected_outcome:{http:{url:"http://registry/healthz", status:200}}`
- **THEN** the endpoint SHALL accept and persist the probe with the audit row

#### Scenario: Action without expected_outcome is rejected for autonomy

- **WHEN** an autonomous session attempts an action without a valid `expected_outcome`
- **THEN** the endpoint SHALL respond 400 with `code=missing_expected_outcome`
- **AND** the calling session SHALL transition to `paused_for_approval` requesting a human declare the probe

### Requirement: Post-action probe is enforced server-side

After an action mutates state, the endpoint SHALL execute the `expected_outcome` probe with a bounded timeout (default 30s). The outcome SHALL be recorded as one of: `verified`, `verification_failed`, `verification_timeout`.

#### Scenario: Probe passes

- **WHEN** post-action probe returns the expected response
- **THEN** the endpoint SHALL record `outcome:"verified"` on the audit row and respond 200

#### Scenario: Probe times out

- **WHEN** the probe does not return within its timeout
- **THEN** the endpoint SHALL record `outcome:"verification_timeout"`, attempt rollback if reversible, and respond 504

### Requirement: Rollback on verification failure when reversible

When `outcome ∈ {verification_failed, verification_timeout}` and the action's declared `reversibility ∈ {trivial, easy}`, the endpoint SHALL automatically invoke the inverse and record the rollback in the same audit transaction. When `reversibility ∈ {hard, irreversible}`, the endpoint SHALL NOT roll back and SHALL escalate to HITL with full context.

#### Scenario: Reversible failure rolls back automatically

- **WHEN** a `mutate-config` action with `reversibility:"easy"` fails verification
- **THEN** the endpoint SHALL execute the inverse, record `outcome:"rolled_back"`, and respond 502
- **AND** Alfred's planner SHALL receive the structured failure and decide whether to replan or abort

#### Scenario: Irreversible failure escalates

- **WHEN** a `mutate-data` action with `reversibility:"irreversible"` fails verification
- **THEN** the endpoint SHALL NOT attempt rollback
- **AND** SHALL create an HITL ticket linked to the audit row with severity=critical, paging on-call

### Requirement: Per-fingerprint failure tracking feeds circuit breaker

Every verification failure SHALL increment `circuit_breaker_state(fingerprint).failed_session_count`. The triager's circuit breaker (see autonomous-symptom-triager) uses this counter to open the breaker once it reaches its threshold.

#### Scenario: Failures contribute to breaker

- **WHEN** three sessions for the same fingerprint each end with `verification_failed`
- **THEN** `circuit_breaker_state` SHALL register three failures for that fingerprint
- **AND** the breaker SHALL transition to `open`

### Requirement: Verification artifact persists alongside audit

Each verification result SHALL persist `{probe_definition, probe_result, started_at, ended_at}` linked to the audit row. The portal "Autonomous activity" view SHALL render this artifact so reviewers can see exactly what was checked and why it passed or failed.

#### Scenario: Reviewer inspects a verification artifact

- **WHEN** an admin opens an autonomous-action audit row
- **THEN** the portal SHALL display the probe definition, the actual response captured, and the diff against the expected matcher
