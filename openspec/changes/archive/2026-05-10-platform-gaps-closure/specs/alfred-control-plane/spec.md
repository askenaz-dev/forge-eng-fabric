## ADDED Requirements

### Requirement: Wizard-driven dialogue mode

Alfred SHALL expose a dialogue API consumed by the Portal's Intent Capture Wizard, allowing a non-technical user to author an OpenSpec through a guided conversation. The dialogue surface SHALL coexist with the existing slash-command interface; neither SHALL be deprecated by this change.

#### Scenario: Wizard starts a dialogue against Alfred

- **WHEN** the Portal calls `POST /v1/intent/start` with a free-text intent
- **THEN** Alfred SHALL create a draft OpenSpec scoped to the caller's Workspace, return a `draft_id` plus the first follow-up question, and emit an `intent.dialogue.started.v1` audit event with the caller's principal

#### Scenario: Wizard answer advances the dialogue

- **WHEN** the Portal calls `POST /v1/intent/answer` with a `draft_id` and the user's answer
- **THEN** Alfred SHALL extract structured fields, persist them on the draft, decide on the next question or signal completion, and return the updated state
- **AND** Alfred SHALL emit an `intent.dialogue.turn.v1` audit event with the field changes and any LLM call ID via LiteLLM

#### Scenario: Wizard commits the draft

- **WHEN** the Portal calls `POST /v1/intent/commit` with a complete and validated `draft_id`
- **THEN** Alfred SHALL transition the draft to `committed` atomically, return the canonical `openspec_id`, and emit `intent.committed.v1` for the SDLC orchestrator to consume

### Requirement: Workspace autonomy presets surfaced by Alfred

Alfred SHALL expose Workspace-scoped autonomy presets (named bundles of `autonomy_policy`) via a read API consumed by the Portal, with a per-action-class ceiling that bounds user-level overrides.

#### Scenario: Wizard lists available presets

- **WHEN** the Portal calls `GET /v1/workspaces/{id}/autonomy/presets`
- **THEN** Alfred SHALL return the configured presets, mark one as the Workspace default, and include the per-action-class ceilings

#### Scenario: User override within ceiling accepted

- **WHEN** an answer or explicit toggle modifies an autonomy field within the ceiling
- **THEN** Alfred SHALL accept the modification and persist it on the draft

#### Scenario: User override beyond ceiling rejected

- **WHEN** an attempted override would exceed the ceiling (e.g., enable autonomous prod deploy when the ceiling forbids it)
- **THEN** Alfred SHALL reject the modification with a structured error, and Alfred SHALL emit an `autonomy.override.rejected.v1` audit event

### Requirement: Admin-managed autonomy preset configuration

Workspace admins SHALL be able to author, edit, and disable autonomy presets through a documented administrative surface. Preset changes SHALL be audited and SHALL not affect already-committed OpenSpecs retroactively.

#### Scenario: Admin authors a new preset

- **WHEN** a Workspace admin calls the preset write API with a new named preset
- **THEN** the preset SHALL be persisted, audited with the admin's principal, and made available to wizard sessions started after the change

#### Scenario: Already-committed OpenSpecs unaffected

- **WHEN** an admin disables a preset that some committed OpenSpecs reference
- **THEN** the disable SHALL only prevent new OpenSpecs from selecting it; existing OpenSpecs SHALL continue to enforce their embedded `autonomy_policy`
