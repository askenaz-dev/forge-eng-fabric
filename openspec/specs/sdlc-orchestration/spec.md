# sdlc-orchestration Specification

## Purpose

The SDLC Orchestration layer coordinates the full software delivery lifecycle from intent capture through to observability. It manages phase sequencing, gate evaluation, and per-phase target policies for every Initiative running on the platform.

**Phase ordering (canonical, as of sdlc-end-to-end):**

| # | Phase | Key | Default Target |
|---|-------|-----|---------------|
| 1 | Product | `product` | _(always runs)_ |
| 2 | Architecture | `architecture` | `required` |
| 3 | Design | `design` | `optional` |
| 4 | Development | `development` | `required` |
| 5 | QA | `qa` | `required` |
| 6 | Security | `security` | `required` |
| 7 | DevOps | `devops` | `required` |
| 8 | Infrastructure | `iac` | `opt-in` |
| 9 | SRE | `sre` | `optional` |
| 10 | FinOps | `finops` | `opt-in` |
| 11 | Observability | `observability` | `opt-in` |

**Target policies:** `required` (fail on gate fail) · `optional` (warn and continue) · `opt-in` (skipped unless explicitly included) · `skipped` (removed from plan entirely).

**Reference workflow:** `forge.reference.intent-to-infrastructure@1` exercises the full chain from intent → architecture → design → development → qa → security → devops → iac → deploy → sre → observability.
## Requirements
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

### Requirement: Coordination by Alfred
Alfred SHALL coordinate phase-aware capabilities — invoking them directly or via workflows — and SHALL consolidate their outputs into the related OpenSpec and linked artifacts.

#### Scenario: Alfred coordinates Architecture and Security for a critical change
- **WHEN** an OpenSpec is classified as security-impacting
- **THEN** Alfred invokes the Architecture capability for diagrams/ADRs and the Security capability for threat modeling, and links results to the OpenSpec

### Requirement: Traceability from intent to deploy
The orchestration SHALL preserve traceability across phases: intent → OpenSpec → backlog (Jira) → design (Figma/Confluence) → code (GitHub) → CI/CD → deploy → observability → incidents/postmortems.

#### Scenario: Trace path is queryable end-to-end
- **WHEN** a user opens a deployment in the Portal
- **THEN** the platform exposes the full trace path back to the originating intent and OpenSpec

### Requirement: Approved assets only in production-relevant phases
Phases that affect production-relevant artifacts (Architecture, Security, DevOps/SRE) SHALL invoke only `approved` assets, except in T0 sandboxes/labs explicitly authorized.

#### Scenario: Non-approved Security asset cannot be invoked in prod-related flow
- **WHEN** a Security asset in `in_review` is invoked from a flow targeting prod-related artifacts
- **THEN** the invocation is blocked and audited

### Requirement: Jira and Confluence integration
The platform SHALL integrate with **Jira** (read/write epics, stories, tasks, sprints, statuses) and **Confluence** (read/write pages, ADRs, runbooks). Integrations SHALL respect Workspace permissions and propagate identity.

#### Scenario: PO capability creates epic and stories from OpenSpec
- **WHEN** the PO capability is invoked on an OpenSpec
- **THEN** it creates/updates the corresponding Jira epic and stories, links them in the OpenSpec, and emits audit events

### Requirement: Quality and security gates as part of orchestration
The orchestration SHALL include quality and security gates (lint, unit tests, SAST, SCA, DAST, performance/e2e where applicable, evals for agentic outputs) before progressing to deploy stages, with results visible in PRs and dashboards.

#### Scenario: Failing gate blocks progression
- **WHEN** SAST or SCA fails on a PR with high-severity findings
- **THEN** the orchestration blocks progression to staging/prod stages and notifies owners

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

