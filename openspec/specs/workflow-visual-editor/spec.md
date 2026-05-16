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

The platform SHALL record the visual editor implementation choice as an Architecture Decision Record. ADR-0001 (`docs/governance/adrs/0001-workflow-visual-editor.md`) SHALL be set to status `Superseded` with a forward reference to ADR-0002. ADR-0002 (`docs/governance/adrs/0002-canvas-react-flow.md`) SHALL record the React Flow + custom-canvas decision, alternatives considered (Flowise embed, n8n fork, Drawflow/LiteFlow, native SVG), consequences, and review date. The new ADR SHALL be referenced from this capability and from `docs/platform-enablement.md`.

#### Scenario: Superseded ADR-0001 and accepted ADR-0002 exist

- **WHEN** a contributor reads this capability or `docs/platform-enablement.md`
- **THEN** ADR-0001 SHALL show status `Superseded by ADR-0002` with a link
- **AND** ADR-0002 SHALL be in `Accepted` status with the React Flow choice, alternatives, consequences, and review date

### Requirement: Editor integrated into the Portal

The visual editor SHALL be embedded in the Portal at `/workflows/editor` and SHALL operate within the Portal's authentication, authorization, and Workspace context. The user-facing copy SHALL refer to AI Flows (ES: "Flujos AI") sourced from `portal/src/i18n/dictionary.ts`. Internal identifiers (routes, API paths, service names) SHALL keep `workflow` to preserve backward compatibility.

#### Scenario: Editor opens within a Workspace

- **WHEN** a user with `workflow.author` permission navigates to `/workflows/editor` in their Workspace
- **THEN** the editor SHALL load with the Workspace context applied
- **AND** SHALL display the canvas, trigger palette, node palette, property panel, and dry-run drawer
- **AND** SHALL only display nodes and assets visible to the Workspace per OpenFGA

#### Scenario: Editor honors approval policies

- **WHEN** a user without `workflow.author` permission attempts to open the editor
- **THEN** the Portal SHALL deny access and surface a clear permission-error message localized via `portal/src/i18n/dictionary.ts`

#### Scenario: User-facing copy reads "AI Flows"

- **WHEN** the editor renders chrome (page title, headings, palette section labels, save button)
- **THEN** copy SHALL render as "AI Flows" / "Flujos AI" sourced from the dictionary

### Requirement: Canonical node catalog rendered

The editor SHALL render the canonical node catalog. The catalog SHALL be split into four palette sections:

- **Triggers**: `manual`, `cron`, `webhook-in`, `event-bus`, `email-inbound`.
- **AI**: `llm`, `agent`, `prompt-template`.
- **Actions**: `mcp`, `skill`, `webhook` (outbound), `github-action`, `deploy-action`, `approval-action`, `notification-action`.
- **Logic**: `branch`, `loop`, `retry`, `human-in-the-loop`, `eval`.

A `custom` node category SHALL appear once at least one custom node is registered for the workspace, rendered under its declared `category`. Additional node types SHALL be added only via OpenSpec change with capability impact.

#### Scenario: Palette renders the four canonical sections

- **WHEN** a user opens the node palette
- **THEN** the palette SHALL show four sections in order: Triggers, AI, Actions, Logic
- **AND** every canonical node type SHALL be present in its section
- **AND** assets in non-`approved` state SHALL be visually marked and disallowed in saved workflows

### Requirement: Canvas based on React Flow

The visual editor SHALL be built on `@xyflow/react` (MIT). Canvas behaviors (drag-drop, pan, zoom, minimap, keyboard navigation, edge routing) SHALL be implemented using the library's primitives. Custom node renderers SHALL live under `portal/src/components/flow/nodes/` and SHALL implement the `FlowNodeRenderer` interface defined by the custom-node SDK. The canvas SHALL persist canonical AST via the `ast-canvas-adapter` (renamed from `flowise-adapter`), with round-trip parity preserved by an existing-style test.

#### Scenario: Canvas round-trips AST losslessly

- **GIVEN** a canonical AST with two triggers, an LLM node, a branch, and an MCP action
- **WHEN** the AST is loaded via the adapter, modified by adding a node, and saved
- **THEN** the persisted AST MUST contain the original nodes plus the new one
- **AND** the round-trip test `ast-canvas-adapter` MUST pass

#### Scenario: Canvas accessibility primitives present

- **WHEN** a keyboard-only user opens the editor
- **THEN** every palette item SHALL be reachable via Tab
- **AND** Enter on a focused palette item SHALL add the node to the canvas
- **AND** the canvas SHALL announce node additions via an ARIA live region

### Requirement: Trigger palette section

The editor SHALL render a `Triggers` palette section above the node palette. Triggers SHALL be dragged onto a dedicated canvas region distinct from steps, and the canvas SHALL visually indicate the trigger → steps flow. A workflow MAY have zero triggers (invoke-only) and the canvas SHALL surface a `Triggered by: Manual invoke` placeholder when no trigger is present.

