# Spec Delta: mcp-and-skills (MODIFIED)

## MODIFIED Requirements

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
