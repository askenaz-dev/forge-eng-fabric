# per-asset-observability Specification

## Purpose
TBD - created by syncing completed phase changes. Update Purpose after archive.
## Requirements

### Requirement: Observability tab per asset

Every asset of type `skill`, `prompt`, `workflow`, `mcp_tool` MUST expose an Observability tab with invocations, success rate, latency p50/p95/p99, cost per execution, and eval-drift indicator.

#### Scenario: Owner inspects skill metrics

- **GIVEN** skill `sdlc-product/refine-user-story@1.2.0` registered
- **WHEN** the owner opens its Observability tab
- **THEN** the tab MUST show metrics over selectable time ranges (1h/24h/7d/30d)
- **AND** drill-down to top failing invocations MUST be available

### Requirement: Cost per execution visible

For every invocation, the platform MUST attribute LLM cost (from LiteLLM/Langfuse) and compute cost (from runtime telemetry); aggregated cost per execution is shown.

#### Scenario: Cost reported with breakdown

- **GIVEN** workflow `wf-1` executed 100 times
- **WHEN** the dashboard renders
- **THEN** average cost MUST be shown with breakdown LLM vs compute
- **AND** outliers (top 1% cost) MUST be highlighted

### Requirement: Eval drift detection

The dashboard MUST plot eval score over time and alert when a regression > Δ persists for N runs.

#### Scenario: Drift alert raised

- **GIVEN** an eval score baseline of 0.92 on `wf-1`
- **WHEN** 3 consecutive runs report ≤ 0.85
- **THEN** an alert MUST be emitted (`asset.eval.drift.detected.v1`)
- **AND** the asset detail MUST show a drift banner
