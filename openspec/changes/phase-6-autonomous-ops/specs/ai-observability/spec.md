# Spec Delta: ai-observability (MODIFIED)

## MODIFIED Requirements

### Requirement: Healing metrics

The observability stack SHALL expose metrics: `healing_invocations_total{level}`, `healing_success_rate{level}`, `mttr_seconds{capability,env}`, `incident_count_total{severity}`, `auto_rollback_rate`, `kill_switch_activation_count`.

#### Scenario: MTTR measurable per capability

- **GIVEN** 20 incidents resolved in `capability=sdlc-devops, env=prod`
- **WHEN** the dashboard queries MTTR
- **THEN** the metric MUST report median, p95 and trend

### Requirement: Detection→action funnel

The dashboard MUST show the funnel `detected → diagnosed → acted (per level) → resolved` with dropout rates per stage.

#### Scenario: Funnel reveals diagnosis bottleneck

- **GIVEN** 100 detected, 95 diagnosed, 60 acted, 50 resolved
- **WHEN** the dashboard renders
- **THEN** the funnel MUST surface stage-by-stage conversion and highlight `diagnosed→acted` as the largest dropout
