## MODIFIED Requirements

### Requirement: Identity propagation and policy hooks for tools
Every MCP/Skill invocation SHALL be routed through `mcp-gateway`. The gateway SHALL propagate the calling principal's identity via signed headers, evaluate policy before execution, and emit audit and telemetry on success/failure. Direct dial to an MCP endpoint by any internal caller SHALL be blocked by network policy once `gateway.enforced=true` is set for the Tenant.

#### Scenario: Tool call denied by policy
- **WHEN** Alfred attempts to invoke a tool whose policy evaluates to `deny`
- **THEN** the gateway MUST block the call, return a structured policy-denial error and emit `policy.decision.v1` with `outcome=deny`

#### Scenario: Direct dial bypass blocked
- **GIVEN** a runner attempting to call an MCP endpoint directly without traversing `mcp-gateway` in a Tenant with `gateway.enforced=true`
- **WHEN** the request is sent
- **THEN** the network policy MUST drop the connection and emit `guardrail.trip.v1` with reason `mcp_direct_dial_blocked`

## ADDED Requirements

### Requirement: External MCP onboarding

The platform SHALL support registering an external (third-party) MCP server as an Asset Registry asset with `provenance=external`. Onboarding SHALL require: endpoint URL, per-Tenant `credential_ref` (vault path), optional tool `allowlist`, and a one-time manifest fetch + hash. Approval and trust-level promotion SHALL follow the standard lifecycle.

#### Scenario: Operator onboards vendor MCP

- **GIVEN** Tenant `t1` with a Nexus-stored credential at `vault://t1/vendor-x/api-key`
- **WHEN** an operator submits an external MCP registration for `vendor-x` with endpoint `https://vendor-x.example.com/mcp` and `allowlist=[read_doc, list_docs]`
- **THEN** the registry MUST fetch the live manifest, persist its hash, create the asset with `provenance=external, lifecycle_state=proposed` and emit `com.forge.asset.external_registered.v1`

#### Scenario: External MCP credential never exposed

- **GIVEN** external MCP `vendor-x` with `credential_ref=vault://t1/vendor-x/api-key`
- **WHEN** the gateway forwards a tool call to `vendor-x`
- **THEN** the credential MUST be resolved at execution time, attached to the outbound request only, and MUST NOT appear in logs, traces, audit payloads or telemetry

### Requirement: MCP discovery flows through the gateway catalog

Consumers SHALL discover available MCPs (internal and external) through the gateway catalog endpoint, which surfaces every approved MCP regardless of provenance with consistent metadata, `how_to` and `active_surface` blocks.

#### Scenario: Catalog lists internal and external MCPs uniformly

- **GIVEN** Tenant `t1` with internal MCPs `github`, `jira` and external MCP `vendor-x` all in `lifecycle_state=approved`
- **WHEN** a caller queries `GET /v1/gw/mcp/catalog`
- **THEN** all three MUST be returned with `provenance`, `active_surface.endpoint`, `how_to.install` and `how_to.usage`
