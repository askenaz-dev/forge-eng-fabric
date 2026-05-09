# diagnosis-pipeline Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Multi-source evidence gathering

The pipeline MUST gather context from: relevant OpenSpecs, runbooks (Confluence + repo), Prometheus/Loki/Tempo telemetry, recent deployments (Fase 3), eval results, FinOps signals, and similar incidents from the KB.

#### Scenario: All sources queried within timeout

- **GIVEN** an incident on asset `app-foo` in `env=prod`
- **WHEN** the diagnosis runs
- **THEN** the report MUST list evidence sections for each source
- **AND** missing sources MUST be reported as `unavailable` rather than silently omitted
- **AND** total p95 latency MUST be ≤ 60s

### Requirement: Hypothesis generation with citation enforcement

Every hypothesis produced MUST cite at least one verifiable source URL/id; uncited hypotheses MUST be discarded.

#### Scenario: Discard uncited hypothesis

- **GIVEN** an LLM output containing 3 hypotheses, one without citations
- **WHEN** citation enforcement runs
- **THEN** the uncited hypothesis MUST be removed
- **AND** the report MUST log `discarded_hypotheses=1, reason=missing_citation`

### Requirement: Ranked output with suggested actions

The output MUST be a `diagnosis_report` with top-N hypotheses ordered by confidence and KB match, each linked to candidate healing actions.

#### Scenario: Suggested actions linked

- **GIVEN** top hypothesis "pod CrashLoopBackOff due to OOM"
- **WHEN** the report is built
- **THEN** suggested actions MUST include `restart-pod` and `scale-up` from the catalog
- **AND** each suggestion MUST include risk and reversibility flags
