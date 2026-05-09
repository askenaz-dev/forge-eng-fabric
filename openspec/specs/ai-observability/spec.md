# ai-observability Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Langfuse (or equivalent) integration
The platform SHALL emit AI observability data — prompts, responses, tool calls, traces, evals and costs — to **Langfuse** (or equivalent), with sensitive fields redacted per data-classification policy.

#### Scenario: Prompt and response captured with redaction
- **WHEN** Alfred performs a model call via LiteLLM
- **THEN** Langfuse receives a trace with prompt/response (sensitive fields redacted), model, tokens, cost, latency and eval scores

### Requirement: Correlation with platform telemetry
AI observability traces SHALL share `correlation_id` with platform OpenTelemetry traces and audit events for end-to-end navigation.

#### Scenario: Trace from intent to model call
- **WHEN** an SRE follows a `correlation_id` from an audit event
- **THEN** the same id resolves to the Langfuse trace and the Tempo trace

### Requirement: Cost and quality dashboards
The platform SHALL provide dashboards (Grafana and/or Langfuse) showing cost, tokens, latency, eval-score trends and guardrail-trip rates by Tenant/Workspace/asset.

#### Scenario: Workspace owner views cost trend
- **WHEN** a Workspace owner opens the AI observability dashboard
- **THEN** the dashboard shows the cost trend per asset and per model with comparable baselines

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
