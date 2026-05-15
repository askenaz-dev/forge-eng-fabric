## ADDED Requirements

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

## MODIFIED Requirements

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

### Requirement: Progressive OpenSpec field assembly

The wizard SHALL extract structured fields from each user answer and append them to the draft OpenSpec, producing a per-field completeness map that lists fields still missing required content. The completeness map SHALL include `app_id` as a required field; the draft SHALL never reach "complete" while `app_id` points to `_unassigned`.

#### Scenario: Answer contributes multiple fields

- **WHEN** the user answers "It needs to be in production by end of quarter, owned by HR, and only handles internal data"
- **THEN** the wizard SHALL update `requirements.non_functional.timeline`, `stakeholders.owning_bu`, and `requirements.non_functional.data_classification` in a single turn, and reflect them in the completeness map

#### Scenario: Completeness map flags unassigned app

- **WHEN** the wizard renders state to the user while `draft.app_id` points to `_unassigned`
- **THEN** the UI SHALL mark the `app_id` field as incomplete with a dedicated CTA "Pick an App"
- **AND** the overall draft state SHALL NOT be reportable as `complete`
