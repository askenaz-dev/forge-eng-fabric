# spec-deduplication Specification

## Purpose

Intent-deduplication layer — a Milvus-backed retrieval endpoint that detects similar existing specs before a new draft is persisted, surfaces a match dialog with Extender / Crear nuevo / Implementar actions, keeps the dedup index in sync with spec lifecycle events, and emits telemetry for retrieval relevance training. Created by archiving change `alfred-console-redesign`.

## Requirements

### Requirement: Intent dedup endpoint

The platform SHALL expose `POST /v1/intent/match` accepting `{ workspace_id, app_id?, text, k? }` and returning `{ matches: [{spec_id, title, score, lifecycle_state, summary, source}], threshold, scope }`. The endpoint SHALL run a Milvus retrieval over the relevant corpus (App-scoped if `app_id` is set, Workspace-scoped otherwise), return the top `k` (default 5) candidates with cosine similarity scores, and indicate the threshold currently in effect for the tenant.

#### Scenario: Match returned for similar intent

- **GIVEN** a workspace `ws-1` with `app-1` and an existing spec `spec-7` titled "Time Off Tracker"
- **WHEN** a caller calls `POST /v1/intent/match` with `{workspace_id: ws-1, app_id: app-1, text: "Necesito una app para registrar vacaciones del equipo"}`
- **THEN** the response MUST list `spec-7` with `score >= 0.80`, `lifecycle_state` and a summary
- **AND** `scope` MUST equal `app:app-1`

#### Scenario: Scope falls back to workspace when no app_id

- **WHEN** the caller omits `app_id`
- **THEN** the retrieval scope MUST be the Workspace corpus
- **AND** `scope` MUST equal `workspace:<ws_id>`

#### Scenario: Below-floor matches never returned

- **GIVEN** a tenant with `spec_match.threshold=0.65` (the hard floor)
- **WHEN** the top hit's score is 0.61
- **THEN** the response MUST return an empty `matches` array
- **AND** the response MUST include `threshold=0.65` and the score the top hit reached for telemetry

### Requirement: Tenant-tunable threshold with a hard floor

The match threshold SHALL be tunable per tenant via `tenant.spec_match.threshold` with a hard platform floor of `0.65`. Values below the floor SHALL be rejected at write time. The platform default SHALL be `0.80`.

#### Scenario: Threshold below floor rejected

- **WHEN** a tenant admin sets `tenant.spec_match.threshold=0.50`
- **THEN** the platform MUST reject with `422 threshold_below_floor`
- **AND** the error MUST include the floor value

#### Scenario: Threshold defaults to 0.80

- **GIVEN** a freshly created tenant
- **WHEN** `POST /v1/intent/match` is called
- **THEN** the response MUST include `threshold=0.80`

### Requirement: Match dialog actions: Extender, Crear nuevo, Implementar

When `POST /v1/intent/match` returns a top hit with `score >= threshold`, the consuming UI (both Friendly and Advanced views, the wizard, the dock) SHALL present a match dialog with the following actions, in order:

- If the top hit's `lifecycle_state in {approved, committed}`: **Implementar** (primary), **Extender**, **Crear nuevo**.
- Otherwise: **Extender** (primary), **Crear nuevo**, "Ver otros similares" (secondary link expanding the remaining candidates).

The dialog MUST also include an explicit "No, esto no es lo mismo" feedback button that emits `alfred.intent.match_dismissed.v1` carrying the rejected `spec_id` and the original intent text, for retrieval relevance training.

#### Scenario: Committed match promotes Implementar

- **GIVEN** a match `spec-7` with `lifecycle_state=committed`
- **WHEN** the dialog is rendered
- **THEN** "Implementar" MUST be the primary CTA at the bottom-right
- **AND** "Extender" MUST be the secondary CTA
- **AND** "Crear nuevo" MUST be the tertiary CTA

#### Scenario: Implementar bypasses the wizard

- **WHEN** the user clicks "Implementar" for a committed `spec-7`
- **THEN** the UI MUST call `POST /v1/agent-mode/sessions` with `{openspec_id: spec-7, start_step: "architect"}`
- **AND** the wizard MUST NOT be opened
- **AND** the agent-mode session MUST emit `alfred.agent_mode.session_started.v1` with `start_step=architect`

#### Scenario: Crear nuevo bypasses the match

- **WHEN** the user clicks "Crear nuevo"
- **THEN** the UI MUST call `POST /v1/intent/start` with `bypass_match=true`
- **AND** a fresh draft spec MUST be created without referencing `spec-7`

#### Scenario: Match dismissed feedback recorded

- **WHEN** the user clicks "No, esto no es lo mismo"
- **THEN** `alfred.intent.match_dismissed.v1` MUST be emitted with the `spec_id`, the user's intent text and the score
- **AND** the dialog MUST close and the flow MUST continue as if no match had been found

### Requirement: Dedup index reacts to spec lifecycle events

The dedup index SHALL react to OpenSpec lifecycle events so that retrieved candidates remain accurate. On `spec.purged.v1` the spec MUST be removed from the index. On `spec.reparented.v1` the spec's `app_id` MUST be updated in the index metadata. On `intent.committed.v1` the spec MUST be indexed if not already present. On lifecycle transitions the cached `lifecycle_state` MUST be updated.

#### Scenario: Purged spec disappears from retrieval

- **GIVEN** an indexed `spec-7` referenced in the App corpus
- **WHEN** the migration job emits `spec.purged.v1` for `spec-7`
- **THEN** within 60 seconds subsequent `POST /v1/intent/match` calls MUST NOT return `spec-7` as a candidate

#### Scenario: Re-parented spec follows the new App

- **GIVEN** an indexed `spec-7` with `app_id=app-1`
- **WHEN** `spec.reparented.v1` fires with `from_app_id=app-1, to_app_id=app-2`
- **THEN** an App-scoped retrieval against `app-1` MUST NOT return `spec-7`
- **AND** an App-scoped retrieval against `app-2` MUST return `spec-7`

### Requirement: Telemetry for match outcomes

The platform SHALL emit `alfred.intent.match_found.v1` whenever the match dialog is shown to the user, and `alfred.intent.match_dismissed.v1` whenever the user explicitly rejects the match via the "No, esto no es lo mismo" button. Both events SHALL include the principal, App scope, threshold in effect, candidate `spec_id`, score and the intent text (truncated to 280 chars).

#### Scenario: Dialog open emits match_found

- **WHEN** the dialog opens for a candidate `spec-7` with score 0.86 and threshold 0.80
- **THEN** `alfred.intent.match_found.v1` MUST be emitted with `{spec_id: spec-7, score: 0.86, threshold: 0.80}` and the principal
