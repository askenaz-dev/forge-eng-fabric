# healing-engine Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements
### Requirement: Five-level healing model

The engine SHALL support 5 levels: L1 (Notify), L2 (Suggest), L3 (Act-with-approval), L4 (Act-autonomously), L5 (Act-and-rollback). Each invocation MUST record `level_decided` and rationale.

#### Scenario: L3 path requires approval

- **GIVEN** an envelope assigning L3 for action `restart-pod` in `env=stage`
- **WHEN** an incident triggers `restart-pod`
- **THEN** the engine MUST create an Approvals Inbox entry with diagnosis context
- **AND** wait for the approval signal up to TTL
- **AND** emit `healing.level_decided.v1{level=L3}` and `healing.triggered.v1{status=waiting_approval}`

#### Scenario: L5 auto-rollback on verify failure

- **GIVEN** action `refresh-cache` allowed at L5 in `env=prod` with `reversible=true`
- **WHEN** the action runs and post-action verify fails
- **THEN** the engine MUST invoke the inverse workflow (rollback)
- **AND** emit `healing.executed.v1{outcome=failed}` followed by `healing.rolled_back.v1`
- **AND** escalate to a human via `healing.escalated.v1`

### Requirement: Envelope enforcement

The engine MUST consult the matching envelope for `(capability, asset, env, criticality)`; actions exceeding `allowed_levels` MUST be degraded to the maximum allowed level.

#### Scenario: Degrade L4 to L3 when envelope caps at L3

- **GIVEN** envelope cap `allowed_levels=[L1,L2,L3]` for `env=prod`
- **WHEN** an action would default to L4
- **THEN** the engine MUST execute at L3 instead
- **AND** emit `healing.level_decided.v1{requested=L4, applied=L3, reason=envelope_cap}`

### Requirement: Kill switch precedence

When the kill switch is active (global or Workspace-scoped), the engine MUST degrade all actions to L1 (Notify-only) until disabled.

#### Scenario: Kill switch suppresses execution

- **GIVEN** the global kill switch is `active=true`
- **WHEN** any incident matches an action
- **THEN** the engine MUST emit `healing.triggered.v1{level=L1, suppressed=true, reason=kill_switch}`
- **AND** NOT invoke the underlying workflow

### Requirement: Action promotion gating

Promotion of an action to L4 or L5 in a given env MUST require: eval suite ≥95% on last 50 runs, ≥20 successful L3 dry-run executions, 30-day postmortem-free window, approval by `platform-admin` and `security-approver`.

#### Scenario: Reject premature promotion

- **GIVEN** an action with only 10 successful L3 runs
- **WHEN** promotion to L4 is requested
- **THEN** the engine MUST refuse with `412 promotion_prerequisites_unmet`
- **AND** report the missing prerequisites

### Requirement: L1 detect — notify with HITL context

The healing engine SHALL run continuous detection over the platform's signal sources (metrics anomalies from Prometheus, CI failures via `ci.failed.v1`, deploy failures via `deployment.failed.v1`, alert threshold crossings, incident creations). Every detection SHALL emit `healing.detected.v1` carrying: the originating signal, the affected asset reference (App, service, runtime), a fresh diagnosis report from `diagnosis-pipeline`, candidate hypotheses with cited evidence, candidate actions from `healing-action-catalog`, and a blast-radius estimate. L1 SHALL NOT execute any action; it SHALL only notify the on-call rotation and produce the inbox card.

#### Scenario: Anomaly produces a notify-only inbox card

- **GIVEN** a Prometheus alert `HighP95Latency` firing for `app-1` in `env=prod`
- **WHEN** the healing engine processes the alert
- **THEN** the engine MUST emit `healing.detected.v1` with the alert payload, a diagnosis report from `diagnosis-pipeline`, ranked hypotheses with cited evidence and candidate actions
- **AND** the engine MUST NOT invoke any action from the catalog at this level
- **AND** an inbox card MUST appear in the Approvals Inbox tagged `healing-l1` with all the context

#### Scenario: L1 envelope cap honored

- **GIVEN** an envelope cap `allowed_levels=[L1]` for `(asset=app-1, env=prod)`
- **WHEN** detection would have proposed any non-L1 action
- **THEN** the engine MUST clamp the response to L1 (notify only)
- **AND** emit `healing.level_decided.v1{requested=L2, applied=L1, reason=envelope_cap}`

