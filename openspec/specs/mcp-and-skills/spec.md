# mcp-and-skills Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: MCP base SDK
The platform SHALL provide a Python **MCP base SDK** that standardizes server scaffolding, identity propagation, secret brokering, telemetry, audit and policy hooks for any MCP server in Forge.

#### Scenario: New MCP server uses the SDK
- **WHEN** a developer scaffolds a new MCP server with the SDK
- **THEN** the server inherits identity propagation, telemetry, audit and policy hooks without custom wiring

### Requirement: Initial MCP servers
The platform SHALL ship initial MCP servers for: **GitHub**, **Jira**, **Confluence** and **OpenSpec**, registered in the Asset Registry with metadata, eval scores and a trust level.

#### Scenario: Alfred invokes the GitHub MCP to read repo metadata
- **WHEN** Alfred invokes the GitHub MCP for a Workspace with the corresponding delegated permission
- **THEN** the call propagates identity, returns the requested data, and produces audit and telemetry records

### Requirement: Initial reference Skills
The platform SHALL ship at least three reference Skills: `create-user-stories`, `scaffold-service`, `generate-test-cases`, registered in the Registry with `inputs_schema`, `outputs_schema`, evals and `approved` lifecycle for at least T1 use.

#### Scenario: Alfred invokes a reference Skill
- **WHEN** Alfred invokes `generate-test-cases` for an OpenSpec
- **THEN** the Skill executes through the runner with policy checks, returns structured outputs validated against `outputs_schema`, and produces audit and telemetry records

### Requirement: Identity propagation and policy hooks for tools
Every MCP/Skill invocation SHALL be routed through `mcp-gateway`. The gateway SHALL propagate the calling principal's identity via signed headers, evaluate policy before execution, and emit audit and telemetry on success/failure. Direct dial to an MCP endpoint by any internal caller SHALL be blocked by network policy once `gateway.enforced=true` is set for the Tenant.

#### Scenario: Tool call denied by policy
- **WHEN** Alfred attempts to invoke a tool whose policy evaluates to `deny`
- **THEN** the gateway MUST block the call, return a structured policy-denial error and emit `policy.decision.v1` with `outcome=deny`

#### Scenario: Direct dial bypass blocked
- **GIVEN** a runner attempting to call an MCP endpoint directly without traversing `mcp-gateway` in a Tenant with `gateway.enforced=true`
- **WHEN** the request is sent
- **THEN** the network policy MUST drop the connection and emit `guardrail.trip.v1` with reason `mcp_direct_dial_blocked`

### Requirement: Allowlists per trust level and data classification
Sensitive tools SHALL be invocable only when the caller's context (trust level, data classification, environment, criticality) is on the allowlist defined by Security policy.

#### Scenario: T4 deploy tool blocked from T1 caller
- **WHEN** a caller without sufficient trust level attempts to invoke a T4 deploy tool
- **THEN** the platform blocks the call and audits the attempt

### Requirement: GitHub MCP write-mode

The GitHub MCP SHALL be extended from read-only to **read/write** with a tool catalog covering repo creation, branch management, PRs, branch protections, CODEOWNERS, PR/issue templates, and required checks. All write tools MUST be gated by policy and audited.

#### Scenario: Create repo via MCP with policy approval

- **GIVEN** Alfred holds a `delegated_permission` with action_class `repo:write` and an approved policy
- **WHEN** Alfred invokes `github.create_repo` with valid parameters
- **THEN** the MCP MUST issue a short-lived installation token (≤10 min) scoped to the org
- **AND** create the repo
- **AND** emit `mcp.tool.invoked.v1` with tool=`github.create_repo`, outcome=`success`, scope details
- **AND** record an audit entry

#### Scenario: Reject MCP write outside Workspace scope

- **GIVEN** Alfred is scoped to Workspace `ws-1`
- **WHEN** Alfred attempts `github.create_repo` in an org bound to `ws-2`
- **THEN** the MCP MUST refuse with `403 cross_workspace_denied`
- **AND** emit `guardrail.trip.v1` with reason `cross_workspace_mutation`

