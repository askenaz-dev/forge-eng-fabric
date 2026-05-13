## ADDED Requirements

### Requirement: Public listing of installable assets

The gateway SHALL expose `GET /v1/gateway/assets` returning every Asset Registry item that is (a) `lifecycle_state=approved`, (b) `trust_level>=T1`, (c) has `distribution.gateway_published=true`, and (d) is visible to the authenticated developer's Tenant. Each entry SHALL include `id`, `version`, `type` (`skill|mcp|agent`), `name`, `description`, `trust_level`, `eval_scores`, `package_digest` (for skills), `remote_transport` (for MCP/agent), `required_scopes`, `homepage_url` and `install_hint` per supported client.

#### Scenario: Approved skill is listed

- **WHEN** an authenticated developer calls `GET /v1/gateway/assets?type=skill`
- **THEN** the response includes every approved, T1+, gateway-published skill in the developer's Tenant with non-null `package_digest`

#### Scenario: Unapproved asset is hidden

- **GIVEN** an asset with `lifecycle_state=in_review`
- **WHEN** the developer lists assets
- **THEN** the asset MUST NOT appear in the response

#### Scenario: Cross-tenant asset is hidden

- **GIVEN** an asset belonging to Tenant B
- **WHEN** a developer authenticated against Tenant A calls the endpoint
- **THEN** the asset MUST NOT appear regardless of trust level or publication state

### Requirement: Signed Agent Skills package download

The gateway SHALL serve `GET /v1/gateway/assets/{id}@{version}/package` returning a content-addressed `.tar.zst` bundle in the open Agent Skills format. The response SHALL include `X-Forge-Package-Digest` (sha256), `X-Forge-Signature` (cosign signature over the digest) and `X-Forge-Asset-Version` headers. Bundle contents SHALL match the digest recorded in the registry's `asset_package` row.

#### Scenario: Digest matches package

- **WHEN** a developer downloads the package
- **THEN** `sha256(body) == X-Forge-Package-Digest == asset_package.digest`

#### Scenario: Tampered package is rejected by CLI

- **GIVEN** the gateway response body has been altered in transit
- **WHEN** the CLI verifies `X-Forge-Signature` against the digest
- **THEN** verification MUST fail and the CLI MUST refuse the install

#### Scenario: Wrong version returns 404

- **WHEN** a developer requests a version not present in the registry
- **THEN** the gateway responds `404 package_not_found`

### Requirement: Authenticated MCP proxy with identity propagation

The gateway SHALL serve `/v1/gateway/mcp/{asset_id}` over both Streamable HTTP and SSE as a transparent proxy to the registered MCP server, requiring a developer PAT or OIDC bearer token. The gateway SHALL propagate the developer principal, Tenant and Workspace to the MCP runtime, broker any required secrets, evaluate policy before every tool call, and emit `com.forge.gateway.invocation.v1` per tool invocation.

#### Scenario: Tool call carries identity

- **WHEN** an external client invokes a tool through the MCP proxy
- **THEN** the MCP runtime receives the developer's `sub`, Tenant and Workspace claims
- **AND** an audit event records the tool name, outcome, latency and cost

#### Scenario: Policy denies cross-workspace mutation

- **GIVEN** a developer scoped to Workspace A invoking `github.create_repo` targeting an org bound to Workspace B
- **WHEN** the policy engine evaluates the request
- **THEN** the gateway responds `403 cross_workspace_denied` and emits `guardrail.trip.v1`

#### Scenario: Stdio-only MCP is not proxiable

- **GIVEN** an MCP whose `remote_transport` is `stdio` only
- **WHEN** a developer attempts to open the proxy
- **THEN** the gateway responds `409 remote_transport_unavailable`

### Requirement: Agent-to-Agent (A2A) invocation endpoint

The gateway SHALL expose `POST /v1/gateway/a2a/{asset_id}` accepting A2A-protocol task envelopes for assets of type `agent`. Responses SHALL stream task events (`status`, `artifact`, `final`) over SSE and carry the same identity, policy and audit guarantees as the MCP proxy.

#### Scenario: External agent delegates a task

