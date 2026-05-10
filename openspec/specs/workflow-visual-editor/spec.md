# workflow-visual-editor Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Editor produces canonical AST

The visual editor MUST persist workflows as the canonical AST consumed by the runtime; the editor MUST NOT serialize a proprietary editor-only format.

#### Scenario: Save from editor matches DSL

- **GIVEN** a workflow built in the editor with 5 nodes and 1 branch
- **WHEN** saved
- **THEN** the persisted artifact MUST be the canonical AST
- **AND** exporting to DSL YAML MUST produce a valid file that re-imports identically

### Requirement: Live Registry catalog and validation

The editor MUST query the Registry for available skills/MCPs/prompts in real time and reject nodes referencing non-existent or non-approved assets.

#### Scenario: Reject node referencing in-review skill

- **GIVEN** a skill with `lifecycle_state=in_review`
- **WHEN** the user attempts to add it as a node
- **THEN** the editor MUST mark the node as invalid with reason `skill_not_approved`
- **AND** publish MUST be blocked

### Requirement: Debug dry-run

The editor MUST support a `dry_run` mode that executes the workflow with mocked I/O and surfaces the input/output of each step without invoking real tools.

#### Scenario: Dry-run does not call real MCPs

- **GIVEN** a workflow with `github.create_pr` step
- **WHEN** dry-run is executed
- **THEN** no actual GitHub call MUST occur
- **AND** the user MUST see mocked inputs/outputs based on declared schemas

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