#### Scenario: Reject mutation without approval where required

- **GIVEN** a Workspace policy requiring approval for `github.set_branch_protection`
- **WHEN** Alfred invokes the tool without an approved request
- **THEN** the MCP MUST refuse with `403 approval_required`
- **AND** create an entry in the Approvals Inbox with the proposed mutation

### Requirement: Mutation guardrails

Write tools MUST validate inputs against schema, enforce allowlists for org/repo names, deny destructive operations (`delete_repo`, `force_push`) without explicit override, and log a full diff of mutations applied.

#### Scenario: Deny force-push without override

- **GIVEN** Alfred attempts `github.force_push` to `main`
- **WHEN** no override `allow-force-push` is approved
- **THEN** the MCP MUST refuse
- **AND** emit `guardrail.trip.v1` with reason `destructive_op_denied`

### Requirement: Jira MCP read/write

The MCP catalog SHALL include a Jira MCP supporting `create_issue`, `update_issue`, `transition_issue`, `add_comment`, `link_issue`, `create_epic`, `list_sprints`, `search`. Auth MUST support OAuth 2.0 (Cloud) and API token; credentials MUST be stored encrypted.

#### Scenario: Workspace mapping enforces project boundary

- **GIVEN** Workspace `ws-1` mapped to Jira projects `[ENG, PLAT]`
- **WHEN** Alfred invokes `jira.create_issue` against project `OPS`
- **THEN** the MCP MUST refuse with `403 project_not_mapped`
- **AND** emit `guardrail.trip.v1{reason=jira_project_unmapped}`

#### Scenario: Webhook ingestion produces events

- **GIVEN** Jira webhook configured for project `ENG`
- **WHEN** an issue transitions
- **THEN** the MCP MUST emit `jira.issue.updated.v1` to the bus
- **AND** the traceability service MUST update the relevant nodes/links

### Requirement: Confluence MCP read/write

The MCP catalog SHALL include a Confluence MCP supporting `create_page`, `update_page`, `attach_file`, `add_label`, `search`. Pages created MUST carry label `forge-managed` and a header line referencing the OpenSpec.

#### Scenario: Confluence page reflects OpenSpec link

- **GIVEN** Alfred creates a design page for `spec-7`
- **WHEN** the page is rendered
- **THEN** the page header MUST include `OpenSpec: spec-7`
- **AND** label `forge-managed` MUST be applied

### Requirement: SDLC skills registered

Each `sdlc-*` capability (product, architecture, design, development, qa, security, devops, sre, finops) MUST register at least 3 skills as Registry assets in `lifecycle_state=approved` and `trust_level≥T2`.

#### Scenario: Skills are listable and invokable

- **GIVEN** all SDLC capabilities registered
- **WHEN** querying `GET /v1/skills?capability=sdlc-design`
- **THEN** at least 3 skills MUST be returned with eval scores
- **AND** Alfred MUST be able to invoke each given proper delegated permissions

### Requirement: Editor consumes Registry catalog in real time

The visual editor and DSL parser MUST resolve node references against the Registry; references to non-existent or non-approved assets MUST be rejected at validation time.

#### Scenario: Reject reference to unknown skill

- **GIVEN** a DSL referencing `registry:skill/non-existent@1.0.0`
- **WHEN** validation runs
- **THEN** the parser MUST refuse with `400 unknown_asset`

### Requirement: Pinned references

Workflow steps MUST reference assets by exact id+version; floating tags (e.g., `latest`) MUST be rejected.

#### Scenario: Reject floating reference

- **GIVEN** a DSL referencing `registry:skill/refine-user-story@latest`
- **WHEN** validation runs
- **THEN** the parser MUST refuse with `400 floating_reference_not_allowed`

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

### Requirement: Hybrid sync model for registry assets

Asset registry entries (skills, MCPs, agents) SHALL be kept in sync with their origin via a hybrid push+pull model.

