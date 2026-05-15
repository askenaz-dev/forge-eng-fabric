# mcp-gateway Specification

## Purpose
TBD - created by syncing change `active-registry-gateways`. Update Purpose after archive.

## Requirements

### Requirement: Bidirectional MCP traffic gateway

The platform SHALL provide an `mcp-gateway` service that fronts every approved MCP — internal (registry-owned) and external (third-party endpoint registered per Tenant) — and SHALL be the only path through which runners, Alfred and `services/skill-gateway` reach an MCP. Direct dial from any other component to an MCP endpoint SHALL be blocked by network policy once `gateway.enforced=true` is set for the Tenant.

#### Scenario: Internal caller invokes an internal MCP through the gateway

- **GIVEN** an approved internal MCP `github` registered in the Asset Registry with `active_surface.family=mcp`
- **WHEN** a runner invokes `github.create_pr` via `POST /v1/gw/mcp/github` with a valid workload-identity token
- **THEN** the gateway MUST evaluate OPA policy, propagate `X-Forge-Principal|Tenant|Workspace|Correlation-Id` to the MCP, return the MCP response unchanged, emit `com.forge.mcp.invocation.v1` with `source=internal` and write an audit record

#### Scenario: Internal caller invokes an external MCP through the gateway

- **GIVEN** an approved external MCP `vendor-x` with `provenance=external`, `transport.endpoint=https://vendor-x.example.com/mcp` and a Tenant-scoped credential ref
- **WHEN** a runner invokes a tool on `vendor-x`
- **THEN** the gateway MUST resolve the credential at execution time, attach it to the outbound request, propagate identity headers, evaluate OPA policy, and emit `com.forge.mcp.invocation.v1` with `source=external_proxy`
- **AND** the credential MUST NOT appear in logs, traces or audit payloads

#### Scenario: Direct dial blocked when enforcement is on

- **GIVEN** Tenant `t1` with `gateway.enforced=true`
- **WHEN** a runner inside `t1` attempts to call an MCP endpoint directly without going through `mcp-gateway`
- **THEN** the network policy MUST drop the connection and emit `guardrail.trip.v1` with reason `mcp_direct_dial_blocked`

### Requirement: Identity propagation and signed headers

The gateway SHALL inject identity context on every outbound MCP request, signed with a short-lived workload-identity-derived key that downstream MCPs verify. Headers SHALL include the calling principal, Tenant, Workspace, correlation id, and a signature over those fields.

#### Scenario: Downstream MCP verifies signed identity headers

- **WHEN** the gateway forwards a tool call to an internal MCP
- **THEN** the request MUST carry `X-Forge-Principal`, `X-Forge-Tenant`, `X-Forge-Workspace`, `X-Forge-Correlation-Id` and `X-Forge-Identity-Signature`
- **AND** the MCP SDK MUST reject the call when the signature does not verify

### Requirement: Policy, rate limits and budgets per call

Every MCP invocation through the gateway SHALL be subject to OPA policy evaluation, Tenant/Workspace-scoped rate limits, and Tenant-budget probes consistent with `model-gateway` enforcement. Budget exhaustion SHALL block the call and emit a budget event.

#### Scenario: Tool call blocked by OPA policy

- **GIVEN** a policy that denies `vendor-x.read_pii` for Workspace `ws-1`
- **WHEN** a runner in `ws-1` invokes `vendor-x.read_pii`
- **THEN** the gateway MUST return `403 policy_denied` with the policy reference
- **AND** emit `policy.decision.v1` with `outcome=deny`

#### Scenario: Tenant budget exhaustion blocks further calls

- **GIVEN** a Tenant whose monthly MCP-cost budget has been exhausted
- **WHEN** a new MCP invocation arrives
- **THEN** the gateway MUST return `429 budget_exhausted`
- **AND** emit `budget.exhausted.v1`

### Requirement: External MCP registration and drift detection

External MCPs SHALL be registered as Asset Registry assets with `provenance=external`, a `transport.endpoint` URL, a per-Tenant `credential_ref`, an optional tool `allowlist`, and the tool-manifest hash captured at registration. The gateway SHALL re-verify the manifest hash on each promotion to a higher lifecycle state and on a daily drift cron.

#### Scenario: Manifest drift detected on daily cron

- **GIVEN** external MCP `vendor-x` previously registered with manifest hash `H1`
- **WHEN** the daily drift cron fetches the live manifest and computes hash `H2 ≠ H1`
- **THEN** the gateway MUST emit `com.forge.mcp.external_drift.v1` with both hashes
- **AND** mark the asset `deprecated` automatically if the drift policy thresholds are exceeded

#### Scenario: Tool not on allowlist is rejected

- **GIVEN** external MCP `vendor-x` registered with `allowlist=[read_doc, list_docs]`
- **WHEN** a runner invokes `vendor-x.delete_doc`
- **THEN** the gateway MUST return `403 tool_not_allowlisted`
- **AND** emit `guardrail.trip.v1` with reason `mcp_tool_not_allowlisted`

### Requirement: Streaming (SSE) for long-running tool calls

The gateway SHALL support Server-Sent Events (SSE) for MCP tools that stream incremental output, preserving backpressure end-to-end from the downstream MCP to the calling client.

#### Scenario: Streaming tool relays incremental events

- **GIVEN** an MCP tool that emits progress events via SSE
- **WHEN** a caller subscribes through the gateway
- **THEN** the gateway MUST relay events with `<5 ms` of added per-event latency at p95
- **AND** terminate the upstream stream when the caller disconnects

### Requirement: Per-invocation telemetry

Every MCP call through the gateway SHALL emit OpenTelemetry traces and metrics with `tenant_id`, `workspace_id`, `asset_id`, `tool_name`, `source ∈ {internal, external_proxy}`, `latency`, `outcome`, `policy_decisions`, `cost_class`, and `correlation_id`; per-asset rollups SHALL be consumable by `per-asset-observability`.

#### Scenario: Metrics roll up per asset

- **WHEN** the gateway has served calls for asset `vendor-x` over a 5-minute window
- **THEN** `per-asset-observability` MUST report `invocations`, `p50/p95 latency`, `errors`, `cost` for `vendor-x` with `source=external_proxy`
