## ADDED Requirements

### Requirement: Policy evaluation engine
The platform SHALL provide a policy engine that evaluates declarative policies (YAML/CEL) per Workspace, OpenSpec, asset, environment and criticality, returning `allow`, `requires_approval` (with required approvers) or `deny` with rationale.

#### Scenario: Policy denies action
- **WHEN** an action triggers a policy that evaluates to `deny`
- **THEN** the engine returns the denial with rationale, the caller blocks the action and emits an audit event with the policy decision

### Requirement: Approval requests are durable, notified and tracked
When a policy evaluates to `requires_approval`, the engine SHALL create a durable approval request with: subject, scope, action, requested_by, required_approvers, expiration, criticality and `correlation_id`. Notifications SHALL be sent via configured channels.

#### Scenario: Approval request notifies approvers and survives restarts
- **WHEN** an approval request is created
- **THEN** notifications are sent to all required approvers, the request is persisted, survives restarts, and is visible in the Approvals Inbox

### Requirement: Approvals Inbox in the Portal
The Portal SHALL provide an **Approvals Inbox** where authorized approvers see pending requests scoped to them, with full context (intent, OpenSpec, policy decision, proposed action, telemetry preview), and can approve or reject with comments.

#### Scenario: Approver acts on a pending request
- **WHEN** an authorized approver approves or rejects a request
- **THEN** the engine resumes or aborts the action, emits an `approval.decided.v1` event and audits actor/decision/timestamp

### Requirement: Configurable scope of policies
Policies SHALL be configurable by **Workspace, OpenSpec, asset (and trust level), environment, action type, criticality, role and responsible person**. The most restrictive applicable policy SHALL win.

#### Scenario: OpenSpec policy is more restrictive than Workspace default
- **WHEN** Workspace default is `autonomous` for `deploy:staging` but the OpenSpec requires approval
- **THEN** the engine requires approval for that OpenSpec's staging deploys

### Requirement: Approval expirations and escalation
Approval requests SHALL have an expiration. Expired requests SHALL be automatically rejected (or escalated per policy) and audited.

#### Scenario: Request expires without decision
- **WHEN** an approval request reaches its expiration without a decision
- **THEN** the engine marks it as `expired`, the originating action is aborted (or escalated per policy) and an audit event is emitted
