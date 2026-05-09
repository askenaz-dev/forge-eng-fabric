# sdlc-qa Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: QA skills

The capability SHALL expose `generate-test-plan`, `generate-e2e-tests`, `triage-test-failures` as registered skills.

#### Scenario: E2E tests generated from API contract

- **GIVEN** an initiative with an API contract approved
- **WHEN** Alfred invokes `generate-e2e-tests`
- **THEN** an E2E test suite (Playwright or equivalent) MUST be produced and committed
- **AND** the suite MUST execute in the pipeline
- **AND** failures MUST emit `test.run.failed.v1`

### Requirement: QA gates

Gates `integration_tests_passing`, `e2e_tests_passing`, `perf_budget_met` (for `criticality≥high`) MUST be evaluated before progression to `security`.

#### Scenario: Performance budget blocks high-criticality progression

- **GIVEN** an initiative with `criticality=high` and p95 latency exceeding budget
- **WHEN** progression is requested
- **THEN** gate `perf_budget_met` MUST fail with measurement details
- **AND** Alfred MUST propose remediation actions
