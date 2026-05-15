## MODIFIED Requirements

### Requirement: Policy checks before tool execution
Before executing any tool, MCP, Skill or Workflow node, the platform SHALL evaluate the applicable policies (Workspace, OpenSpec, asset trust level, environment, data classification) at the corresponding active gateway (`mcp-gateway` for MCPs, `a2a-gateway` for agent-to-agent calls, `model-gateway` for LLMs). Failure SHALL block the action and emit an audit event. Internal callers MUST reach MCPs through `mcp-gateway` and remote agents through `a2a-gateway`; direct dial SHALL be blocked by network policy when `gateway.enforced=true` is set for the Tenant.

#### Scenario: Policy denies action and audits attempt
- **WHEN** Alfred attempts to invoke a T4 deploy asset on prod without the required approval
- **THEN** the relevant gateway MUST block execution, emit an audit event, and return a structured policy-denial error

#### Scenario: Direct dial blocked under enforcement
- **GIVEN** Tenant `t1` with `gateway.enforced=true`
- **WHEN** a runner in `t1` attempts to invoke an MCP or remote agent without traversing the corresponding gateway
- **THEN** the network policy MUST drop the connection and emit `guardrail.trip.v1` with reason `gateway_bypass_blocked`

## ADDED Requirements

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
