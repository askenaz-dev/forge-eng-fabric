# intent-to-infrastructure-reference-flow Specification

## Purpose
TBD - created by archiving change sdlc-end-to-end. Update Purpose after archive.
## Requirements
### Requirement: Reference workflow `forge.reference.intent-to-infrastructure@1`

The platform SHALL register a versioned workflow named `forge.reference.intent-to-infrastructure@1` in `workflow-runtime` that orchestrates the full SDLC chain from a committed OpenSpec to a deployed, observed application across `alfred → sdlc-orchestrator → scaffolder → app-onboarding → architect → design → development → qa → security → devops → iac → deploy → sre → observability`. The workflow SHALL coexist with `forge.reference.intent-to-deploy@1` (the smaller default) without replacing it.

#### Scenario: Workflow registered as reference

- **WHEN** an authorized user lists workflows in the marketplace
- **THEN** `forge.reference.intent-to-infrastructure@1` SHALL be listed as a `reference` workflow tagged `forge`, with documentation linking to the runbook
- **AND** the existing `forge.reference.intent-to-deploy@1` SHALL still be listed unchanged

#### Scenario: Workflow plan reflects App targets

- **GIVEN** an App `app-1` with `targets.design=required, targets.iac=opt-in, targets.observability=skipped`
- **WHEN** the workflow is started for `app-1` without opt-in flags
- **THEN** the plan MUST include `design` as a required step
- **AND** the plan MUST NOT include `iac` (opt-in, not requested) or `observability` (skipped)

#### Scenario: Opt-in flags override App default

- **GIVEN** an App with `targets.iac=opt-in`
- **WHEN** the workflow is started with `include=[iac]`
- **THEN** the plan MUST include the `iac` step
- **AND** the run MUST emit `workflow.intent_to_infrastructure.opt_in.v1` with the included steps

### Requirement: Step order and event emission

The workflow SHALL execute its steps in the documented order and each step SHALL emit a structured event referencing the workflow `correlation_id`, the App's `app_id` and the OpenSpec `openspec_id`.

#### Scenario: Step order honored

- **WHEN** the workflow runs end-to-end for an App with all targets `required`
- **THEN** the orchestrator MUST emit events in order: `intent.committed.v1`, `repo.scaffolded.v1`, `sdlc.adr.proposed.v1`, `sdlc.api_contract.proposed.v1`, `sdlc.data_model.proposed.v1`, `sdlc.threat_model.completed.v1`, `sdlc.ui_blueprint.proposed.v1`, `sdlc.component_stubs.committed.v1`, `sdlc.accessibility_audit.completed.v1`, `pr.opened.v1`, `ci.passed.v1`, `sdlc.test_plan.proposed.v1`, `sdlc.iac.generated.v1`, `sdlc.iac.validated.v1`, `sdlc.iac.applied.v1`, `deploy.completed.v1`, `observability.dashboards.provisioned.v1`
- **AND** the test SHALL fail if any milestone is missing or out of order

#### Scenario: Optional step failure produces warning not failure

- **GIVEN** a run where `targets.design=optional` and the design step fails
- **WHEN** the workflow evaluates the gate
- **THEN** the run MUST continue to the next step
- **AND** emit `workflow.intent_to_infrastructure.warning.v1` with the failed step and reason

### Requirement: HITL approval gates inherit from policy

The workflow SHALL pause for HITL approval at every step whose action class resolves to `requires_approval` under the App's `autonomy_policy`. The L2 healing propose-fix gates SHALL also be HITL by definition.

#### Scenario: Production deploy gate inherits from policy

- **WHEN** the workflow reaches the deploy step for `env=prod` and the App's policy marks `deploy:prod=requires_approval`
- **THEN** the workflow MUST pause, open an approval card in the Approvals Inbox, and emit `workflow.paused_for_approval.v1`

### Requirement: `make demo-intent-to-infrastructure` target

The repository root Makefile SHALL expose a `demo-intent-to-infrastructure` target that exercises the new workflow end-to-end against the local stack with a canned intent and a fully-required `targets` matrix. The target SHALL print a JSON report at `build/demo-intent-to-infrastructure/<timestamp>.json` summarising each step and including the final deploy URL plus observability dashboard URLs.

#### Scenario: Demo target completes successfully on local stack

- **GIVEN** a healthy local compose stack and a Minikube environment registered
- **WHEN** an operator runs `make demo-intent-to-infrastructure`
- **THEN** the target SHALL submit a canned intent, drive the new workflow end-to-end (auto-approving HITL gates via documented test-only fixtures), and exit `0` on success
- **AND** the JSON report SHALL list every step with its event name, duration and outcome
- **AND** the report SHALL include the resulting deploy URL and the URL(s) of the provisioned observability dashboards

### Requirement: Smoke test asserts the full event chain

CI SHALL run an integration smoke test against ephemeral infrastructure that runs the workflow end-to-end and asserts the milestones listed in the "Step order honored" scenario, in order, with consistent `correlation_id`, `app_id` and `openspec_id`.

#### Scenario: Smoke test fails on missing observability step

- **GIVEN** an `App.targets.observability=required` configuration
- **WHEN** the smoke test detects no `observability.dashboards.provisioned.v1` emission
- **THEN** the test MUST fail with a clear error pointing at the missing milestone
- **AND** the run MUST NOT be reported as passing

### Requirement: Runbook documents human-facing steps

The platform SHALL publish `docs/runbooks/intent-to-infrastructure-demo.md` describing prerequisites, environment setup, expected outputs at each step, common failure modes, and rollback steps for the new workflow. The runbook SHALL be updated whenever a step is added/removed and SHALL reference the workflow version and the last validation date.

#### Scenario: Runbook reviewed and current

- **WHEN** a release is cut
- **THEN** the runbook SHALL reference the workflow version, the date/version of last validation, the supported `make demo-intent-to-infrastructure` flags, and screenshots of the Approvals Inbox showing each HITL gate

