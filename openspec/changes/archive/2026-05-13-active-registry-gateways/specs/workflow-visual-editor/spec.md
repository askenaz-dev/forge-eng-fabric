## MODIFIED Requirements

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

## ADDED Requirements

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
