# intent-capture-wizard Specification

## Purpose
TBD - created by archiving change platform-gaps-closure. Update Purpose after archive.
## Requirements

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

The wizard SHALL extract structured fields from each user answer and append them to the draft OpenSpec, producing a per-field completeness map that lists fields still missing required content. The completeness map SHALL include `app_id` as a required field; the draft SHALL never reach "complete" while `app_id` points to `_unassigned`.

#### Scenario: Answer contributes multiple fields

- **WHEN** the user answers "It needs to be in production by end of quarter, owned by HR, and only handles internal data"
- **THEN** the wizard SHALL update `requirements.non_functional.timeline`, `stakeholders.owning_bu`, and `requirements.non_functional.data_classification` in a single turn, and reflect them in the completeness map

#### Scenario: Completeness map reflects remaining work

- **WHEN** the wizard renders state to the user
- **THEN** the UI SHALL show which sections (`business_intent`, `problem_statement`, `requirements`, `constraints`, `autonomy_policy`) are complete, partial, or empty

#### Scenario: Completeness map flags unassigned app

- **WHEN** the wizard renders state to the user while `draft.app_id` points to `_unassigned`
- **THEN** the UI SHALL mark the `app_id` field as incomplete with a dedicated CTA "Pick an App"
- **AND** the overall draft state SHALL NOT be reportable as `complete`

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

When the draft is complete, validates, **and `draft.app_id` does not point to `_unassigned`**, the wizard SHALL display a final preview of the OpenSpec and offer a single "Ejecutar SDLC" action that commits the draft and triggers the SDLC orchestrator. If `draft.app_id` still points to `_unassigned` at preview time, the wizard SHALL block the commit, surface the App scope step inline, and refuse to call `POST /v1/intent/commit` until a real App is selected.

#### Scenario: Preview shows the full OpenSpec

- **GIVEN** all required fields are complete and `draft.app_id` references a non-unassigned App
- **WHEN** the wizard transitions to the preview step
- **THEN** the preview SHALL render the full OpenSpec with sections collapsed by default, the resolved App badge at the top, and an explicit "Ejecutar SDLC" CTA

#### Scenario: Execute commits draft and starts SDLC

- **WHEN** the user clicks "Ejecutar SDLC" from the preview and `draft.app_id` is resolved
- **THEN** the draft SHALL be committed atomically as a normal OpenSpec with the resolved `app_id` and the SDLC orchestrator SHALL receive an `intent.committed.v1` event with the new `openspec_id` and `app_id`

#### Scenario: Cancel returns to draft state

- **WHEN** the user dismisses the preview without confirming
- **THEN** the draft SHALL remain in `drafting` state and resumable later from the wizard

#### Scenario: Commit refused while app scope is unassigned

- **GIVEN** a complete draft whose `app_id` still points to `_unassigned`
- **WHEN** the user clicks "Ejecutar SDLC"
- **THEN** the wizard MUST refuse the commit, scroll the user back to the App scope step, and emit an audit event `intent.commit_blocked.v1` with `reason=unassigned_app`

### Requirement: Audit and traceability of wizard interactions

Every wizard turn (user input, generated question, extracted fields, RAG retrievals, policy evaluations, draft commits) SHALL produce an audit record with `correlation_id` and the acting principal.

#### Scenario: Each turn audited

- **WHEN** the user answers a follow-up question
- **THEN** an audit record SHALL be emitted with `event=intent.dialogue.turn`, `draft_id`, `field_changes[]`, `llm_call_id` (if any), and `principal`

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

### Requirement: App scope is the first step of the wizard

