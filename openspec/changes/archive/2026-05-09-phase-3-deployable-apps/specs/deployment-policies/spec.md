# Spec Delta: deployment-policies (ADDED)

## ADDED Requirements

### Requirement: Production approval

Deployments to `env=prod` MUST be gated by an approved request from a `release-manager`; the approval MUST be tied to the specific `revision_id` and TTL ≤ 8h.

#### Scenario: Block prod deploy without approval

- **GIVEN** a request to deploy `app-foo:abc123` to `env=prod`
- **WHEN** no approved `deployment-approval` exists
- **THEN** the orchestrator MUST stop at the policy stage with `pending_approval`
- **AND** create an entry in the Approvals Inbox

### Requirement: Freeze windows

Configurable freeze windows MUST block deployments to specified envs during defined intervals; overrides require `release-manager` approval and explicit reason.

#### Scenario: Friday-evening freeze blocks deploy

- **GIVEN** a freeze window `Fri 18:00 → Mon 08:00 (env=prod)`
- **WHEN** a deploy is requested at Saturday 10:00
- **THEN** the policy MUST refuse with `freeze_window_active`
- **AND** emit `deployment.policy_evaluated.v1{outcome=denied, reason=freeze_window}`

### Requirement: Require signed image

All deploys MUST verify Cosign signature before `Apply`; the policy MUST be active by default and MUST NOT be globally disabled.

#### Scenario: Reject unsigned image deploy

- **GIVEN** an image without valid Cosign signature
- **WHEN** the policy evaluates `require-signed-image`
- **THEN** the deploy MUST fail with `unsigned_image`
- **AND** emit `deployment.image_verified.v1{outcome=failed}`

### Requirement: Canary or blue/green for high-criticality prod

For `criticality≥high` and `env=prod`, the deploy `strategy` MUST be `canary` or `blue_green`; rolling updates without traffic split MUST be denied.

#### Scenario: Reject rolling deploy on critical prod

- **GIVEN** an asset with `criticality=critical` and a request `strategy=rolling` to `env=prod`
- **WHEN** the policy evaluates
- **THEN** the deploy MUST be denied with `strategy_not_allowed_for_criticality`

### Requirement: Rollback plan required

For `criticality≥high`, the request payload MUST include a `rollback_plan` describing reverse migration handling and verification; absence MUST deny the deploy.

#### Scenario: Reject high-criticality deploy without rollback plan

- **GIVEN** a request for `criticality=high` lacking `rollback_plan`
- **WHEN** the policy evaluates
- **THEN** the deploy MUST be denied with `rollback_plan_missing`
