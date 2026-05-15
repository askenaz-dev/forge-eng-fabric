## ADDED Requirements

### Requirement: Wizard surfaces the dedup dialog before its first question

When the wizard is entered (regardless of view), it SHALL surface the match dialog if `POST /v1/intent/start` returned a `spec_match` block. The first dialogue turn SHALL NOT be rendered until the user resolves the match dialog (picks Extender / Crear nuevo / Implementar / dismisses).

#### Scenario: Wizard pauses on match block

- **GIVEN** a wizard session whose `POST /v1/intent/start` response carried a `spec_match` block for `spec-7` with score 0.86
- **WHEN** the wizard renders
- **THEN** the wizard MUST render the match dialog above any draft question
- **AND** the wizard MUST NOT call `POST /v1/intent/answer` until the user resolves the dialog

#### Scenario: Extender path resumes the existing spec

- **WHEN** the user clicks "Extender" for `spec-7` from the match dialog
- **THEN** the wizard MUST re-issue `POST /v1/intent/start` with `resume_spec_id=spec-7`
- **AND** continue the dialogue against the resumed spec context

#### Scenario: Implementar path bypasses the wizard

- **WHEN** the user clicks "Implementar" for a committed match
- **THEN** the wizard MUST close itself, hand off to `POST /v1/agent-mode/sessions {openspec_id, start_step: "architect"}`
- **AND** route the user to the agent-mode session detail page

### Requirement: Friendly-rendered wizard hides raw IDs

When the wizard is opened from the Friendly view (`view=friendly`), the wizard SHALL render every entity by its human label, never by its raw ID, in titles, prompts, completeness map and preview. The preview's "Ejecutar SDLC" CTA SHALL be relabelled "Empezar" / "Start" in Friendly view.

#### Scenario: Preview screen uses friendly copy

- **GIVEN** a wizard session with `view=friendly`
- **WHEN** the preview screen renders
- **THEN** the CTA MUST read "Empezar" (ES) / "Start" (EN)
- **AND** the preview MUST NOT show the spec's UUID
