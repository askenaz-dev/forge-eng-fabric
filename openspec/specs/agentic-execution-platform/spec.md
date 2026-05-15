# agentic-execution-platform Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Isolated runners
The platform SHALL execute agents, tools and workflows in isolated runners (containers/sandboxes) with per-execution identity, network policies and resource quotas. Runners SHALL NOT share secrets or memory across executions or tenants.

#### Scenario: Two concurrent executions cannot read each other's secrets
- **WHEN** two runner executions for different Workspaces run concurrently
- **THEN** neither execution can read or enumerate secrets, env vars or filesystem of the other

### Requirement: Secret brokering and identity propagation
The platform SHALL broker secrets at execution time from a Secret Manager / Vault and propagate the calling principal's identity (user/app) to integrated systems. Secrets SHALL never be embedded in prompts, logs or persisted alongside execution payloads.

#### Scenario: Secret never appears in logs or prompts
- **WHEN** a runner uses a brokered secret to call an external API
- **THEN** the secret is redacted from logs, traces, prompts, and tool input/output records

### Requirement: Policy checks before tool execution
Before executing any tool, MCP, Skill or Workflow node, the platform SHALL evaluate the applicable policies (Workspace, OpenSpec, asset trust level, environment, data classification) at the corresponding active gateway (`mcp-gateway` for MCPs, `a2a-gateway` for agent-to-agent calls, `model-gateway` for LLMs). Failure SHALL block the action and emit an audit event. Internal callers MUST reach MCPs through `mcp-gateway` and remote agents through `a2a-gateway`; direct dial SHALL be blocked by network policy when `gateway.enforced=true` is set for the Tenant.

#### Scenario: Policy denies action and audits attempt
- **WHEN** Alfred attempts to invoke a T4 deploy asset on prod without the required approval
- **THEN** the relevant gateway MUST block execution, emit an audit event, and return a structured policy-denial error

#### Scenario: Direct dial blocked under enforcement
- **GIVEN** Tenant `t1` with `gateway.enforced=true`
- **WHEN** a runner in `t1` attempts to invoke an MCP or remote agent without traversing the corresponding gateway
- **THEN** the network policy MUST drop the connection and emit `guardrail.trip.v1` with reason `gateway_bypass_blocked`

### Requirement: Rate limits, cost limits and budgets
The platform SHALL enforce rate limits and cost limits per Tenant, Workspace, asset and environment. Exceeding budgets SHALL block further executions and notify owners.

#### Scenario: Workspace exceeds LLM budget
- **WHEN** a Workspace's monthly LiteLLM budget is exhausted
- **THEN** further model calls from that Workspace are blocked, owners are notified, and audit records the event

### Requirement: Retry, checkpointing and durability
The platform SHALL support automatic retries with backoff for transient failures and SHALL provide checkpointing/durability for long-running executions. Long-running workflows SHALL be able to use **Temporal** as the execution engine.

#### Scenario: Long-running workflow resumes after runner crash
- **WHEN** a Temporal-backed workflow is interrupted by a runner crash
- **THEN** the workflow resumes from the last checkpoint without duplicating completed side-effects

### Requirement: Eval harness and guardrails
The platform SHALL provide an eval harness producing scores for quality, safety, cost and latency, and SHALL apply guardrails (input/output filtering, prompt-injection detection, allowlists, schema validation) on agent and LLM interactions.

#### Scenario: Guardrail blocks suspicious tool call from injected content
- **WHEN** retrieved RAG content contains instructions attempting to invoke a sensitive tool not on the allowlist
- **THEN** the guardrail blocks the tool call and audits the prompt-injection attempt

### Requirement: Telemetry for executions
Every runner execution SHALL emit OpenTelemetry traces and metrics including `correlation_id`, `tenant_id`, `workspace_id`, `asset_id`, `version`, `policy_decisions`, `latency`, `tokens`, `cost`, `eval_scores` and `outcome`. AI-specific telemetry SHALL be sent to **Langfuse** (or equivalent).

#### Scenario: Execution emits traces and AI telemetry
- **WHEN** an asset is invoked from a workflow
- **THEN** OpenTelemetry traces are emitted with required attributes and Langfuse receives prompt/response/eval telemetry

### Requirement: Per-Tenant gateway enforcement flag

The platform SHALL expose a per-Tenant `gateway.enforced` configuration flag. When the flag is `true`, network policy SHALL block direct dial from runners and Alfred to MCP and agent endpoints, allowing traffic only through `mcp-gateway` and `a2a-gateway`. When the flag is `false`, a compatibility shim SHALL forward direct-dial attempts through the gateways while emitting a deprecation telemetry event.

#### Scenario: Enforcement off — shim forwards with deprecation event

- **GIVEN** Tenant `t2` with `gateway.enforced=false`
- **WHEN** a runner uses the legacy client to call an MCP directly
- **THEN** the compatibility shim MUST forward the call through `mcp-gateway`
- **AND** emit `com.forge.runtime.gateway_bypass_deprecated.v1` with the caller identity

#### Scenario: Enforcement on — bypass blocked at network policy

- **GIVEN** Tenant `t1` with `gateway.enforced=true`
- **WHEN** a runner attempts a direct dial outside the gateway
- **THEN** the network policy MUST drop the connection
- **AND** emit `guardrail.trip.v1` with reason `gateway_bypass_blocked`

### Requirement: Pinned-asset enforcement during orchestration

When a workflow or OpenSpec carries a non-empty `selected_assets` block (from the intent capture wizard or the visual editor), the runtime SHALL enforce that every asset invocation during execution matches an entry in the pinned set; invocations outside the pinned set SHALL be rejected with a structured error.

#### Scenario: Invocation outside pinned set rejected

- **GIVEN** a workflow with `selected_assets.skills=[skill-a@1.0.0, skill-b@2.1.0]`
- **WHEN** during execution the LLM attempts to invoke `skill-c@1.0.0`
- **THEN** the runtime MUST refuse with `403 asset_not_pinned`
- **AND** emit `guardrail.trip.v1` with reason `asset_not_in_pinned_set`

#### Scenario: Empty pinned set preserves current behavior

- **GIVEN** a workflow with `selected_assets` empty or absent
- **WHEN** the runtime executes
- **THEN** asset selection MUST behave exactly as it did before this change (no pinning enforcement)

