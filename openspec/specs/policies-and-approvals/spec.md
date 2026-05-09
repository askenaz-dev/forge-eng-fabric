# policies-and-approvals Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

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

### Requirement: Onboarding policy templates

The policy engine SHALL ship with policy templates specific to onboarding: `require-approval-for-repo-creation`, `require-openspec-link`, `enforce-branch-protections`, `bypass-gate`, `merge-without-openspec`, `relax-branch-protection`, `allow-force-push`. Templates MUST be parameterizable per Workspace and criticality.

#### Scenario: Apply default onboarding policies on Workspace creation

- **GIVEN** a new Workspace is created in Phase 2
- **WHEN** default policies are applied
- **THEN** `require-openspec-link` MUST be active for `criticality≥medium`
- **AND** `enforce-branch-protections` MUST be active for all criticalities
- **AND** the policies MUST be visible in the Portal

### Requirement: Override TTL and audit

All onboarding-related overrides MUST have an explicit TTL (≤24h), be approved by a `security-approver` or `release-manager`, be auto-reverted at expiration, and be fully audited via `policy.override.granted.v1`, `policy.override.consumed.v1`, `policy.override.expired.v1`.

#### Scenario: Override expires and reverts

- **GIVEN** an approved override `relax-branch-protection` with TTL=4h applied to `main`
- **WHEN** 4 hours elapse
- **THEN** the engine MUST re-apply the standard branch protections
- **AND** emit `policy.override.expired.v1` with override id and revert details

#### Scenario: Reject override without approver role

- **GIVEN** a requester without `security-approver` or `release-manager` role
- **WHEN** they attempt to grant an override
- **THEN** the engine MUST refuse with `403 insufficient_role`
- **AND** record the denial in audit

### Requirement: Deployment policy templates

The policy engine SHALL provide deployment-specific templates: `require-approval-prod`, `freeze-window`, `require-signed-image`, `require-canary`, `require-rollback-plan`, `allow-unsigned-image` (override), `allow-non-reversible-rollback` (override).

#### Scenario: Templates available on Workspace creation

- **GIVEN** a new Workspace created in Phase 3 or later
- **WHEN** policy templates are listed
- **THEN** the deployment templates above MUST be available with sensible defaults
- **AND** `require-signed-image` MUST be active and not disable-able globally

### Requirement: Approvals Inbox supports deployment approvals

The Approvals Inbox SHALL accept items of type `deployment-approval` carrying `revision_id`, `target_env`, `runtime_id`, `image_digest`, `rollback_plan_summary`, `risk_summary`.

#### Scenario: Release manager reviews production deploy

- **GIVEN** a pending `deployment-approval` for `app-foo` to `env=prod`
- **WHEN** a `release-manager` opens the inbox
- **THEN** they MUST see image digest, OpenSpec links, PR sha, rollback plan summary
- **AND** be able to approve/deny with mandatory rationale
- **AND** the decision MUST emit `approval.decision.v1`

### Requirement: SDLC gate policy templates

The policy engine SHALL provide templates for SDLC gates: `require-architecture-review`, `require-threat-model`, `require-test-coverage`, `require-perf-budget`, `require-security-clean`, `require-slo-defined`, `require-runbook`, `require-on-call`, `require-cost-estimate`, `phase-progression-bypass` (override).

#### Scenario: Templates available for new Workspaces

- **GIVEN** a Workspace created in Phase 4 or later
- **WHEN** policy templates are listed
- **THEN** SDLC gate templates MUST be available with criticality-aware defaults

### Requirement: Phase progression bypass override

`phase-progression-bypass` MUST require approval by `release-manager`, mandatory rationale, TTL ≤ 24h, single-use, and full audit.

#### Scenario: Single-use bypass cannot be reused

- **GIVEN** an approved bypass for `phase=qa` of `initiative-3`
- **WHEN** the bypass is consumed once
- **THEN** subsequent attempts to use it MUST fail with `403 override_already_consumed`
- **AND** emit `policy.override.exhausted.v1`

### Requirement: Workflow publish/install policy templates

The policy engine SHALL provide templates `require-eval-pass`, `require-security-review`, `require-tenant-share-approval`, `forge-certification-prerequisites`.

#### Scenario: Block publish without eval pass

- **GIVEN** policy `require-eval-pass` active for Workspace `ws-1`
- **WHEN** a publish is attempted without a passing eval run
- **THEN** the publish MUST be denied with `eval_pass_missing`

### Requirement: Tenant share approval

`require-tenant-share-approval` template MUST gate promotions to `visibility=tenant` requiring `tenant-admin` approval.

#### Scenario: Tenant promotion approval flow

- **GIVEN** a request to promote a workflow to `tenant` visibility
- **WHEN** policy evaluates
- **THEN** an Approvals Inbox entry MUST be created for `tenant-admin`
- **AND** until approved, visibility MUST remain at the prior tier

### Requirement: Autonomy envelope policy template

The policy engine SHALL provide template `autonomy-envelope` allowing per-(capability, asset_pattern, env, criticality) configuration of `default_level`, `allowed_levels`, `time_windows`, `max_actions_per_hour`.

#### Scenario: Envelope applied per request

- **GIVEN** envelope for `capability=sdlc-devops, asset_pattern=application/svc-*, env=prod, criticality=high`
- **WHEN** an action targeting `application/svc-foo` triggers in prod
- **THEN** the engine MUST consult that envelope first
- **AND** apply the constraints

### Requirement: Kill switch policy

The policy engine SHALL expose a `kill-switch` toggle global and per-Workspace; activation MUST require role `platform-admin` or `security-approver` and MUST be auditable.

#### Scenario: Kill switch activation logged

- **GIVEN** a `platform-admin` activates the global kill switch
- **WHEN** the action persists
- **THEN** the audit log MUST contain actor, timestamp, reason
- **AND** all healing engines MUST observe the change within 30s (cache TTL)

### Requirement: L5 reversibility constraint

Policy `require-reversible-for-l5` MUST be active and MUST refuse any L5 promotion or execution for actions with `reversible=false`.

#### Scenario: Reject L5 execution for non-reversible action

- **GIVEN** an action `delete-pvc` with `reversible=false` mistakenly assigned L5
- **WHEN** the engine evaluates
- **THEN** the policy MUST refuse with `412 reversibility_required_for_l5`
- **AND** degrade to the highest reversible level allowed
