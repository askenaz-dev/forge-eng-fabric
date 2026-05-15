## ADDED Requirements

### Requirement: Dual-approval flow for autonomous code-fix PRs

When Alfred opens a PR through `POST /v1/code-fixes/open-pr` and the risk classifier returns `approvers ∈ {"admin|owner", "dual"}`, the approvals engine SHALL create an approval request that any single approver from the required set (or both, for `dual`) can decide. The required approver set for `mutate-code` SHALL include the workspace admin role AND the app owner(s) declared in the asset's metadata.

#### Scenario: Single-approver path (admin OR owner)

- **WHEN** Alfred opens a PR for app `acme/payments` and policy returns `approvers:"admin|owner"`
- **THEN** the approvals engine SHALL create a request visible to (a) workspace admins of `acme` and (b) listed owners of `payments`
- **AND** approval by either party SHALL satisfy the request and unblock the action

#### Scenario: Dual-approver path

- **WHEN** policy returns `approvers:"dual"` (e.g., irreversible migration)
- **THEN** the engine SHALL require two distinct approvers from the required set
- **AND** the same human SHALL NOT be allowed to provide both approvals (distinct subject IDs)

### Requirement: Approvals link to triggering symptom

Every approval row created in response to a `system:alfred`-initiated action SHALL persist `triggered_by = symptom_id` and `session_id` alongside the existing `requested_by`. The Approvals Inbox SHALL surface this provenance so the approver sees the symptom that caused the request.

#### Scenario: Approver sees symptom context

- **WHEN** an admin opens an approval request created by Alfred
- **THEN** the inbox SHALL display the originating symptom (fingerprint, evidence excerpt, evidence_ref), the proposed action descriptor, the policy decision, and the sandbox verification artifact if any

#### Scenario: Audit row preserves provenance

- **WHEN** an approval is decided
- **THEN** the audit row SHALL include `triggered_by`, `session_id`, `approvers[]`, `decision`, and the resulting action's `audit_event_id` (linked via `correlation_id`)

### Requirement: Approver self-revocation window

After approving an autonomous action, the approver SHALL have a configurable window (default 60 seconds) during which they MAY rescind the approval. Within that window, if the action has not yet started its mutating phase, it SHALL be aborted; if it has started but is reversible, the platform SHALL trigger rollback; if it is irreversible, rescission SHALL be recorded but cannot reverse the action.

#### Scenario: Rescission before mutation

- **WHEN** an admin approves a restart action and rescinds 10 seconds later, before the executor dispatches the restart
- **THEN** the executor SHALL receive the rescission signal, mark the step `aborted_by_approver_rescission`, and SHALL NOT call the platform-ops endpoint

#### Scenario: Rescission after irreversible action

- **WHEN** an admin rescinds after an irreversible migration has already completed
- **THEN** the rescission SHALL be recorded as audit metadata
- **AND** SHALL NOT roll back the action (no rollback exists); SHALL escalate to incident response per the configured runbook

## MODIFIED Requirements

### Requirement: Approval requests are durable, notified and tracked

When a policy evaluates to `requires_approval` or `requires_dual_approval`, the engine SHALL create a durable approval request with: subject, scope, action, `requested_by` (human user or `system:alfred`), `triggered_by` (optional `symptom_id` for autonomous triggers), `session_id` (optional for autonomous triggers), required_approvers, approver_set_semantics (`any` for single-approver, `dual` for two-of-set), expiration, criticality, and `correlation_id`. Notifications SHALL be sent via configured channels.

#### Scenario: Approval request notifies approvers and survives restarts

- **WHEN** an approval request is created (whether by a human action or by Alfred)
- **THEN** notifications SHALL be sent to all required approvers via the configured channel, the request SHALL be persisted with all provenance fields, survive restarts, and be visible in the Approvals Inbox

#### Scenario: Autonomous-trigger approval includes symptom context

- **WHEN** the approval request is created with `requested_by="system:alfred"` and `triggered_by=<symptom_id>`
- **THEN** the persisted record SHALL link the symptom event, session id, and the policy bundle hash so the approver and auditors can reconstruct the decision context
