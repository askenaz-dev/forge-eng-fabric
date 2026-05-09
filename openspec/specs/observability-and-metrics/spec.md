# observability-and-metrics Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: Five observability dimensions
The platform SHALL provide observability across five dimensions: **operational** (logs/metrics/traces/health), **agentic** (tool calls, token usage, latency, model routing, eval scores, guardrail trips, tool errors), **product/adoption** (DAU/WAU, active teams/Workspaces, reused assets, executed workflows), **cost** (per model/workflow/Workspace/feature/deploy) and **security** (findings, policy violations, secrets-exposure attempts, prompt-injection attempts, SAST/SCA/DAST results).

#### Scenario: Workspace dashboard displays all five dimensions
- **WHEN** a Workspace owner opens the observability dashboard
- **THEN** the dashboard surfaces operational, agentic, product/adoption, cost and security indicators scoped to that Workspace

### Requirement: AI observability with Langfuse (or equivalent)
The platform SHALL emit AI observability data (prompts, responses, traces, evals, costs) to **Langfuse** or an equivalent system, with data classification respected and sensitive fields redacted.

#### Scenario: Prompt/response telemetry sent to Langfuse with redaction
- **WHEN** a model call is performed via LiteLLM
- **THEN** Langfuse receives the trace with prompt/response (sensitive fields redacted), model, tokens, cost and eval results

### Requirement: Priority KPIs measurable from day one
The platform SHALL compute and surface the three priority KPIs defined for Forge: **adoption by teams**, **time intent → PR/deploy**, and **asset reuse**. KPIs SHALL be queryable by Tenant, BU and Workspace.

#### Scenario: KPI report shows time intent → PR
- **WHEN** an owner queries the KPI report for a Workspace
- **THEN** the report shows median and p90 of time from intent recorded in OpenSpec to first PR opened

### Requirement: Per-asset dashboards
Each registered asset SHALL have a dashboard showing usage, eval scores trend, cost class actuals, latency, error rate and reuse across Workspaces.

#### Scenario: Asset owner sees reuse and eval trend
- **WHEN** an asset owner opens the asset dashboard
- **THEN** the dashboard shows usage by Workspace, eval scores history and current versus target SLA

### Requirement: Correlation across logs, metrics and traces
All observability signals SHALL share a `correlation_id` that ties an intent → OpenSpec → action → tool call → deployment → incident, enabling end-to-end navigation.

#### Scenario: Navigate from incident to originating intent
- **WHEN** an SRE opens an incident
- **THEN** following `correlation_id` they can traverse to the deployment, PR, OpenSpec and originating intent

### Requirement: Alerting and SLOs
The platform SHALL support SLOs and alerting on operational, agentic and security indicators with notifications routed to Workspace owners and the SDLC Team for platform-wide signals.

#### Scenario: Guardrail-trip rate exceeds threshold
- **WHEN** the guardrail-trip rate for a Workspace exceeds the configured SLO
- **THEN** the platform alerts Workspace owners and the SDLC Team

