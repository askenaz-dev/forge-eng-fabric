## ADDED Requirements

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
