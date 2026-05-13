## ADDED Requirements

### Requirement: Remote-transport contract for gateway-eligible MCPs

Every MCP server intended to be installable through the developer skill gateway SHALL declare a `remote_transport` block in its registry metadata with at least one of `http` (Streamable HTTP, MCP spec compliant) or `sse` (Server-Sent Events fallback). The block SHALL include `path_template` (relative URL the gateway will mount), `auth_modes` (which subset of `pat`, `oidc_bearer`, `none` it accepts behind the gateway), and a `health_path`. MCPs that only support `stdio` SHALL remain valid for in-platform use but SHALL be rejected by the gateway-publication hook with `409 remote_transport_required`.

#### Scenario: Approved MCP declares HTTP transport

- **GIVEN** an MCP with `remote_transport.http.path_template="/mcp"` and a passing `/healthz`
- **WHEN** the gateway-publication hook is called
- **THEN** the hook succeeds and the MCP becomes installable via `forge skills install`

#### Scenario: Stdio-only MCP cannot be gateway-published

- **GIVEN** an MCP whose registry record has no `remote_transport`
- **WHEN** a publisher attempts the gateway-publication hook
- **THEN** the hook responds `409 remote_transport_required`
- **AND** the MCP remains usable inside the platform runtime

### Requirement: Identity propagation through the gateway

When a tool call reaches an MCP through `/v1/gateway/mcp/{id}`, the MCP SDK SHALL accept the gateway-forwarded headers `X-Forge-Principal`, `X-Forge-Tenant`, `X-Forge-Workspace`, `X-Forge-Correlation-Id` as the canonical identity for that invocation. The MCP SHALL ignore any conflicting identity claims in the inbound MCP-protocol payload.

#### Scenario: Conflicting client claim is ignored

- **GIVEN** an inbound MCP payload that claims `user: attacker@evil.com`
- **WHEN** the gateway-forwarded `X-Forge-Principal` is `ana@acme.io`
- **THEN** the MCP records `ana@acme.io` as the actor in audit
- **AND** the gateway logs a `header_override` event for forensics

### Requirement: Reference skills are gateway-publishable

The three reference skills mandated by this capability — `create-user-stories`, `scaffold-service`, `generate-test-cases` — SHALL each be packaged in the open Agent Skills format and gateway-published to channel `stable` at trust level T2 minimum, alongside their existing in-platform availability.

#### Scenario: Reference skill installs from a clean machine

- **GIVEN** a developer with a valid PAT
- **WHEN** they run `forge skills install generate-test-cases`
- **THEN** the bundle resolves, verifies, and lands in the active client's skills directory