**Push (preferred):** origin repositories notify Forge on new releases via a webhook. For internal repos this is the existing CI `gateway-publish` hook. For public-origin assets (npm, GitHub Packages public), publishers MAY opt in to a Forge-provided webhook receiver endpoint; Forge SHALL expose `POST /v1/registry/webhooks/npm` and `POST /v1/registry/webhooks/github` to accept standard release event payloads from those registries.

**Pull (backstop):** a Sync Worker SHALL poll each registered asset's origin periodically (configurable per-Tenant, default: weekly). On detecting a version newer than the latest registry entry, the Sync Worker SHALL trigger the mirror flow (for public-origin skills) or a version-update notification (for private-origin assets). The Sync Worker MUST batch origin queries to respect source API rate limits and MUST emit `registry.sync.completed.v1` with counts of checked, drifted and mirrored assets after each run.

#### Scenario: npm webhook triggers mirror on new release

- **GIVEN** asset `cool-skill` registered with `origin_ref=npm:cool-skill` and npm hook configured to `POST /v1/registry/webhooks/npm`
- **WHEN** publisher releases `cool-skill@2.3.0` on npm
- **THEN** Forge MUST receive the webhook, fetch and mirror `2.3.0`, set `lifecycle_state=mirrored`, notify the asset owner

#### Scenario: Sync Worker detects drift and auto-mirrors

- **GIVEN** weekly Sync Worker run; registry has `cool-skill@2.1.0` approved; npm has `2.2.0`
- **WHEN** the worker queries the npm registry API
- **THEN** it MUST detect the version gap, mirror `2.2.0`, emit `asset.version.mirrored.v1`, and notify the asset owner (or auto-promote if `auto_promote_policy` permits)

#### Scenario: Sync Worker respects API rate limits

- **GIVEN** a Tenant with 500 public-origin assets
- **WHEN** the weekly sync run starts
- **THEN** the worker MUST batch npm registry API queries (≤100 req/min) and MUST NOT exhaust the origin's rate limit allowance

### Requirement: MCP tool-list drift detection

The `mcp-gateway` SHALL compare the live `tools/list` response from an MCP server against the cached tool list in the registry on every new client session. When a difference is detected (tools added, removed, or renamed), the gateway SHALL:

1. Emit `mcp.tool_list.drifted.v1` with the full before/after diff.
2. Update the registry cache with the new tool list immediately (so subsequent callers see current tools).
3. Notify the asset owner that the registered tool list has changed and invite confirmation of the updated metadata.

This replaces the need to poll MCP endpoints separately — the gateway's existing connection is the pull mechanism for MCPs.

#### Scenario: Gateway detects new tool on connection

- **GIVEN** MCP `vendor-x` has registry cache `[read_doc, list_docs]`
- **WHEN** a new session connects and `tools/list` returns `[read_doc, list_docs, exec_command]`
- **THEN** the gateway MUST emit `mcp.tool_list.drifted.v1{added=[exec_command]}`
- **AND** update the registry cache to `[read_doc, list_docs, exec_command]`
- **AND** notify the asset owner

#### Scenario: Removed tool triggers drift notification

- **GIVEN** MCP `vendor-x` has registry cache `[read_doc, list_docs, exec_command]`
- **WHEN** `tools/list` returns `[read_doc, list_docs]` (exec_command gone)
- **THEN** the gateway MUST emit `mcp.tool_list.drifted.v1{removed=[exec_command]}`
- **AND** the asset owner MUST be notified so they can update any workflow steps referencing `exec_command`

### Requirement: Public-origin badge in catalog and asset UI

Assets with `is_public_origin=true` SHALL be visually distinguished in the Portal asset catalog and asset detail page with a **PUBLIC ORIGIN** badge. The badge tooltip SHALL display `origin_ref` and `last_synced_at`. The asset's `auto_promote_policy` setting SHALL be editable by the asset owner from the detail page.
