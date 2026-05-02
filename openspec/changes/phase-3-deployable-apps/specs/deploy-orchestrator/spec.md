# Spec Delta: deploy-orchestrator (ADDED)

## ADDED Requirements

### Requirement: Stage pipeline with events

Every deployment MUST execute the stages: preflight → policy → image-verify → render → apply → verify → notify; each stage MUST emit a CloudEvent capturing inputs, outputs, duration, and outcome.

#### Scenario: Successful end-to-end deploy

- **GIVEN** a request `POST /v1/deployments` for asset `app-foo` to runtime `rt-1` env `dev`
- **WHEN** the pipeline runs without failures
- **THEN** events MUST be emitted in order: `deployment.requested.v1`, `deployment.policy_evaluated.v1`, `deployment.image_verified.v1`, `deployment.applied.v1`, `deployment.verified.v1`
- **AND** the deployment record MUST contain `revision_id`, `image_digest`, `runtime_id`, `env`, `openspec_ids`, `pr_sha`

### Requirement: Idempotency

Requests with the same `request_id` MUST be idempotent: a duplicate returns the existing deployment state without re-applying.

#### Scenario: Duplicate request returns existing state

- **GIVEN** a completed deployment with `request_id=req-7`
- **WHEN** a duplicate `POST /v1/deployments` arrives with `request_id=req-7`
- **THEN** the response MUST be `200` with the existing `deployment_id` and current `status`
- **AND** no new side effects MUST occur

### Requirement: Verify and auto-rollback

After `Apply`, the orchestrator MUST run health checks (HTTP probe + rollout status) within a configurable timeout; on failure with `auto_rollback=true`, it MUST re-apply the previous revision.

#### Scenario: Auto-rollback on health check failure

- **GIVEN** an env with `auto_rollback=true`
- **WHEN** post-apply verification fails
- **THEN** the orchestrator MUST trigger rollback to the previous `revision_id`
- **AND** emit `deployment.failed.v1` followed by `deployment.rolled_back.v1`
- **AND** notify the Approvals Inbox for review

### Requirement: Manual rollback

Rollback MAY be triggered manually via `POST /v1/deployments/{id}/rollback`; it MUST follow the same stage discipline as a forward deploy and require policy evaluation.

#### Scenario: Manual rollback on production

- **GIVEN** a deployment in `env=prod`
- **WHEN** an operator with role `release-manager` invokes manual rollback
- **THEN** the orchestrator MUST evaluate policies (including `require-approval-prod` for the rollback target)
- **AND** re-apply the previous revision on approval
- **AND** record `rollback_record` and emit `deployment.rolled_back.v1`

### Requirement: Live status streaming

`GET /v1/deployments/{id}/stream` SHALL provide an SSE stream emitting each stage transition with structured payload.

#### Scenario: Operator observes a live deployment

- **GIVEN** a deployment in progress
- **WHEN** an operator opens the SSE stream
- **THEN** they MUST receive `stage.started`, `stage.completed`, `stage.failed` events with timestamps
