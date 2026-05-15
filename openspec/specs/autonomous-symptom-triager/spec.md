# autonomous-symptom-triager Specification

## Purpose
TBD - created by syncing change alfred-autonomous-platform-ops. Update Purpose after archive.

## Requirements

### Requirement: Triager is the sole producer of non-human agent-mode sessions

The platform SHALL run a single logical service `symptom-triager` that is the only producer of `AgentModeSession` records with `trigger_source ∈ {symptom, playbook}`. All other code paths SHALL be rejected by the agent-mode session API when `trigger_source != human`.

#### Scenario: Triager spawns a session

- **WHEN** the triager decides a symptom warrants action
- **THEN** it SHALL call `POST /v1/agent-mode/sessions` with `actor="system:alfred"`, `trigger_source="symptom"`, `symptom_id=<uuid>`, and the resolved autonomy preset
- **AND** the session API SHALL accept the request

#### Scenario: Non-triager service is denied

- **WHEN** any service other than `symptom-triager` calls `POST /v1/agent-mode/sessions` with `trigger_source="symptom"`
- **THEN** the session API SHALL return 403 with `code=forbidden_trigger_source`

### Requirement: Triage decision pipeline

For each consumed event, the triager SHALL evaluate, in this order, and act on the first match: (1) noise-rule active for fingerprint → drop; (2) circuit-breaker open for fingerprint → enqueue to HITL queue; (3) active session for fingerprint → adhere (coalesce); (4) pre-approved playbook matching fingerprint → spawn session with playbook-specific intent; (5) otherwise → spawn session with `"diagnose & fix <fingerprint>"` intent and the `diagnose-then-propose` autonomy preset.

#### Scenario: Noise rule short-circuits

- **WHEN** an event arrives with a fingerprint matched by an active `noise_rule`
- **THEN** the triager SHALL increment `triager.noise_silenced_total{fingerprint}` and not spawn a session

#### Scenario: Circuit breaker queues for HITL

- **WHEN** an event arrives for a fingerprint whose `circuit_breaker_state.open=true`
- **THEN** the triager SHALL insert a row into the HITL queue with the symptom payload and notify on-call
- **AND** SHALL NOT spawn an autonomous session

#### Scenario: Active session coalesces

- **WHEN** an event arrives for a fingerprint with an active (non-terminal) session
- **THEN** the triager SHALL append the symptom id to that session's `attached_symptoms` list
- **AND** SHALL NOT spawn a new session

#### Scenario: Playbook match spawns deterministic plan

- **WHEN** an event matches a registered playbook (by fingerprint pattern)
- **THEN** the triager SHALL spawn a session whose initial plan is the playbook's steps verbatim
- **AND** the session SHALL carry `playbook_id` in its metadata

#### Scenario: Novel fingerprint produces diagnostic-led session

- **WHEN** none of the prior rules match
- **THEN** the triager SHALL spawn a session with autonomy preset `diagnose-then-propose` and intent `"diagnose & fix <fingerprint>"`

### Requirement: Per-fingerprint circuit breaker

The triager SHALL maintain `circuit_breaker_state(fingerprint, opened_at, failed_session_count, cooldown_until)`. After N (default 3) consecutive sessions for the same fingerprint terminate with `failed | aborted`, the breaker SHALL open for a cooldown period (default 30min). While open, all events for that fingerprint route to HITL queue. The breaker SHALL close automatically at `cooldown_until` or via explicit admin reset through `platform-ops`.

#### Scenario: Three consecutive failures open the breaker

- **WHEN** three sessions in a row for the same fingerprint terminate with status `failed`
- **THEN** the breaker SHALL open with `cooldown_until = now() + 30min`
- **AND** subsequent events for that fingerprint SHALL queue to HITL until cooldown elapses

#### Scenario: Admin force-resets the breaker

- **WHEN** an admin calls the platform-ops endpoint to reset the breaker
- **THEN** the breaker SHALL close immediately and the reset action SHALL be audited

### Requirement: Per-hour budget and session caps

The triager SHALL enforce a per-hour cap on (a) LLM tokens spent on triager-initiated sessions and (b) number of sessions spawned. Defaults are global; per-tenant overrides MAY be configured. When either cap is reached, the triager SHALL stop spawning sessions but SHALL continue consuming the bus (symptoms queue to HITL) and SHALL page on-call.

#### Scenario: Token budget exhausted

- **WHEN** the running hour's LLM token usage from triager-spawned sessions exceeds the configured cap
- **THEN** the triager SHALL halt spawning, set metric `triager.budget_exhausted_total` += 1, and emit a high-severity audit event
- **AND** subsequent symptoms SHALL be queued to HITL with `reason=budget_exhausted`

### Requirement: Recovery event closes outstanding sessions

When an emitter publishes a recovery event (severity `info`, title `recovery`) for a fingerprint with an active session, the triager SHALL close that session with status `resolved_externally` and record the recovery symptom id on the session.

#### Scenario: Recovery arrives during a session

- **WHEN** a probe-recovery event arrives for `service:control-plane|signal:probe-failed` while a session is mid-execution
- **THEN** the triager SHALL request the session to stop at its next safe point
- **AND** SHALL mark the session `resolved_externally` if it had not already mutated state, otherwise leave the session running so verification can complete
