## ADDED Requirements

### Requirement: Agent-mode session lives alongside the single-intent loop

Alfred SHALL expose an **agent-mode session** surface (long-running, plan-driven, resumable) in addition to the existing single-intent loop and wizard dialogue. The single-intent loop (`POST /v1/intents`) and wizard dialogue API (`/v1/intent/start|answer|commit`) SHALL remain unchanged and SHALL continue to be the entry points for ad-hoc requests and OpenSpec authoring.

#### Scenario: Single-intent loop is unaffected by agent-mode

- **WHEN** a client posts to `POST /v1/intents` after agent-mode is deployed
- **THEN** Alfred SHALL execute exactly one loop iteration set (RAG → LLM → policy → tool/approval/final) and return an `IntentResponse` as before — no agent-mode session is created and no agent-mode events are emitted

#### Scenario: Wizard dialogue remains the OpenSpec authoring path

- **WHEN** a client drives the wizard via `/v1/intent/start|answer|commit`
- **THEN** the dialogue completes by producing a committed `openspec_id` exactly as today
- **AND** the committed OpenSpec is what an agent-mode session subsequently consumes when the user chooses to ship it

### Requirement: Per-step reuse of the existing policy and permission stack

Each agent-mode step that calls a tool, MCP or sub-agent SHALL reuse the existing per-iteration stack: delegated-permissions check via `PermissionsClient`, OPA evaluation via `PolicyClient`, redacted decision record via `Store.append_decision`, and observability capture via `AIObserver`. Agent-mode SHALL NOT introduce a parallel policy path.

#### Scenario: Step inherits the same permission gate as a single-intent call

- **WHEN** an agent-mode step invokes `mcp:github.open_pr`
- **THEN** the permission check SHALL use `subject="alfred"`, `action_class="github:write"`, `scope_kind="workspace"`, `scope_id=<workspace_id>` and the resolved criticality from the plan step — identical to a single-intent invocation of the same tool

#### Scenario: Policy denial halts the session, not just the step

- **WHEN** OPA returns `deny` for an agent-mode step
- **THEN** the session SHALL transition to `failed` with the policy rationale persisted on the session row and a final `alfred.agent_mode.failed.v1` event emitted
- **AND** subsequent steps SHALL NOT execute even if their isolated policy decision would have been `allow`

### Requirement: Resolved autonomy policy is frozen at session start

A session SHALL freeze a copy of the workspace's active `autonomy_policy` (and its per-action ceilings) on the session row at creation time. Subsequent admin edits to the workspace's presets SHALL NOT alter the running session's ceilings.

#### Scenario: Admin tightens preset after session start

- **WHEN** a workspace admin replaces `full-autonomy` with `manual-prod` while a session is running
- **THEN** the in-flight session SHALL continue under the originally frozen `full-autonomy` ceilings
- **AND** the next agent-mode session started afterward SHALL pick up the new `manual-prod` ceilings

### Requirement: Session storage and durable event log

The Alfred store SHALL persist `alfred_agent_session` rows and `alfred_agent_step` rows, both keyed by the session's `correlation_id`. Every state transition and step outcome SHALL also write a `DecisionRecord` so the audit trail and the agent-mode event log share one source of truth.

#### Scenario: Session and decisions are joined by correlation

- **WHEN** an auditor queries decisions for an agent-mode session
- **THEN** all `DecisionRecord` rows for the session SHALL share the same `correlation_id` and SHALL be retrievable via `GET /v1/agent-mode/sessions/{id}/decisions`

#### Scenario: Step row preserves dispatched workflow / tool identity

- **WHEN** a `workflow` step triggers `forge.reference.intent-to-deploy@1`
- **THEN** the `alfred_agent_step` row SHALL record the workflow id and run id, and joining back via the run id SHALL return the workflow runtime's per-step events without re-deriving them
