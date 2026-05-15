## MODIFIED Requirements

### Requirement: Supported asset types

The Registry SHALL support exactly six asset types: **MCP Server, Agent Skill, Agent, Workflow, Prompt Template, Design System**. Each asset SHALL be uniquely identified, typed, versioned (SemVer) and owned by a team.

#### Scenario: Publish an MCP Server asset

- **WHEN** a team publishes a new MCP Server with name, version, owner and metadata
- **THEN** the Registry persists the asset, assigns lifecycle state `proposed`, and emits a publication event

#### Scenario: Publish a Design System asset

- **WHEN** the design team publishes a new Design System with `name`, `version`, `owner`, `manifest.tokens`, `manifest.components`, `manifest.fonts`, `manifest.screenshots`, `manifest.use_case`
- **THEN** the Registry persists the asset with `type=design_system`, `lifecycle_state=proposed`, and emits `asset.design_system.published.v1`

#### Scenario: Reject unknown asset type

- **WHEN** any caller submits an asset with `type` not in {mcp-server, agent-skill, agent, workflow, prompt-template, design-system}
- **THEN** the Registry MUST reject with `422 unsupported_asset_type`

## ADDED Requirements

### Requirement: Design System asset metadata

A Design System asset record SHALL include `manifest.tokens` (HTTPS URL to the token CSS sheet, sha256-pinned), `manifest.components` (HTTPS URL to the component pack archive, sha256-pinned), `manifest.fonts` (array of font preload entries `{family, weights, italic, source}`), `manifest.screenshots` (`{light: url, dark: url}` with both URLs sha256-pinned), `manifest.use_case` (a string of at most 240 characters describing the intended look-and-feel). Submissions missing any of these MUST be rejected with `422 missing_design_system_manifest_field`.

#### Scenario: Manifest missing screenshots is rejected

- **WHEN** a publisher submits a Design System asset without `manifest.screenshots.dark`
- **THEN** the Registry MUST reject with `422 missing_design_system_manifest_field` listing `manifest.screenshots.dark`

#### Scenario: Tokens URL must be sha256-pinned

- **WHEN** a publisher submits `manifest.tokens=https://example/tokens.css` without an explicit `manifest.tokens_sha256`
- **THEN** the Registry MUST reject with `422 missing_token_digest`

### Requirement: Design System eval scores

Approval of a Design System asset SHALL require `eval_scores.accessibility >= 0.9` (Axe-derived) and `eval_scores.brand_fidelity >= 0.8` (rubric-driven). Built-in templates SHALL ship with these scores attached at publication.

#### Scenario: Approve fails under accessibility threshold

- **GIVEN** a Design System submission with `eval_scores.accessibility=0.85`
- **WHEN** the transition to `approved` is requested
- **THEN** the Registry MUST refuse with `409 eval_below_threshold` listing the failing dimension
