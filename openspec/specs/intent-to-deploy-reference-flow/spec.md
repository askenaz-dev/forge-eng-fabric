# intent-to-deploy-reference-flow Specification

## Purpose
TBD - created by archiving change platform-gaps-closure. Update Purpose after archive.
## Requirements

### Requirement: Reference workflow `forge.reference.intent-to-deploy@1`

The platform SHALL register a versioned workflow named `forge.reference.intent-to-deploy@1` in `workflow-runtime` that orchestrates the full happy-path from a committed OpenSpec to a deployed application across `alfred → sdlc-orchestrator → scaffolder → app-onboarding → ci → deploy-orchestrator`. When agent-mode is enabled for the workspace, the workflow SHALL be triggered and supervised by an Alfred **agent-mode session** as its default operator; direct manual triggers SHALL remain valid for break-glass use.

#### Scenario: Workflow is discoverable in the registry

- **WHEN** an authorized user lists workflows in the marketplace
- **THEN** `forge.reference.intent-to-deploy@1` SHALL be listed as a `reference` workflow tagged `forge`, with documentation linking to the runbook
- **AND** the listing SHALL note that Alfred agent-mode is the default operator when the workspace flag is on

#### Scenario: Workflow executes in correct order under Alfred supervision

- **WHEN** the workflow is triggered from an Alfred agent-mode session with a committed `openspec_id`
- **THEN** it SHALL execute steps in this order: scaffold repository, open PR, run CI with security gates, request HITL approval before production deploy, deploy to the configured runtime
- **AND** each step SHALL emit a structured `step.completed` event referencing both the workflow `correlation_id` and the agent-mode `session_id`

#### Scenario: Manual break-glass trigger still works

- **WHEN** an operator triggers the workflow directly (without agent-mode) via the workflow-runtime CLI or API
- **THEN** the workflow SHALL execute the same steps in the same order, without an `alfred_agent_session` row, and SHALL be auditable identically to today

### Requirement: HITL approval gate before production deploy

The reference flow SHALL pause for HITL approval before the production deploy step regardless of the OpenSpec's autonomy preset, and the gate SHALL be surfaced in the Portal's approvals queue. When the workflow is operated by an Alfred agent-mode session, the gate SHALL additionally surface inside the Alfred dock for the originator of the session.

#### Scenario: Gate visible to authorized approver

- **WHEN** the workflow reaches the pre-prod-deploy gate
- **THEN** an approval request SHALL appear in the Workspace's approvals queue with the OpenSpec, build artifact references, and the proposed runtime target
- **AND** if the workflow is operated by an agent-mode session, the same approval SHALL appear inside the Alfred dock for the session originator as a `paused_for_approval` step with a one-click `Approvals` deep link

#### Scenario: Approval rejection halts the flow

- **WHEN** an authorized approver rejects the request with a reason
- **THEN** the workflow SHALL terminate with status `aborted_by_approver`, the rejection reason SHALL be recorded in the OpenSpec's decision log, and no production deploy SHALL occur
- **AND** the parent agent-mode session (when present) SHALL transition to `aborted` with the same rejection reason persisted on its row

### Requirement: `make demo-intent-to-deploy` target

The repository root Makefile SHALL expose a `demo-intent-to-deploy` target that triggers the reference workflow against the local stack and prints a human-readable progress report including the resulting deploy URL on success. The target SHALL default to the agent-mode-supervised path when `ALFRED_AGENT_MODE_ENABLED=true` and SHALL accept `--no-agent-mode` to force the legacy direct-trigger path.

#### Scenario: Operator runs the demo with agent-mode on

- **GIVEN** the local compose stack is healthy, a Minikube environment is registered, and `ALFRED_AGENT_MODE_ENABLED=true`
- **WHEN** the operator runs `make demo-intent-to-deploy`
- **THEN** the target SHALL submit a canned intent through Alfred, drive the wizard non-interactively to commit, start an Alfred agent-mode session against the committed OpenSpec, auto-approve the HITL gate via a documented test-only fixture, and report success or failure with a final deploy URL or error
- **AND** the run SHALL produce a JSON report at `build/demo-intent-to-deploy/<timestamp>.json` summarizing each step and including the `session_id`

#### Scenario: Operator can force the legacy path

- **WHEN** the operator runs `make demo-intent-to-deploy NO_AGENT_MODE=1`
- **THEN** the target SHALL trigger the workflow directly and produce the same JSON report shape with `session_id: null`

### Requirement: Smoke test asserts milestone events

CI SHALL run an integration smoke test that triggers the reference workflow against ephemeral infrastructure and asserts the presence and ordering of milestone events. When agent-mode is exercised, the smoke test SHALL also assert the parallel agent-mode event sequence.

#### Scenario: Smoke test passes on green path

- **WHEN** the smoke test runs in CI against the agent-mode-supervised path
- **THEN** it SHALL assert that events `intent.committed.v1`, `alfred.agent_mode.session_started.v1`, `repo.scaffolded.v1`, `pr.opened.v1`, `ci.passed.v1`, `alfred.agent_mode.paused_for_approval.v1`, `approval.granted.v1`, `alfred.agent_mode.resumed.v1`, `deploy.completed.v1`, `alfred.agent_mode.completed.v1` are emitted in order with consistent `correlation_id`
- **AND** the test SHALL fail if any milestone is missing or out of order

### Requirement: Runbook documents human-facing steps

The platform SHALL publish `docs/runbooks/intent-to-deploy-demo.md` describing prerequisites, environment setup, expected outputs at each step, common failure modes, and rollback steps. The runbook SHALL cover both the agent-mode-supervised path (default) and the break-glass direct-trigger path.

#### Scenario: Runbook reviewed and current

- **WHEN** a release is cut
- **THEN** the runbook SHALL reference the workflow version, the `make demo-intent-to-deploy` flags supported in that release (`NO_AGENT_MODE`), the date/version of last validation, and a screenshot of the Alfred dock during the agent-mode-supervised run