The Intent Capture Wizard SHALL begin every new draft with an **App scope** step that resolves the destination App before any intent body is captured. The step SHALL offer three branches: (a) **extend an existing App** (lists Apps in the current Workspace visible to the caller), (b) **create a new App** (collects `name`, `slug`, `description`, at least one `owner`), (c) **decide later** (parks the draft against the workspace's `_unassigned` App and surfaces a persistent banner until a real App is selected).

#### Scenario: Wizard offers existing Apps first

- **GIVEN** a Workspace `ws-1` with Apps `hr-portal` and `internal-billing` visible to the caller
- **WHEN** the wizard renders the App scope step
- **THEN** the step MUST list `hr-portal` and `internal-billing` as primary options, with their description and last activity
- **AND** the "create new App" option MUST be a secondary action below the list

#### Scenario: New App created inline

- **WHEN** the user picks "create a new App" and submits `name=Time Off Tracker, slug=time-off-tracker, owners=[me]`
- **THEN** the wizard MUST create the App via `POST /v1/workspaces/ws-1/apps`, set `draft.app_id` to the new App id
- **AND** the wizard MUST proceed to the intent capture step with the new App as the anchor

#### Scenario: Decide later parks the draft against unassigned

- **WHEN** the user picks "decide later"
- **THEN** `draft.app_id` MUST point to the workspace's `_unassigned` App
- **AND** the wizard MUST render a sticky banner reading "Pick an App before committing this intent"

### Requirement: Design System selection step in the wizard (new-App branch)

The Intent Capture Wizard SHALL include a **Design System** step that activates only on the "create a new App" branch of the App scope step. The step SHALL list the four built-in templates (`desing-system-1..4`) with their screenshots and use-case copy, render a live preview panel that mounts the chosen tokens onto a sample composition (a button stack, a KPI tile, a card with a run row), and persist the selection on the draft App as `design_system_ref`. The step SHALL default to `ds-forge-default`. If the user is on the "extend an existing App" branch, the step SHALL be skipped (the App already has a Design System).

#### Scenario: Step appears only when creating a new App

- **GIVEN** a wizard session on the "extend an existing App" branch with `draft.app_id=app-1`
- **WHEN** the wizard advances past the App scope step
- **THEN** the Design System step MUST be skipped
- **AND** the draft MUST NOT contain a `design_system_selection_pending` flag

#### Scenario: Step shows four templates with previews

- **GIVEN** a wizard session on the "create a new App" branch
- **WHEN** the user reaches the Design System step
- **THEN** the step MUST display `desing-system-1`, `desing-system-2`, `desing-system-3`, `desing-system-4` as selectable cards with screenshots in both light and dark
- **AND** the preview panel MUST render a sample composition with the tokens of the currently focused template

#### Scenario: Default is ds-forge-default

- **WHEN** the user lands on the Design System step without an explicit choice
- **THEN** the step MUST highlight `desing-system-1` (resolved via the `ds-forge-default` alias) as the default
- **AND** clicking "Continue" without changing the selection MUST persist `design_system_ref=ds-forge-default`

#### Scenario: Selection persists on the draft App

- **WHEN** the user picks `desing-system-3` and clicks "Continue"
- **THEN** the wizard MUST persist `design_system_ref=desing-system-3@<latest_approved_version>` on the draft App in the wizard state
- **AND** the App creation call invoked at commit time MUST carry that value

### Requirement: Live preview panel renders real tokens

The Design System preview panel SHALL fetch the tokens CSS sheet of the selected template (using its sha256-pinned URL from the asset manifest) and apply it to a sandboxed sample composition. The panel SHALL render the sample in both light and dark themes simultaneously (side-by-side). The panel SHALL NOT leak the tokens into the wizard's own chrome.

#### Scenario: Preview applies tokens in isolation

- **WHEN** the user focuses `desing-system-2`
- **THEN** the preview MUST apply `desing-system-2`'s tokens to a sandboxed composition (e.g., a shadow DOM or a scoped `style[scoped]`)
- **AND** the wizard's own chrome (sidebar, top bar, footer) MUST continue to render with the Portal's design system

#### Scenario: Preview renders both themes side-by-side

- **WHEN** the preview panel is open
- **THEN** the panel MUST show two columns labelled "Claro" / "Oscuro" with the same sample composition, each with the corresponding theme applied

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
