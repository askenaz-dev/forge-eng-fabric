## MODIFIED Requirements

### Requirement: Phase-aware capabilities

The platform SHALL provide agentic capabilities mapped to SDLC phases: **Product, Architecture, Design (UI/UX), Development, QA, Security, DevOps, Infrastructure (IaC), SRE, FinOps**. Capabilities MAY be implemented as Skills, Agents or Workflows registered in the Asset Registry. The new Infrastructure phase sits between DevOps and SRE.

#### Scenario: QA capability generates test cases from OpenSpec

- **WHEN** an OpenSpec is updated with new acceptance criteria
- **THEN** the QA capability proposes/updates test cases linked to the OpenSpec, subject to Workspace policy

#### Scenario: Infrastructure phase invoked between DevOps and SRE

- **GIVEN** an App with `targets.iac in {required, optional, opt-in}` and the orchestrator entering the post-DevOps stage
- **WHEN** the next phase is selected
- **THEN** the orchestrator MUST invoke the Infrastructure capability before SRE
- **AND** the Infrastructure gates MUST be evaluated before any SRE step runs

## ADDED Requirements

### Requirement: `targets:` opt-in semantics on the App entity

The orchestrator SHALL consult the App entity's `targets` map to decide whether each SDLC phase is `required`, `optional`, `opt-in` or `skipped`. Semantics:

- `required` — the phase runs and the workflow fails if any required gate fails.
- `optional` — the phase runs; gate failure produces a warning event but does not fail the workflow.
- `opt-in` — the phase is part of the catalog but is not in the default plan; the workflow operator must opt in at start time via an `include=[phase]` flag.
- `skipped` — the phase is removed from the plan entirely.

The orchestrator SHALL refuse to advance past any phase whose gate fails when its target is `required`.

#### Scenario: required gate failure blocks workflow

- **GIVEN** an App with `targets.security=required` and a failing `security_review_passed` gate
- **WHEN** the orchestrator evaluates progression
- **THEN** the workflow MUST stop and emit `sdlc.phase.blocked.v1` with `phase=security, target=required`

#### Scenario: optional gate failure emits warning and continues

- **GIVEN** an App with `targets.design=optional` and a failing `ui_blueprint_present` gate
- **WHEN** the orchestrator evaluates progression
- **THEN** the workflow MUST continue past the design phase
- **AND** emit `sdlc.phase.warning.v1` with `phase=design, target=optional, gate=ui_blueprint_present`

#### Scenario: opt-in phase runs only when explicitly included

- **GIVEN** an App with `targets.iac=opt-in`
- **WHEN** the workflow is started without `include=[iac]`
- **THEN** the orchestrator MUST omit all iac steps from the plan
- **AND** the SRE phase MUST be reachable directly from DevOps

### Requirement: Orchestrator records skip / warning audit trail

The orchestrator SHALL emit a phase-level audit event for every skipped or warned phase including the App's `targets` value at that moment, so audit consumers can reconstruct the decision matrix per run.

#### Scenario: Skipped phase audited

- **GIVEN** an App with `targets.observability=skipped`
- **WHEN** the orchestrator builds the plan
- **THEN** an `sdlc.phase.skipped.v1` event MUST be emitted with `phase=observability, target=skipped, app_id`
