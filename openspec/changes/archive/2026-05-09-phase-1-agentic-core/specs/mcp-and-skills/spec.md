## ADDED Requirements

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
