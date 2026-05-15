## ADDED Requirements

### Requirement: Bidirectional A2A protocol gateway

The platform SHALL provide an `a2a-gateway` service that implements the [Agent-to-Agent (A2A) protocol](https://a2aproject.github.io/A2A/) and SHALL front every approved agent — internal (registry-owned) and external (third-party A2A endpoint registered per Tenant). The gateway SHALL support `tasks/send`, `tasks/get`, `tasks/cancel` and `tasks/sendSubscribe` over HTTP + SSE.

#### Scenario: Internal agent invokes an external A2A agent

- **GIVEN** an approved external A2A agent `partner-a` with `provenance=external`, `transport.endpoint=https://partner-a.example.com/a2a` and a Tenant-scoped credential ref
- **WHEN** an internal agent calls `tasks/send` via `POST /v1/gw/a2a/partner-a`
- **THEN** the gateway MUST evaluate OPA policy, attach the brokered credential, propagate identity headers and the agent card hash, return the task envelope unchanged, emit `com.forge.a2a.invocation.v1` with `source=external_proxy` and write an audit record

#### Scenario: External agent invokes a registered Forge agent

- **GIVEN** a registered internal Forge agent `forge-sdlc-architect` with `active_surface.family=a2a` and Tenant `t1` configured to accept inbound A2A from partner `partner-b`
- **WHEN** `partner-b` sends `tasks/send` to `/v1/gw/a2a/forge-sdlc-architect`
- **THEN** the gateway MUST authenticate the inbound caller against the partner credential, evaluate OPA policy, propagate the partner identity as `principal_kind=external_agent`, route to the internal agent runtime, return the task envelope and emit `com.forge.a2a.invocation.v1` with `source=inbound_external`

#### Scenario: Streaming task via `tasks/sendSubscribe`

- **GIVEN** an internal agent that streams partial outputs through `tasks/sendSubscribe`
- **WHEN** the caller subscribes via the gateway
- **THEN** the gateway MUST relay events over SSE, preserve backpressure, terminate the upstream on caller disconnect and emit a final `task.completed` or `task.failed` event with the same correlation id

### Requirement: Identity propagation and policy on every task

Every A2A task through the gateway SHALL carry signed identity headers (`X-Forge-Principal`, `X-Forge-Tenant`, `X-Forge-Workspace`, `X-Forge-Correlation-Id`, `X-Forge-Identity-Signature`) and SHALL be subject to OPA policy evaluation before dispatch. Policy denial SHALL block the task and emit an audit event.

#### Scenario: Task denied by policy

- **GIVEN** a policy denying `partner-a` for Workspace `ws-2`
- **WHEN** an agent in `ws-2` invokes `partner-a` via `tasks/send`
- **THEN** the gateway MUST return `403 policy_denied`
- **AND** emit `policy.decision.v1` with `outcome=deny`

### Requirement: External A2A registration with agent-card verification

External A2A agents SHALL be registered as Asset Registry assets with `provenance=external`, a `transport.endpoint` URL, a per-Tenant `credential_ref`, an optional `skills/tasks allowlist` and the agent-card hash captured at registration. The gateway SHALL re-verify the agent-card hash on each promotion and on a daily drift cron.

#### Scenario: Agent-card drift triggers deprecation

- **GIVEN** external A2A agent `partner-a` with agent-card hash `H1`
- **WHEN** the daily cron fetches the live agent card and computes hash `H2 ≠ H1`
- **THEN** the gateway MUST emit `com.forge.a2a.external_drift.v1`
- **AND** mark the asset `deprecated` automatically if the drift policy thresholds are exceeded

### Requirement: Inbound trust boundary for external agents

The gateway SHALL accept inbound A2A calls only from external agents that the Tenant has explicitly enrolled, with mTLS or signed-JWT authentication, and SHALL reject anonymous inbound traffic.

#### Scenario: Unenrolled partner is rejected

- **WHEN** an A2A request arrives from a partner not enrolled for the target Tenant
- **THEN** the gateway MUST return `401 unknown_partner`
- **AND** emit `guardrail.trip.v1` with reason `a2a_unknown_partner`

### Requirement: Per-task telemetry

Every A2A task through the gateway SHALL emit OpenTelemetry traces and metrics with `tenant_id`, `workspace_id`, `asset_id`, `task_id`, `source ∈ {internal, external_proxy, inbound_external}`, `latency`, `outcome`, `policy_decisions` and `correlation_id`; per-asset rollups SHALL feed `per-asset-observability`.

#### Scenario: Metrics roll up per agent

- **WHEN** the gateway has served tasks for `partner-a` over a 5-minute window
- **THEN** `per-asset-observability` MUST report `tasks_started`, `tasks_completed`, `tasks_failed`, `p50/p95 latency` and `cost` for `partner-a` with `source=external_proxy`
