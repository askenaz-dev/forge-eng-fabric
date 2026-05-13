## ADDED Requirements

### Requirement: Agent-mode session is a first-class, long-running object

Alfred SHALL expose an `agent_mode_session` resource that represents one autonomous orchestration run from a committed `openspec_id` toward a deployed application. The session SHALL hold a structured plan, an ordered list of executed steps, the current status (`planning`, `running`, `paused_for_approval`, `completed`, `aborted`, `failed`), the active workspace's resolved `autonomy_policy`, and a stable `correlation_id` propagated to every downstream call.

#### Scenario: Authorized user starts an agent-mode session

- **WHEN** an authorized user calls `POST /v1/agent-mode/sessions` with `{ workspace_id, openspec_id }` and the workspace has `alfred:agent-mode.run` set to `autonomous` or `requires_approval`
- **THEN** Alfred SHALL create a session, resolve the workspace's active `autonomy_policy`, produce an initial plan grounded in the OpenSpec and its linked artifacts, persist the session row plus an `alfred.agent_mode.session_started.v1` audit event, and return the `session_id` along with the first plan revision

#### Scenario: Session survives Alfred process restart

- **WHEN** Alfred restarts while a session is `running` or `paused_for_approval`
- **THEN** on next read of `GET /v1/agent-mode/sessions/{id}` the session SHALL report its persisted status, the last completed step, and the next planned step
- **AND** clients reconnecting to the SSE stream SHALL receive a replay of all decision records since `last_event_id` before receiving live events

### Requirement: Plan-driven execution across the intent-to-deploy chain

An agent-mode session SHALL execute a plan whose steps map to the intent-to-deploy chain — at minimum: scaffold repository, open PR, drive CI to green, request HITL approval at policy-required gates, deploy to the configured runtime, post-deploy verification. Each step SHALL be one of: `tool` (MCP/skill invocation), `workflow` (workflow-runtime trigger), `agent` (sub-agent delegation), or `approval` (HITL pause).

#### Scenario: Session executes steps in the reference order

- **WHEN** a session runs against a workspace with `full-autonomy` preset and the intent does not touch production
- **THEN** Alfred SHALL execute, in order: a `scaffold` workflow step, a `pr.open` tool step, a `ci.wait_for_green` step, a `deploy.staging` step, a `verify` step
- **AND** each step SHALL produce a decision record linked to the session and emit `alfred.agent_mode.step_completed.v1` with `step_idx`, `kind`, `tool_id`, `outcome` and `correlation_id`

#### Scenario: Plan is regenerated when a step fails recoverably

- **WHEN** a step fails with a recoverable error (e.g., CI red on a fixable lint issue)
- **THEN** Alfred SHALL append a new plan revision that includes a `fix` step before the failed step, increment the plan revision counter, and continue execution without operator intervention
- **AND** the plan revision SHALL be retrievable via `GET /v1/agent-mode/sessions/{id}?revision=<n>`

### Requirement: HITL approval pause is mandatory at policy-required gates

When a planned step's `action_class` resolves to `requires_approval` or `requires_dual_control` under the workspace's `autonomy_policy`, the session SHALL pause, open an approval request through the existing approvals service, and refuse to advance until the request resolves. Cancellation of the approval SHALL abort the session.

#### Scenario: Pre-prod deploy pauses for HITL even in full-autonomy

- **WHEN** a session reaches a `deploy:prod` step and the workspace's preset enforces `deploy:prod = requires_approval`
- **THEN** Alfred SHALL move the session to `paused_for_approval`, open an approval request with the OpenSpec, the proposed artifact and runtime target, emit `alfred.agent_mode.paused_for_approval.v1`, and stop dispatching subsequent steps
- **AND** on approval the session SHALL resume from the paused step and emit `alfred.agent_mode.resumed.v1`

#### Scenario: Approval rejection aborts the session

- **WHEN** an approver rejects the request
- **THEN** the session SHALL transition to `aborted`, write a final decision record with `outcome=aborted_by_approver` and the approver's rationale, emit `alfred.agent_mode.aborted.v1`, and refuse further `POST /messages` calls

### Requirement: Sub-agent delegation respects policy and budget

A session step of kind `agent` SHALL invoke a registered specialized agent (e.g., security review, threat model, postmortem) through the existing Asset Registry. Delegation SHALL be policy-checked, audited, and constrained by the same LiteLLM tenant budget that bounds the parent session.

#### Scenario: Security-impacting OpenSpec triggers Security Agent delegation

- **WHEN** the OpenSpec is classified `security-impacting` and an `approved` Security Agent exists in the registry
- **THEN** the session plan SHALL include a `delegate:security-review` step that runs the agent against the OpenSpec
- **AND** the delegated run's decisions SHALL be linked back to the parent `agent_mode_session` and counted against the same correlation chain

#### Scenario: Budget exhaustion halts further LLM-bound steps

- **WHEN** the LiteLLM tenant budget probe returns `over_budget` mid-session
- **THEN** Alfred SHALL pause the session with `status=paused_for_budget`, emit `alfred.agent_mode.paused_for_budget.v1`, and require an admin top-up or admin resume before continuing

### Requirement: Live session stream for the dock

Alfred SHALL expose `GET /v1/agent-mode/sessions/{id}/stream` as a Server-Sent Events endpoint. Events SHALL include `step_started`, `step_completed`, `plan_revised`, `paused_for_approval`, `resumed`, `completed`, `aborted`, `failed`, plus periodic `heartbeat`. The stream SHALL support resumption via `Last-Event-ID` against the durable decision log so a reconnecting client never misses an event.

#### Scenario: Late joiner receives full replay then live events

- **WHEN** a client subscribes with a `Last-Event-ID` older than the most recent step
- **THEN** the server SHALL emit each missed decision record in order before any new events, with monotonic event IDs

#### Scenario: Idle stream sends heartbeats

- **WHEN** no domain event has been produced for 15 seconds
- **THEN** the server SHALL emit a `heartbeat` event so the client can detect a stalled run distinctly from a still-working one

### Requirement: Follow-up intent during a running session

While a session is `running` or `paused_for_approval`, the authorized user SHALL be able to `POST /v1/agent-mode/sessions/{id}/messages` with a follow-up intent that Alfred MAY incorporate into the next plan revision without aborting the run. Follow-ups SHALL be audited and bounded by the same autonomy ceiling as the original session.

#### Scenario: Follow-up adds a step without aborting

- **WHEN** the user sends `"also bump the runtime image to node:20"` while the session is between steps
- **THEN** Alfred SHALL produce a plan revision that inserts the requested change before the next deploy step, audit the follow-up with `alfred.agent_mode.follow_up_received.v1`, and continue
- **AND** if the follow-up would cross a ceiling (e.g., asks Alfred to skip a required approval) Alfred SHALL reject it with `autonomy.override.rejected.v1` and leave the session unchanged

### Requirement: Session cancellation by the originator

The user who started a session, or any user with `alfred:agent-mode.cancel`, SHALL be able to cancel a running session. Cancellation SHALL stop dispatching new steps, allow the current in-flight step to finalize, and persist a terminal status of `aborted` with the cancelling principal recorded.

#### Scenario: Cancel mid-run

- **WHEN** the originator calls `POST /v1/agent-mode/sessions/{id}/cancel`
- **THEN** no further steps SHALL be dispatched, the in-flight step SHALL run to completion or timeout, and the session transitions to `aborted` with `cancelled_by=<principal>`