#### Scenario: Add an email trigger from the palette

- **WHEN** a user drags `email-inbound` onto the trigger region
- **THEN** the canvas SHALL render the trigger node
- **AND** the property panel SHALL prompt the user to select a mailbox secret and optional filter
- **AND** the canonical AST SHALL include the trigger in `spec.triggers` on save

#### Scenario: Invoke-only workflow renders placeholder

- **WHEN** a workflow has no triggers
- **THEN** the trigger region SHALL render `Triggered by: Manual invoke` and `Show how to invoke this flow` link to docs

### Requirement: LLM node configuration panel

When an `llm` node is selected on the canvas, the property panel SHALL allow the author to: pick a prompt template (filtered to `approved` assets in the workspace), pick a model via `model-gateway` (filtered to the workspace's `allowed_models` whitelist if declared), set per-node overrides (`temperature`, `max_tokens`, `top_p`), select tools from the workflow's in-scope MCPs (sourced from `selected_assets.mcps` when pinned), define the output schema, and set `max_tool_calls`. The panel SHALL surface estimated cost-per-execution computed from the selected model's per-token pricing × prompt template token count + per-tool-call estimate.

#### Scenario: Author configures an LLM node end-to-end

- **WHEN** the author selects an `llm` node and fills the property panel
- **THEN** the panel SHALL update the canonical AST step in real time
- **AND** the dry-run drawer SHALL show the estimated cost
- **AND** saving SHALL persist the full LLM step shape per the `llm-flow-node` spec

### Requirement: Code view tab

The editor SHALL provide a `Code view` tab that renders the current AST as canonical YAML. Edits in code view SHALL update the canvas in real time and vice versa. The YAML SHALL be the exact canonical form persisted by `workflow-registry`.

#### Scenario: Edits in code view reflect in canvas

- **GIVEN** a workflow open in the editor with the canvas tab active
- **WHEN** the user switches to the `Code view` tab and edits the YAML to add a `cron` trigger
- **THEN** switching back to the canvas SHALL render the new trigger node
- **AND** saving SHALL persist the trigger

#### Scenario: Invalid YAML in code view blocks save

- **WHEN** the user edits the YAML to invalid syntax
- **THEN** the editor SHALL surface the parse error inline
- **AND** the save button SHALL be disabled until the YAML parses

### Requirement: Library surface at `/workflows`

The route `/workflows` SHALL render the AI Flow library: a list of workflows scoped to the active Workspace, filterable by name/tags, with version count and last-published timestamp per row. Clicking a row SHALL navigate to `/workflows/[id]/history` showing the version timeline and the diff viewer. Editing SHALL happen exclusively at `/workflows/editor`.

#### Scenario: Library lists workflows for the workspace

- **WHEN** an authorized user navigates to `/workflows`
- **THEN** the page SHALL list every workflow visible per OpenFGA
- **AND** each row SHALL show name, id, visibility, version count, last published

#### Scenario: History sub-route shows version diff

- **WHEN** a user clicks a row and lands on `/workflows/[id]/history`
- **THEN** the page SHALL list versions newest-first and render the existing diff viewer comparing any two selected versions

### Requirement: Reference AI-Flow demo published

The platform SHALL publish a reference workflow `forge.reference.ai-email-triage@1` exercising: an `email-inbound` trigger, an `llm` classify-and-draft step, a `branch` on classification, and an `mcp` outbound webhook for the send. The workflow SHALL be the basis of an end-to-end Playwright test that drag-builds it and dry-runs it.

#### Scenario: Reference flow exists in the registry

- **WHEN** a user opens the workflow library in a workspace where the reference flow is provisioned
- **THEN** `forge.reference.ai-email-triage@1` SHALL appear in the library
- **AND** opening it in the editor SHALL render the canvas with all four primitive families (trigger, AI, logic, action)

#### Scenario: Playwright e2e dry-runs the reference flow

- **WHEN** the Playwright suite `portal/tests/e2e/ai-email-triage.spec.ts` runs
- **THEN** the test SHALL drag-build the flow, configure each node, and trigger a dry-run
- **AND** the dry-run drawer SHALL show each step's mock input/output
- **AND** no real MCP/email/LLM call SHALL be made

### Requirement: Editor persists canonical AST in `workflow-registry`

The editor SHALL persist workflows by writing the canonical AST to `workflow-registry`. Workflows SHALL be versioned: each save SHALL produce a new immutable version, and the editor SHALL allow opening prior versions read-only.

#### Scenario: Save creates a new version

- **WHEN** a user saves changes to a workflow
- **THEN** `workflow-registry` SHALL persist a new version with monotonically increasing `version` and SHALL retain the prior version

#### Scenario: Open historical version read-only

- **WHEN** a user opens a non-latest version
- **THEN** the editor SHALL render the workflow read-only and SHALL offer "fork as new latest" as the only mutating action

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
