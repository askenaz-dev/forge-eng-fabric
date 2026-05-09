## ADDED Requirements

### Requirement: Reference workflow `forge.reference.intent-to-deploy@1`

The platform SHALL register a versioned workflow named `forge.reference.intent-to-deploy@1` in `workflow-runtime` that orchestrates the full happy-path from a committed OpenSpec to a deployed application across `alfred → sdlc-orchestrator → scaffolder → app-onboarding → ci → deploy-orchestrator`.

#### Scenario: Workflow is discoverable in the registry

- **WHEN** an authorized user lists workflows in the marketplace
- **THEN** `forge.reference.intent-to-deploy@1` SHALL be listed as a `reference` workflow tagged `forge`, with documentation linking to the runbook

#### Scenario: Workflow executes in correct order

- **WHEN** the workflow is triggered with a committed `openspec_id`
- **THEN** it SHALL execute steps in this order: scaffold repository, open PR, run CI with security gates, request HITL approval before production deploy, deploy to the configured runtime
- **AND** each step SHALL emit a structured `step.completed` event referencing the `correlation_id`

### Requirement: HITL approval gate before production deploy

The reference flow SHALL pause for HITL approval before the production deploy step regardless of the OpenSpec's autonomy preset, and the gate SHALL be surfaced in the Portal's approvals queue.

#### Scenario: Gate visible to authorized approver

- **WHEN** the workflow reaches the pre-prod-deploy gate
- **THEN** an approval request SHALL appear in the Workspace's approvals queue with the OpenSpec, build artifact references, and the proposed runtime target

#### Scenario: Approval rejection halts the flow

- **WHEN** an authorized approver rejects the request with a reason
- **THEN** the workflow SHALL terminate with status `aborted_by_approver`, the rejection reason SHALL be recorded in the OpenSpec's decision log, and no production deploy SHALL occur

### Requirement: `make demo-intent-to-deploy` target

The repository root Makefile SHALL expose a `demo-intent-to-deploy` target that triggers the reference workflow against the local stack and prints a human-readable progress report including the resulting deploy URL on success.

#### Scenario: Operator runs the demo

- **GIVEN** the local compose stack is healthy and a Minikube environment is registered
- **WHEN** the operator runs `make demo-intent-to-deploy`
- **THEN** the target SHALL submit a canned intent through Alfred, drive the wizard non-interactively to commit, trigger the reference workflow, auto-approve the HITL gate via a documented test-only fixture, and report success or failure with a final deploy URL or error
- **AND** the run SHALL produce a JSON report at `build/demo-intent-to-deploy/<timestamp>.json` summarizing each step

### Requirement: Smoke test asserts milestone events

CI SHALL run an integration smoke test that triggers the reference workflow against ephemeral infrastructure and asserts the presence and ordering of milestone events.

#### Scenario: Smoke test passes on green path

- **WHEN** the smoke test runs in CI
- **THEN** it SHALL assert that events `intent.committed.v1`, `repo.scaffolded.v1`, `pr.opened.v1`, `ci.passed.v1`, `approval.granted.v1`, `deploy.completed.v1` are emitted in order with consistent `correlation_id`
- **AND** the test SHALL fail if any milestone is missing or out of order

### Requirement: Runbook documents human-facing steps

The platform SHALL publish `docs/runbooks/intent-to-deploy-demo.md` describing prerequisites, environment setup, expected outputs at each step, common failure modes, and rollback steps.

#### Scenario: Runbook reviewed and current

- **WHEN** a release is cut
- **THEN** the runbook SHALL reference the workflow version, the `make demo-intent-to-deploy` flags supported in that release, and the date/version of last validation
