## ADDED Requirements

### Requirement: LiteLLM is the single entry point for model access
All language-model access in Forge — by Alfred, agents, MCPs, Skills, Workflows, Prompt Templates and any platform component — SHALL pass through **LiteLLM**. Direct calls to provider SDKs that bypass LiteLLM SHALL be blocked by network and platform policy.

#### Scenario: Direct provider call bypassing LiteLLM is blocked
- **WHEN** any platform component attempts to call a provider endpoint directly without LiteLLM
- **THEN** the call is denied at policy/network level and an audit event is emitted

### Requirement: Switching, fallback and model routing
LiteLLM SHALL provide model switching, automatic fallback on provider failure, and routing by `cost_class` and `data_classification`.

#### Scenario: Fallback to secondary provider on primary outage
- **WHEN** the primary provider returns repeated errors above a configured threshold
- **THEN** LiteLLM transparently routes the next request to the configured fallback provider and audits the routing

### Requirement: Cost tracking, rate limits and budgets
LiteLLM SHALL track cost per request and aggregate by Tenant, Workspace, asset, OpenSpec and feature. Rate limits and budgets SHALL be configurable at Tenant and Workspace levels, with notifications and hard stops on overage.

#### Scenario: Workspace hits monthly budget
- **WHEN** a Workspace's accumulated LLM cost reaches the configured monthly budget
- **THEN** further model calls are denied, owners are notified, and a budget-exceeded audit event is emitted

### Requirement: Data-classification-aware policies
LiteLLM SHALL apply per-request policies based on `data_classification` of the payload (public, internal, confidential, restricted) to allow only approved providers for that classification.

#### Scenario: Restricted data is denied to external provider
- **WHEN** a request flagged as `restricted` targets an external provider not approved for that classification
- **THEN** LiteLLM denies the call and emits an audit event with policy reference

### Requirement: Approved providers list
The platform SHALL maintain an explicit list of approved providers (e.g., **Vertex AI**, **Microsoft Foundry**, approved externals) governed by the SDLC Team. Adding/removing a provider SHALL be an audited operation.

#### Scenario: Unapproved provider cannot be used
- **WHEN** a configuration attempts to route to a provider not in the approved list
- **THEN** the operation is rejected and audited

### Requirement: Observability for model usage
LiteLLM telemetry SHALL be emitted to OpenTelemetry and **Langfuse** (or equivalent) including prompt, response, model, latency, tokens, cost, fallback events and policy decisions, with sensitive content redacted as configured.

#### Scenario: Model call telemetry includes cost and policy decisions
- **WHEN** LiteLLM completes a model call
- **THEN** telemetry is emitted with `tenant_id`, `workspace_id`, `model`, `tokens`, `cost`, `latency`, `fallback_used` and `policy_decisions`