### Requirement: L2 propose-fix — suggest a patch through the Approvals Inbox

When the envelope allows L2 for the affected scope, the engine SHALL pick the highest-confidence hypothesis from the L1 diagnosis and produce a **proposed fix**: either a code diff against the App's repo (for code-rooted issues like a regression spotted by triage) or a configuration change (for config-rooted issues like a runtime parameter). The proposed fix SHALL be rendered in the Approvals Inbox card with a file-level diff and the citations from the diagnosis. Approving the card SHALL transition the case to L3 (existing flow); rejecting it SHALL record the reason and close the case.

#### Scenario: Code-rooted issue produces a diff PR-comment-style preview

- **GIVEN** a CI failure traced to a regression in `services/payments/internal/router.go` and an envelope permitting L2
- **WHEN** the engine generates the propose-fix
- **THEN** the inbox card MUST render a file-level diff for `services/payments/internal/router.go` with syntax highlighting and the diagnosis citations
- **AND** emit `healing.fix_proposed.v1` with the diff hash, the affected files and the OpenSpec link

#### Scenario: Config-rooted issue produces a YAML diff

- **GIVEN** an SLA breach traced to insufficient replicas
- **WHEN** the engine generates the propose-fix
- **THEN** the inbox card MUST render a YAML diff against `infra/<app-slug>/helm/values-prod.yaml` increasing `replicaCount` and emitting the rationale citation
- **AND** the card MUST surface the expected blast-radius (e.g., "scales prod from 3 to 5 replicas, ~+0.4 vCPU and ~+1Gi memory")

#### Scenario: Approval transitions to L3 execution

- **GIVEN** an approved L2 propose-fix card for `(asset=app-1, action=scale-up)`
- **WHEN** the approver clicks "Approve"
- **THEN** the engine MUST hand off to L3 execution against the catalog action `scale-up` with the approved parameters
- **AND** emit `healing.fix_approved.v1{level_from=L2, level_to=L3}`

#### Scenario: Rejection records reason and closes the case

- **GIVEN** an L2 propose-fix card
- **WHEN** the approver rejects with reason "false positive — known maintenance window"
- **THEN** the engine MUST mark the case `rejected`, emit `healing.fix_rejected.v1` with the reason
- **AND** add the rejection to the L2 training feedback corpus for the next eval cycle

### Requirement: L2 propose-fix safety eval before surfacing

The engine SHALL run a safety eval on every proposed fix before surfacing it to the inbox. The eval SHALL verify: the diff does not touch any file in the App's `protected_paths[]`, the patch is below the configured size budget (default 200 changed lines or `protected_paths=*` configured by tenant), no secret references appear in the diff. Failed evals SHALL downgrade the case to L1 with the original diagnosis.

#### Scenario: Diff exceeding size budget downgraded

- **GIVEN** a generated propose-fix with 500 changed lines (above 200-line default)
- **WHEN** the safety eval runs
- **THEN** the engine MUST downgrade the case to L1
- **AND** emit `healing.fix_downgraded.v1{from=L2, to=L1, reason=size_budget_exceeded}`

#### Scenario: Diff touching protected path downgraded

- **GIVEN** a generated propose-fix that modifies a file under `protected_paths=["security/**"]`
- **THEN** the eval MUST reject the patch and downgrade to L1

### Requirement: L1/L2 events feed the Friendly view's "Operar" card

The "Operar" card in the Alfred Console Friendly view SHALL surface `healing.detected.v1` and `healing.fix_proposed.v1` events for Apps the user can view, rendering them as plain-language summaries (no raw IDs, friendly action names).

#### Scenario: Operar card shows recent detections

- **GIVEN** a user with `app#viewer` on `app-1` and a recent `healing.detected.v1` for `app-1`
- **WHEN** the user opens the Friendly view's "Operar" card
- **THEN** the panel MUST list the detection summarised in plain language, e.g., "Detecté latencia alta en la App 'Time Off Tracker' en producción. Hay un fix propuesto pendiente de tu aprobación."
- **AND** the inbox card MUST be reachable in one click

