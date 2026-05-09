# finops-recommendations Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Daily cost-reduction analysis

A cron job MUST analyze cloud + LLM cost data daily and produce concrete recommendations with `expected_savings` calculated.

#### Scenario: Idle resource recommendation

- **GIVEN** a Cloud Run service with sustained CPU < 5% for 14 days
- **WHEN** the analysis runs
- **THEN** a `finops_recommendation` MUST be created of type `downsize`
- **AND** include estimated monthly savings, current vs proposed config, risk
- **AND** emit `finops.recommendation.created.v1`

### Requirement: Recommendations land as PRs through standard pipelines

Recommendations whose remediation is concrete MUST open PRs (Terraform, prompt template, cache config) following the standard onboarding (Phase 2) and SDLC gates (Phase 4).

#### Scenario: Terraform downsize PR opens

- **GIVEN** a `downsize` recommendation for a GKE node pool
- **WHEN** Alfred materializes it
- **THEN** a PR MUST be opened in the IaC repo with the change
- **AND** include OpenSpec link, expected savings, performance risk assessment
- **AND** the PR MUST be subject to all CI gates and approvals

### Requirement: Performance impact thresholds

Recommendations affecting `criticality≥high` assets MUST estimate performance impact ≤ Δ% and require explicit human approval.

#### Scenario: High-criticality recommendation needs approval

- **GIVEN** a recommendation downsizing a critical service estimated at 8% latency increase
- **WHEN** the threshold for high is 5%
- **THEN** the recommendation MUST be marked `needs-approval`
- **AND** an Approvals Inbox entry MUST be created for the Workspace owner
