## ADDED Requirements

### Requirement: Alfred runs dedup retrieval before persisting a new draft

For every new intent capture flow (Friendly view conversation, Advanced view `/forge` invocation, wizard, dock empty-state), Alfred SHALL run the dedup retrieval (`POST /v1/intent/match`) before persisting a draft spec. If the top hit's score meets the tenant threshold, the dialogue API SHALL return a `spec_match` block carrying the candidate(s) so the consuming UI can render the match dialog. The caller MUST send `bypass_match=true` to skip the pass (used by "Crear nuevo" and by automation that has already resolved the question).

#### Scenario: Match block returned on intent start

- **GIVEN** a workspace `ws-1` with `app-1` and a similar existing spec `spec-7`
- **WHEN** any UI calls `POST /v1/intent/start` with `{workspace_id: ws-1, app_id: app-1, intent: "Necesito una app para registrar vacaciones"}`
- **THEN** Alfred MUST first run the dedup retrieval, and if the top hit scores ≥ threshold MUST return `200 OK` with a body containing `spec_match: { candidate: { spec_id: spec-7, score, lifecycle_state, ... }, threshold }` and no `draft_id`
- **AND** the response MUST NOT persist a draft yet
- **AND** an `alfred.intent.match_found.v1` event MUST be emitted

#### Scenario: bypass_match skips dedup

- **WHEN** the UI calls `POST /v1/intent/start` with `bypass_match=true`
- **THEN** Alfred MUST skip the dedup retrieval, persist a fresh draft and return the `draft_id` + first question

#### Scenario: resume_spec_id extends an existing spec

- **WHEN** the UI calls `POST /v1/intent/start` with `resume_spec_id=spec-7`
- **THEN** Alfred MUST hydrate the existing spec into the dialogue context, persist a continuation draft referencing `spec-7`, and proceed with a continuation prompt instead of the fresh-intent prompt

### Requirement: Dialogue context carries view marker for persona rendering

Alfred's dialogue API SHALL accept a `view: "friendly" | "advanced"` field on `POST /v1/intent/start` and on `POST /v1/intent/answer`. The view marker SHALL be propagated to the LLM call so the persona rendering (label vs raw ID, citation footer style) matches the consumer surface. Audit events SHALL include `view` for slice-and-dice.

#### Scenario: Friendly view triggers label-only rendering

- **WHEN** any call carries `view=friendly`
- **THEN** Alfred's response MUST replace every raw entity ID in the rendered text with the entity's human label (using the platform label resolver)
- **AND** the audit event MUST include `view=friendly`

#### Scenario: Advanced view preserves raw IDs

- **WHEN** any call carries `view=advanced`
- **THEN** Alfred's response MAY include raw IDs in the rendered text
- **AND** the audit event MUST include `view=advanced`
