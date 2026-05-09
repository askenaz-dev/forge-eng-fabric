# Spec Delta: workflow-versioning (ADDED)

## ADDED Requirements

### Requirement: SemVer immutable versions

Workflow versions MUST follow SemVer and be immutable once published; updates require new versions.

#### Scenario: Reject overwrite of published version

- **GIVEN** workflow `wf-1@1.2.0` already published
- **WHEN** a publish attempt targets `1.2.0` again
- **THEN** the registry MUST refuse with `409 version_already_exists`

### Requirement: Breaking-change detection

The registry MUST automatically classify changes between versions and require MAJOR bump for breaking changes (input/output schema reductions, output removals).

#### Scenario: Auto-bump MAJOR on input field removal

- **GIVEN** `wf-1@1.2.0` declares input `story` (required)
- **WHEN** a new version removes `story`
- **THEN** the detector MUST flag the change as breaking
- **AND** require MAJOR bump (minimum next version `2.0.0`)
- **AND** refuse publication as `1.3.0`

### Requirement: Pinned installations

Installations to Workspaces MUST pin to an exact version; updates require explicit re-install.

#### Scenario: Existing installs unaffected by new minor version

- **GIVEN** Workspace `ws-1` installed `wf-1@1.2.0`
- **WHEN** `wf-1@1.3.0` is published
- **THEN** `ws-1` MUST continue executing `1.2.0`
- **AND** an upgrade prompt MUST appear in the marketplace UI
