## ADDED Requirements

### Requirement: LiteLLM is the single entry point for model access
All language-model access in Forge SHALL pass through **LiteLLM**. Direct calls to provider SDKs/endpoints from any other service SHALL be blocked at network level by NetworkPolicy/egress rules in addition to application-level policy.

#### Scenario: Direct provider call from another service is blocked
- **WHEN** any service other than the LiteLLM gateway attempts to reach a provider endpoint
- **THEN** the egress is denied by NetworkPolicy and the attempt is logged as a security event

### Requirement: At least one approved provider configured
LiteLLM SHALL be configured with at least one approved provider (e.g., **Vertex AI**) for the bootstrap. The list of approved providers SHALL be maintained in versioned configuration.

#### Scenario: Bootstrap smoke test calls the approved provider
- **WHEN** the bootstrap smoke test issues a chat completion via LiteLLM
- **THEN** the request reaches the approved provider, returns a valid response, and emits cost/latency telemetry

### Requirement: Internal SDK/client for LiteLLM
The platform SHALL publish an internal SDK/client (Go and Python) that wraps LiteLLM access with standard headers (`forgetenantid`, `forgeworkspaceid`, `forgecorrelationid`, `data_classification`) and is the **only sanctioned way** for Forge services to call LLMs.

#### Scenario: SDK injects required headers
- **WHEN** a service uses the internal SDK to call LiteLLM
- **THEN** the request carries the standard headers and the gateway records them in telemetry

### Requirement: Cost and latency telemetry from day one
LiteLLM SHALL emit cost and latency telemetry (tokens, cost, model, fallback events) tagged by `forgetenantid` and `forgeworkspaceid` to the platform observability stack.

#### Scenario: Cost is attributed to a Workspace
- **WHEN** a model call is performed via LiteLLM for a given Workspace
- **THEN** the cost is recorded against that Workspace and visible in dashboards
