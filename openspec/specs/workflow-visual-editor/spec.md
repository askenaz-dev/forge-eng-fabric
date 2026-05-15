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

The editor MUST resolve available skills/MCPs/prompts/agents through the gateway catalog endpoints (`/v1/gw/mcp/catalog`, `/v1/gw/a2a/catalog`, and the registry skill catalog filtered by `active_surface ≠ null`), in real time, and MUST reject nodes referencing non-existent or non-approved assets, or assets without a populated `active_surface`.

#### Scenario: Reject node referencing in-review skill

- **GIVEN** a skill with `lifecycle_state=in_review`
- **WHEN** the user attempts to add it as a node
- **THEN** the editor MUST mark the node as invalid with reason `skill_not_approved`
- **AND** publish MUST be blocked

#### Scenario: Reject node referencing asset without active surface

- **GIVEN** an approved asset whose `active_surface` is null
- **WHEN** the user attempts to add it as a node
- **THEN** the editor MUST mark the node as invalid with reason `missing_active_surface`
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

### Requirement: Saved nodes carry the gateway endpoint

When a workflow is saved, each skill / MCP / agent node SHALL persist both the asset reference (`id@version`) and the `active_surface.endpoint` resolved at save time, so the runtime invokes through the same gateway endpoint that the editor surfaced.

#### Scenario: Saved AST contains gateway endpoint

- **GIVEN** a workflow with one `mcp` node referencing `github@2.0.0`
- **WHEN** the user saves the workflow
- **THEN** the persisted AST MUST include `node.asset_ref="github@2.0.0"` and `node.active_surface.endpoint="/v1/gw/mcp/github"`

### Requirement: Editor honors pinned set from OpenSpec

When a workflow is opened in the context of an OpenSpec carrying a non-empty `selected_assets` block, the editor SHALL seed the node palette with the pinned set first and SHALL visually mark non-pinned assets as outside-of-pin. Adding a non-pinned asset SHALL prompt the user to either widen the pinned set on the OpenSpec or cancel.

#### Scenario: Pinned skills appear first in palette

- **GIVEN** an OpenSpec with `selected_assets.skills=[skill-a@1.0.0, skill-b@2.1.0]`
- **WHEN** the user opens the editor against that OpenSpec
- **THEN** `skill-a` and `skill-b` MUST appear at the top of the Skills palette
- **AND** other approved skills MUST be visible but tagged `outside-of-pin`

#### Scenario: Adding outside-of-pin asset prompts the user

- **GIVEN** a workflow tied to a pinned OpenSpec
- **WHEN** the user drags `skill-c` (outside-of-pin) onto the canvas
- **THEN** the editor MUST prompt with two options: "Add `skill-c` to OpenSpec pinned set" or "Cancel"
- **AND** save MUST be blocked until the user resolves the prompt
