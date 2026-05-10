## ADDED Requirements

### Requirement: Progressive draft state for OpenSpecs

The OpenSpec backbone SHALL support a `status` lifecycle of `drafting → validating → committed → abandoned`. Drafts SHALL share the same `openspec_id` namespace as committed OpenSpecs and SHALL be persisted across sessions, but SHALL NOT satisfy production-relevance gates until they reach `committed`.

#### Scenario: Draft persists across sessions

- **WHEN** a user starts an intent dialogue, leaves, and returns later
- **THEN** the draft SHALL be retrievable by the same `draft_id`/`openspec_id` and resumable from the last persisted state

#### Scenario: Draft cannot satisfy production-relevance gates

- **WHEN** a deployment to production references an OpenSpec in `drafting` state
- **THEN** the platform SHALL block the deployment with an error stating that only committed OpenSpecs satisfy production gates

#### Scenario: Atomic commit transitions draft to committed

- **WHEN** the wizard or any caller invokes the commit API on a complete draft
- **THEN** the OpenSpec SHALL transition from `drafting` to `committed` in a single atomic operation; partial commits SHALL NOT be observable

### Requirement: Per-field completeness reporting

The OpenSpec backbone SHALL expose a completeness API that returns, for any draft, which required and recommended fields are complete, partial, or empty, sufficient for the wizard to choose the next question.

#### Scenario: Completeness reflects current draft

- **WHEN** the wizard calls `GET /v1/openspecs/{id}/completeness`
- **THEN** the response SHALL list each section (`business_intent`, `problem_statement`, `requirements.functional`, `requirements.non_functional`, `constraints`, `autonomy_policy`, `stakeholders`, `success_metrics`) with status `complete | partial | empty` and field-level detail

#### Scenario: Completeness updates after a turn

- **WHEN** an answer adds or modifies fields on a draft
- **THEN** the next call to the completeness API SHALL reflect the changes within the same logical transaction

### Requirement: Draft expiry and cleanup

Drafts in `drafting` state SHALL expire after 14 days of inactivity. Expiry SHALL transition the draft to `abandoned` and SHALL NOT delete the audit trail.

#### Scenario: Inactive draft expires

- **WHEN** a draft has had no activity for 14 days
- **THEN** the next cleanup run SHALL transition it to `abandoned` and emit an `intent.draft.abandoned.v1` event

#### Scenario: Audit trail preserved on expiry

- **WHEN** a draft transitions to `abandoned`
- **THEN** all dialogue turns, field changes, and policy decisions SHALL remain in audit storage subject to the data retention policy
