## ADDED Requirements

### Requirement: Conversational intent capture in the Portal

The platform SHALL provide an Intent Capture Wizard in the Portal that accepts a free-text natural-language intent and conducts a guided dialogue until a complete OpenSpec is producible. The wizard SHALL be accessible from the Portal's `alfred` module and SHALL not require knowledge of slash commands or schema field names.

#### Scenario: Non-technical user starts an intent dialogue

- **WHEN** a user with the `workspace.member` role opens the wizard and types "I need an internal HR app to track time-off requests"
- **THEN** the wizard SHALL create a draft OpenSpec scoped to the user's Workspace, render the intent as the `business_intent`, and present the first follow-up question

#### Scenario: Slash-command surface remains available

- **WHEN** a power user navigates to the existing slash-command Alfred Console route
- **THEN** the slash-command interface SHALL function unchanged

### Requirement: Dynamic follow-up question generation

The wizard SHALL generate follow-up questions dynamically from the current draft state, the configured domain templates, and Alfred's RAG knowledge base. Questions SHALL cover at minimum: application type, data sensitivity classification, runtime preference, criticality tier, target environments, owning Business Unit, and budget envelope.

#### Scenario: Question depends on previously captured fields

- **GIVEN** the user has indicated the application processes employee personal data
- **WHEN** the wizard generates the next question
- **THEN** the question SHALL probe data classification (e.g., "Is this restricted PII or internal?") before asking about runtime placement

#### Scenario: RAG-grounded question selection

- **WHEN** the wizard generates a question and the Workspace has prior similar OpenSpecs
- **THEN** the question generator SHALL retrieve relevant prior OpenSpecs via RAG and adapt the question, recording the retrieval references on the draft

### Requirement: Progressive OpenSpec field assembly

The wizard SHALL extract structured fields from each user answer and append them to the draft OpenSpec, producing a per-field completeness map that lists fields still missing required content.

#### Scenario: Answer contributes multiple fields

- **WHEN** the user answers "It needs to be in production by end of quarter, owned by HR, and only handles internal data"
- **THEN** the wizard SHALL update `requirements.non_functional.timeline`, `stakeholders.owning_bu`, and `requirements.non_functional.data_classification` in a single turn, and reflect them in the completeness map

#### Scenario: Completeness map reflects remaining work

- **WHEN** the wizard renders state to the user
- **THEN** the UI SHALL show which sections (`business_intent`, `problem_statement`, `requirements`, `constraints`, `autonomy_policy`) are complete, partial, or empty

### Requirement: Autonomy preset selection during capture

The wizard SHALL surface the Workspace's configured autonomy presets and allow the user to pick one and inline-override individual action classes within the admin-defined ceiling. The selected preset SHALL be embedded in the draft as `autonomy_policy`.

#### Scenario: User picks the workspace default preset

- **WHEN** the user has not modified autonomy in the wizard
- **THEN** the draft SHALL inherit the Workspace's default preset and display its name and effective rules

#### Scenario: User overrides one action class within ceiling

- **WHEN** the user toggles `deploy:prod` from `autonomous` to `manual`
- **THEN** the override SHALL be persisted on the draft, the preset name SHALL be displayed as "<preset> (modified)", and the change SHALL be allowed because tightening within the ceiling is permitted

#### Scenario: User attempts to exceed ceiling

- **WHEN** the user toggles `deploy:prod` from `manual` to `autonomous` while the preset's ceiling forbids autonomous prod deploys
- **THEN** the wizard SHALL reject the change and explain that the Workspace ceiling forbids it

### Requirement: Preview and execute hand-off

When the draft is complete and validates, the wizard SHALL display a final preview of the OpenSpec and offer a single "Ejecutar SDLC" action that commits the draft and triggers the SDLC orchestrator.

#### Scenario: Preview shows the full OpenSpec

- **GIVEN** all required fields are complete
- **WHEN** the wizard transitions to the preview step
- **THEN** the preview SHALL render the full OpenSpec with sections collapsed by default and an explicit "Ejecutar SDLC" CTA

#### Scenario: Execute commits draft and starts SDLC

- **WHEN** the user clicks "Ejecutar SDLC" from the preview
- **THEN** the draft SHALL be committed atomically as a normal OpenSpec and the SDLC orchestrator SHALL receive an `intent.committed.v1` event with the new `openspec_id`

#### Scenario: Cancel returns to draft state

- **WHEN** the user dismisses the preview without confirming
- **THEN** the draft SHALL remain in `drafting` state and resumable later from the wizard

### Requirement: Audit and traceability of wizard interactions

Every wizard turn (user input, generated question, extracted fields, RAG retrievals, policy evaluations, draft commits) SHALL produce an audit record with `correlation_id` and the acting principal.

#### Scenario: Each turn audited

- **WHEN** the user answers a follow-up question
- **THEN** an audit record SHALL be emitted with `event=intent.dialogue.turn`, `draft_id`, `field_changes[]`, `llm_call_id` (if any), and `principal`
