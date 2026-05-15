## ADDED Requirements

### Requirement: Pin assets step in the wizard

The Intent Capture Wizard SHALL include a **Pin assets** step that surfaces the three gateway catalogs (skills, MCPs, agents) filtered to `lifecycle_state=approved`, `active_surface ≠ null` and Workspace-visible. The user SHALL be able to pin zero or more assets per family into the draft OpenSpec under a `selected_assets: { skills: [], mcps: [], agents: [] }` block. The step SHALL be optional — an empty pinned set SHALL preserve current orchestration behavior.

#### Scenario: User pins skills, MCPs and agents from gateway catalogs

- **GIVEN** a draft OpenSpec scoped to Workspace `ws-1`
- **WHEN** the user opens the Pin assets step and selects `skill-a@1.0.0`, `mcp-github`, `agent-architect`
- **THEN** the wizard MUST persist `selected_assets.skills=[skill-a@1.0.0]`, `selected_assets.mcps=[mcp-github]`, `selected_assets.agents=[agent-architect]`
- **AND** each pin MUST reference the asset id, version and `active_surface.endpoint`

#### Scenario: Pin rejected for non-approved asset

- **GIVEN** a skill `skill-x` in `lifecycle_state=in_review`
- **WHEN** the user attempts to pin `skill-x`
- **THEN** the wizard MUST refuse the pin with reason `asset_not_approved`
- **AND** display the lifecycle state in the error

#### Scenario: Pin rejected when active surface is missing

- **GIVEN** an asset `skill-y` in `lifecycle_state=approved` but with `active_surface=null` (registry data integrity gap)
- **WHEN** the user attempts to pin `skill-y`
- **THEN** the wizard MUST refuse the pin with reason `missing_active_surface`

### Requirement: Wizard validates pinned set against Workspace visibility

The wizard SHALL validate that every pinned asset is visible to the user's Workspace per OpenFGA at submission and at every commit. Assets that become invisible between pinning and commit SHALL be flagged and removed from the pinned set with an audit trail.

#### Scenario: Asset visibility revoked between pin and commit

- **GIVEN** a draft where `skill-a` was pinned by the user
- **WHEN** OpenFGA revokes the user's visibility to `skill-a` and the user later clicks Ejecutar SDLC
- **THEN** the wizard MUST surface a notice listing `skill-a` as removed from the pinned set
- **AND** emit an audit event recording the auto-removal with `reason=visibility_revoked`

### Requirement: Pinned set travels with the OpenSpec into orchestration

When the wizard commits the draft, the `selected_assets` block SHALL be persisted on the OpenSpec and SHALL be carried into the `intent.committed.v1` event consumed by the SDLC orchestrator.

#### Scenario: Pinned set in the intent event

- **GIVEN** a draft with `selected_assets.skills=[skill-a@1.0.0]`
- **WHEN** the user commits via Ejecutar SDLC
- **THEN** the `intent.committed.v1` event MUST include the `selected_assets` block verbatim
- **AND** the SDLC orchestrator MUST acknowledge receipt with the pinned set listed in the run record
