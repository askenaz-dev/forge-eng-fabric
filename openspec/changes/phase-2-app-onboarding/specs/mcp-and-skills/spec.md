# Spec Delta: mcp-and-skills (MODIFIED)

## MODIFIED Requirements

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