- **WHEN** an external Claude Code session POSTs an A2A `tasks/send` payload to an approved Forge agent
- **THEN** the gateway opens an SSE stream emitting incremental `status` and `artifact` events until a `final` event
- **AND** each event is correlated by `correlation_id` to the developer's session

#### Scenario: Agent not approved

- **GIVEN** an agent in `proposed` state
- **WHEN** an A2A invocation is sent
- **THEN** the gateway responds `403 not_approved` without contacting the runtime

### Requirement: Developer personal access tokens

The gateway SHALL issue developer PATs via `POST /v1/gateway/tokens` and revoke via `DELETE /v1/gateway/tokens/{id}`. Each PAT SHALL be bound to a single `(developer_sub, tenant_id)`, carry an explicit list of scopes from `{gateway.read, gateway.install, gateway.invoke}`, optionally pin an `assume_workspace_id` and an asset allowlist, expire at most 90 days after creation and be storable only as a hashed secret server-side.

#### Scenario: Issued PAT is shown once

- **WHEN** a developer creates a PAT
- **THEN** the plaintext token is returned in the 201 body
- **AND** subsequent `GET` calls return only the hash, scopes, `created_at`, `expires_at`

#### Scenario: Expired token is refused

- **GIVEN** a PAT whose `expires_at` is in the past
- **WHEN** the developer presents it
- **THEN** the gateway responds `401 token_expired`

#### Scenario: Revoked token is refused immediately

- **WHEN** the owner revokes a PAT
- **THEN** the next request bearing the token is refused with `401 token_revoked` within 5 seconds across all gateway replicas

### Requirement: OIDC device-code login for the CLI

The gateway SHALL implement an OIDC device-authorization flow at `/v1/gateway/auth/device` and `/v1/gateway/auth/token` so the CLI can mint short-lived tokens without a browser callback on the developer's machine.

#### Scenario: Device-code login from CLI

- **WHEN** the CLI requests a device code
- **THEN** the gateway returns a `user_code`, a verification URL and a polling interval
- **AND** the developer completes login in any browser
- **AND** the CLI's polling request returns an access token + refresh token

### Requirement: Rate limits, budgets and quotas

The gateway SHALL enforce per-PAT and per-developer rate limits, a per-Tenant monthly cost budget for LLM-bearing invocations and a per-asset concurrency cap. Exceeding any limit SHALL respond `429` (rate) or `402` (budget) with `Retry-After` and emit an audit event.

#### Scenario: Developer hits per-minute rate limit

- **WHEN** a developer exceeds 60 requests/minute against `/v1/gateway/mcp`
- **THEN** further requests in that window receive `429 rate_limited` with `Retry-After` seconds

#### Scenario: Tenant exhausts monthly LLM budget

- **WHEN** an A2A or MCP call would push LiteLLM spend past the Tenant budget
- **THEN** the gateway refuses with `402 budget_exhausted` before the model call

### Requirement: Observability for gateway traffic

Every gateway request SHALL emit OpenTelemetry spans with `correlation_id`, `tenant_id`, `workspace_id`, `developer_sub`, `asset_id`, `version`, `route` and outcome. Invocation events SHALL be published to Kafka as `com.forge.gateway.invocation.v1` and to Langfuse for any AI payload. Install events SHALL be published as `com.forge.gateway.installed.v1`.

#### Scenario: Install emits an event

- **WHEN** the CLI downloads a package
- **THEN** the gateway emits `com.forge.gateway.installed.v1` with developer, asset, version and package digest

### Requirement: Public ingress hardening

The gateway SHALL terminate TLS, require a bearer token on every non-health endpoint, set `X-Forge-Correlation-Id` on each response, reject requests larger than 8 MB, deny requests whose `Origin` is not on a configured allowlist for browser clients, and expose only `/healthz` and `/readyz` without auth.

#### Scenario: Browser request from unknown origin

- **GIVEN** a JavaScript client whose `Origin` is not configured
- **WHEN** it issues a preflighted request
- **THEN** the gateway responds with no CORS-allow headers and a 403 to the actual request
