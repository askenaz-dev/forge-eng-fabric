## ADDED Requirements

### Requirement: Session accepts non-human trigger sources

The session model SHALL include a required field `trigger_source ∈ {human, symptom, playbook, replan}`. When `trigger_source ∈ {symptom, playbook}` the session's `actor` SHALL be `system:alfred` and a non-null `symptom_id` SHALL be persisted on the session. When `trigger_source = replan` the session SHALL link to the parent session via `parent_session_id`.

#### Scenario: Symptom-triggered session links to its symptom

- **WHEN** the `symptom-triager` calls `POST /v1/agent-mode/sessions` with `actor="system:alfred"`, `trigger_source="symptom"`, `symptom_id=<uuid>`
- **THEN** the session SHALL be created with that actor and symptom_id, and the audit row SHALL include both fields

#### Scenario: Human trigger remains the default for openspec-rooted sessions

- **WHEN** a user calls `POST /v1/agent-mode/sessions` with `{workspace_id, openspec_id}` (no trigger_source field)
- **THEN** the session SHALL default `trigger_source="human"` and require a human-bearer token

#### Scenario: Non-triager callers cannot start non-human sessions

- **WHEN** any caller other than `symptom-triager` calls the session API with `trigger_source="symptom"`
- **THEN** the API SHALL return 403 with `code=forbidden_trigger_source`

### Requirement: Per-step pre-validate and post-validate hooks are mandatory

Every step of kind `tool`, `workflow` or `agent` that is reached via a non-human trigger SHALL declare an `expected_outcome` probe alongside its action descriptor. The executor SHALL refuse to dispatch such steps without a probe. After the step's underlying call returns, the executor SHALL invoke the probe and propagate its result into the step's `outcome` field.

#### Scenario: Step without expected_outcome is rejected for autonomous execution

- **WHEN** the executor is about to dispatch a step in a session whose `trigger_source != human` and the step lacks `expected_outcome`
- **THEN** the executor SHALL mark the step `failed` with `reason:"missing_expected_outcome"`, request the planner to replan with a probe, and emit `alfred.agent_mode.step_missing_probe.v1`

#### Scenario: Post-validate failure triggers rollback or replan

- **WHEN** a step's `expected_outcome` probe fails after the action completes
- **THEN** the executor SHALL consult the action's `reversibility`: trivial/easy → rollback automatically; hard/irreversible → pause for HITL with full context

### Requirement: Budget probe gains per-fingerprint and per-hour aggregates

`BudgetProbe` SHALL track LLM token usage and session counts aggregated by `(fingerprint, hour_window)` and `(tenant_id, hour_window)`. The triager and executor SHALL be able to query these aggregates and enforce caps before spawning new sessions or expanding existing plans.

#### Scenario: Per-fingerprint aggregate is queryable

- **WHEN** the triager queries `BudgetProbe.usage(fingerprint=X, window="1h")`
- **THEN** it SHALL receive `{tokens, sessions, last_session_at}` covering all sessions tagged with that fingerprint in the rolling hour

#### Scenario: Per-hour cap halts session spawning

- **WHEN** the triager queries the per-hour aggregate and the result equals or exceeds the configured cap
- **THEN** the triager SHALL halt spawning, increment `triager.budget_exhausted_total`, and emit a high-severity audit event

## MODIFIED Requirements

### Requirement: Agent-mode session is a first-class, long-running object

Alfred SHALL expose an `agent_mode_session` resource that represents one autonomous orchestration run, started either by a human against a committed `openspec_id` or by the `symptom-triager` against a `symptom_id`. The session SHALL hold a structured plan, an ordered list of executed steps, the current status (`planning`, `running`, `paused_for_approval`, `completed`, `aborted`, `failed`, `resolved_externally`), the active workspace's resolved `autonomy_policy`, a `trigger_source ∈ {human, symptom, playbook, replan}`, an `actor` (`<user-sub> | system:alfred`), an `actor_session` sub-principal (`system:alfred:session:<uuid>` when applicable), optional `symptom_id` / `playbook_id` / `parent_session_id`, and a stable `correlation_id` propagated to every downstream call.

#### Scenario: Authorized user starts an agent-mode session

- **WHEN** an authorized user calls `POST /v1/agent-mode/sessions` with `{ workspace_id, openspec_id }` and the workspace has `alfred:agent-mode.run` set to `autonomous` or `requires_approval`
- **THEN** Alfred SHALL create a session with `trigger_source="human"` and `actor=<user-sub>`, resolve the workspace's active `autonomy_policy`, produce an initial plan grounded in the OpenSpec and its linked artifacts, persist the session row plus an `alfred.agent_mode.session_started.v1` audit event, and return the `session_id` along with the first plan revision

#### Scenario: Triager starts a symptom-driven session

- **WHEN** the `symptom-triager` calls `POST /v1/agent-mode/sessions` with `{ workspace_id?, symptom_id, actor:"system:alfred", trigger_source:"symptom" }`
- **THEN** Alfred SHALL create a session with `actor=system:alfred`, `actor_session=system:alfred:session:<uuid>` (capabilities = intersection of standing grants and symptom-justified capabilities), and emit `alfred.agent_mode.session_started.v1` with `triggered_by:symptom_id`

#### Scenario: Session survives Alfred process restart

- **WHEN** Alfred restarts while a session is `running` or `paused_for_approval`
- **THEN** on next read of `GET /v1/agent-mode/sessions/{id}` the session SHALL report its persisted status, the last completed step, and the next planned step
- **AND** clients reconnecting to the SSE stream SHALL receive a replay of all decision records since `last_event_id` before receiving live events

#### Scenario: Recovery symptom resolves an open session

- **WHEN** the triager closes a session with status `resolved_externally` because a recovery symptom arrived
- **THEN** the session SHALL persist the resolving `symptom_id` and any in-flight steps SHALL be marked `skipped_resolved`
