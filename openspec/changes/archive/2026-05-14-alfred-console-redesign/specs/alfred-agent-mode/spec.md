## MODIFIED Requirements

### Requirement: Agent-mode session is a first-class, long-running object

Alfred SHALL expose an `agent_mode_session` resource that represents one autonomous orchestration run from a committed `openspec_id` toward a deployed application. The session SHALL hold a structured plan, an ordered list of executed steps, the current status (`planning`, `running`, `paused_for_approval`, `completed`, `aborted`, `failed`), the active workspace's resolved `autonomy_policy`, the optional `start_step` hint, and a stable `correlation_id` propagated to every downstream call.

#### Scenario: Authorized user starts an agent-mode session

- **WHEN** an authorized user calls `POST /v1/agent-mode/sessions` with `{ workspace_id, openspec_id }` and the workspace has `alfred:agent-mode.run` set to `autonomous` or `requires_approval`
- **THEN** Alfred SHALL create a session, resolve the workspace's active `autonomy_policy`, produce an initial plan grounded in the OpenSpec and its linked artifacts, persist the session row plus an `alfred.agent_mode.session_started.v1` audit event, and return the `session_id` along with the first plan revision

#### Scenario: Session survives Alfred process restart

- **WHEN** Alfred restarts while a session is `running` or `paused_for_approval`
- **THEN** on next read of `GET /v1/agent-mode/sessions/{id}` the session SHALL report its persisted status, the last completed step, and the next planned step
- **AND** clients reconnecting to the SSE stream SHALL receive a replay of all decision records since `last_event_id` before receiving live events

#### Scenario: start_step jumps to architect for committed spec

- **GIVEN** a committed `spec-7`
- **WHEN** the UI calls `POST /v1/agent-mode/sessions` with `{ workspace_id, openspec_id: spec-7, start_step: "architect" }`
- **THEN** Alfred SHALL skip discovery and the wizard-driven planning, build a plan starting from the architect step, and emit `alfred.agent_mode.session_started.v1` with `start_step=architect`
- **AND** the session SHALL refuse to start with `start_step=architect` if `spec-7.lifecycle_state` is not in `{approved, committed}`, returning `409 spec_not_ready_for_architect`

## ADDED Requirements

### Requirement: start_step hint accepted by the agent-mode session API

The `POST /v1/agent-mode/sessions` request SHALL accept an optional `start_step` field whose value is one of the documented step kinds for the SDLC workflow (e.g., `discovery`, `architect`, `design`, `test`, `iac`, `deploy`). Alfred SHALL build the initial plan such that step 0 corresponds to the requested `start_step`. If the spec lifecycle does not permit jumping to that step, Alfred SHALL refuse with `409 spec_not_ready_for_step`.

#### Scenario: Unknown start_step rejected

- **WHEN** a caller submits `start_step="nope"`
- **THEN** Alfred MUST reject with `422 unknown_start_step` and include the allowed values

#### Scenario: start_step=architect for non-committed spec refused

- **GIVEN** `spec-7.lifecycle_state=proposed`
- **WHEN** a caller submits `start_step=architect`
- **THEN** Alfred MUST reject with `409 spec_not_ready_for_architect`
