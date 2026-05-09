# advanced-eval-harness Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Golden datasets as assets

Eval datasets MUST be registered as Registry assets (`type=eval_dataset`) with versioning and trust level; datasets MUST be immutable per version.

#### Scenario: New dataset version required for changes

- **GIVEN** dataset `ds-7@1.0.0` published
- **WHEN** a contributor edits inputs
- **THEN** the edit MUST require `ds-7@1.1.0` (or higher)
- **AND** prior version MUST remain available for historical comparisons

### Requirement: Regression gate at publish time

Publishing a new workflow/skill version MUST run the regression dataset; key metric drop > Δ blocks publication.

#### Scenario: Block publication on quality regression

- **GIVEN** workflow `wf-1@1.2.0` with success rate 92% on dataset `ds-7@1.0.0`
- **AND** Δ threshold of 3 percentage points
- **WHEN** `wf-1@1.3.0` shows 88% success
- **THEN** the publish MUST be denied with `regression_detected`
- **AND** emit `workflow.publish.regression_blocked.v1`

### Requirement: A/B testing across versions

Workspaces MAY opt-in to A/B between two versions for N executions; the harness MUST collect outcomes and emit a comparison report.

#### Scenario: Opt-in A/B reports significance

- **GIVEN** Workspace `ws-1` opts to A/B `wf-1@1.2.0` vs `1.3.0` for 200 executions
- **WHEN** 200 executions complete
- **THEN** the harness MUST produce a report with success rate, latency, cost per version
- **AND** indicate statistical significance

### Requirement: Business metric instrumentation

Workflows MAY declare `success_metric`; the harness MUST instrument and report it in dashboards.

#### Scenario: Workflow declares success metric

- **GIVEN** `wf-release-train@1.0.0` declares `success_metric: pr_merged_within_24h`
- **WHEN** 50 executions complete
- **THEN** the dashboard MUST show the success metric rate alongside technical metrics
