## ADDED Requirements

### Requirement: Implementation choice recorded as ADR

The platform SHALL record the visual editor implementation choice (embed Flowise vs fork n8n vs build custom) as an Architecture Decision Record at `docs/governance/adrs/0001-workflow-visual-editor.md`. The ADR SHALL be referenced from this capability and from `docs/platform-enablement.md`.

#### Scenario: ADR exists and is referenced

- **WHEN** a contributor reads this capability or `docs/platform-enablement.md`
- **THEN** a link SHALL navigate to `docs/governance/adrs/0001-workflow-visual-editor.md`, and the ADR SHALL be in `accepted` status with the chosen approach, alternatives, consequences, and review date

### Requirement: Editor integrated into the Portal

The visual editor SHALL be embedded in the Portal at `/workflows/editor` and SHALL operate within the Portal's authentication, authorization, and Workspace context.

#### Scenario: Editor opens within a Workspace

- **WHEN** a user with `workflow.author` permission navigates to `/workflows/editor` in their Workspace
- **THEN** the editor SHALL load with the Workspace context applied and SHALL only display nodes and assets visible to the Workspace per OpenFGA

#### Scenario: Editor honors approval policies

- **WHEN** a user without `workflow.author` permission attempts to open the editor
- **THEN** the Portal SHALL deny access and surface a clear permission-error message

### Requirement: Canonical node catalog rendered

The editor SHALL render exactly the canonical node catalog: LLM, MCP, Skill, Agent, Prompt Template, HITL Gate, Branch, Loop, Retry, Eval, Webhook, GitHub Action, Deploy Action, Approval Action, Notification Action. Additional node types SHALL be added only via OpenSpec change with capability impact.

#### Scenario: Catalog matches the canonical list

- **WHEN** a user opens the node palette
- **THEN** every canonical node type SHALL be present and SHALL be queryable from the Registry; nodes referencing assets in non-`approved` state SHALL be visually marked and disallowed in saved workflows

### Requirement: Editor persists canonical AST in `workflow-registry`

The editor SHALL persist workflows by writing the canonical AST to `workflow-registry`. Workflows SHALL be versioned: each save SHALL produce a new immutable version, and the editor SHALL allow opening prior versions read-only.

#### Scenario: Save creates a new version

- **WHEN** a user saves changes to a workflow
- **THEN** `workflow-registry` SHALL persist a new version with monotonically increasing `version` and SHALL retain the prior version

#### Scenario: Open historical version read-only

- **WHEN** a user opens a non-latest version
- **THEN** the editor SHALL render the workflow read-only and SHALL offer "fork as new latest" as the only mutating action
