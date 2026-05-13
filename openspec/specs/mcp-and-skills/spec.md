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
Every MCP/Skill invocation SHALL propagate the calling principal's identity, evaluate policy before execution, and emit audit and telemetry on success/failure.

#### Scenario: Tool call denied by policy
- **WHEN** Alfred attempts to invoke a tool whose policy evaluates to `deny`
- **THEN** the call is blocked, the user-facing error explains the policy decision, and an audit event is emitted

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
