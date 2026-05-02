# Spec Delta: policies-and-approvals (MODIFIED)

## MODIFIED Requirements

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
