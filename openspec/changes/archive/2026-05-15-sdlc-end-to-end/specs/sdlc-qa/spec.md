## ADDED Requirements

### Requirement: QA skill implementations

The capability SHALL ship working implementations of `generate-test-plan`, `generate-e2e-tests`, `triage-test-failures` skills, registered in the Asset Registry with their eval suites. The skills SHALL be invokable through the standard skill-gateway and SHALL persist outputs in the App's repo.

#### Scenario: generate-test-plan produces a plan grounded in the API contract

- **GIVEN** an App `app-1` with an approved API contract and a data model
- **WHEN** Alfred invokes `generate-test-plan`
- **THEN** the skill MUST produce a test plan at `tests/plans/<spec-slug>.md` covering: API contract coverage (one case per documented endpoint × happy-path + error-path), data integrity, security regression, performance (when `criticality≥high`)
- **AND** emit `sdlc.test_plan.proposed.v1` with the plan path

#### Scenario: generate-e2e-tests produces Playwright suite from the plan

- **GIVEN** an approved test plan
- **WHEN** Alfred invokes `generate-e2e-tests`
- **THEN** the skill MUST produce a Playwright suite under `tests/e2e/<spec-slug>/` with one spec file per plan section
- **AND** the suite MUST be runnable via `npm run test:e2e -- <spec-slug>` and MUST execute in the App's CI pipeline

### Requirement: Reactive triage on CI failure

The capability SHALL provide a *reactive triage* hook that automatically invokes `triage-test-failures` whenever the App's CI emits `ci.failed.v1`. The hook SHALL produce a structured triage report (top hypotheses, affected files, proposed minimal patch) and post the report as a PR comment. When `targets.qa in {required, autonomous}` and the proposed patch passes the safety eval, the hook SHALL additionally open a fix PR for review.

#### Scenario: Triage report posted as PR comment

- **GIVEN** an App's CI run that fails on PR `pr-42`
- **WHEN** `ci.failed.v1` is emitted
- **THEN** within 90 seconds the platform MUST invoke `triage-test-failures` against the failing run
- **AND** post a PR comment on `pr-42` with the structured report (top hypotheses, affected files, proposed patch in a collapsible block)
- **AND** emit `sdlc.test_failure.triaged.v1` with the PR URL, the run id and the top hypothesis

#### Scenario: Auto-fix PR opened when targets.qa is required

- **GIVEN** an App with `targets.qa=required` and a proposed patch that passes the safety eval
- **WHEN** triage completes
- **THEN** the platform MUST open a follow-up PR titled `qa-fix: <slug>` against the App's repo with the proposed patch
- **AND** the new PR MUST link back to the originating failing PR/comment

#### Scenario: Rate-limited to one report per PR per 10 minutes

- **GIVEN** a PR with two CI failures within 5 minutes
- **WHEN** the second failure is processed
- **THEN** the existing PR comment MUST be updated in place with the new triage report
- **AND** no second comment MUST be posted

#### Scenario: Auto-fix suppressed when target is optional

- **GIVEN** an App with `targets.qa=optional`
- **WHEN** triage completes with a proposed patch
- **THEN** the comment MUST be posted but no fix PR MUST be opened automatically

## MODIFIED Requirements

### Requirement: QA skills

The capability SHALL expose `generate-test-plan`, `generate-e2e-tests`, `triage-test-failures` as registered skills with concrete implementations. The skills SHALL be invokable proactively (during the design/development phases) and reactively (in response to `ci.failed.v1`).

#### Scenario: E2E tests generated from API contract

- **GIVEN** an initiative with an API contract approved
- **WHEN** Alfred invokes `generate-e2e-tests`
- **THEN** an E2E test suite (Playwright or equivalent) MUST be produced and committed
- **AND** the suite MUST execute in the pipeline
- **AND** failures MUST emit `test.run.failed.v1`

#### Scenario: triage-test-failures invoked reactively

- **WHEN** `ci.failed.v1` is emitted for an App's CI run
- **THEN** the QA capability MUST invoke `triage-test-failures` against the run within 90 seconds
- **AND** emit `sdlc.test_failure.triaged.v1` with the structured triage report
