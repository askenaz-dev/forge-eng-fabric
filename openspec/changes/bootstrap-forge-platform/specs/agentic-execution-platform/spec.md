## ADDED Requirements

### Requirement: Isolated runners
The platform SHALL execute agents, tools and workflows in isolated runners (containers/sandboxes) with per-execution identity, network policies and resource quotas. Runners SHALL NOT share secrets or memory across executions or tenants.

#### Scenario: Two concurrent executions cannot read each other's secrets
- **WHEN** two runner executions for different Workspaces run concurrently
- **THEN** neither execution can read or enumerate secrets, env vars or filesystem of the other

### Requirement: Secret brokering and identity propagation
The platform SHALL broker secrets at execution time from a Secret Manager / Vault and propagate the calling principal's identity (user/app) to integrated systems. Secrets SHALL never be embedded in prompts, logs or persisted alongside execution payloads.

#### Scenario: Secret never appears in logs or prompts
- **WHEN** a runner uses a brokered secret to call an external API
- **THEN** the secret is redacted from logs, traces, prompts, and tool input/output records

### Requirement: Policy checks before tool execution
Before executing any tool, MCP, Skill or Workflow node, the platform SHALL evaluate the applicable policies (Workspace, OpenSpec, asset trust level, environment, data classification). Failure SHALL block the action and emit an audit event.

#### Scenario: Policy denies action and audits attempt
- **WHEN** Alfred attempts to invoke a T4 deploy asset on prod without the required approval
- **THEN** the runner blocks execution, emits an audit event, and returns a structured policy-denial error

### Requirement: Rate limits, cost limits and budgets
The platform SHALL enforce rate limits and cost limits per Tenant, Workspace, asset and environment. Exceeding budgets SHALL block further executions and notify owners.

#### Scenario: Workspace exceeds LLM budget
- **WHEN** a Workspace's monthly LiteLLM budget is exhausted
- **THEN** further model calls from that Workspace are blocked, owners are notified, and audit records the event

### Requirement: Retry, checkpointing and durability
The platform SHALL support automatic retries with backoff for transient failures and SHALL provide checkpointing/durability for long-running executions. Long-running workflows SHALL be able to use **Temporal** as the execution engine.

#### Scenario: Long-running workflow resumes after runner crash
- **WHEN** a Temporal-backed workflow is interrupted by a runner crash
- **THEN** the workflow resumes from the last checkpoint without duplicating completed side-effects

### Requirement: Eval harness and guardrails
The platform SHALL provide an eval harness producing scores for quality, safety, cost and latency, and SHALL apply guardrails (input/output filtering, prompt-injection detection, allowlists, schema validation) on agent and LLM interactions.

#### Scenario: Guardrail blocks suspicious tool call from injected content
- **WHEN** retrieved RAG content contains instructions attempting to invoke a sensitive tool not on the allowlist
- **THEN** the guardrail blocks the tool call and audits the prompt-injection attempt

### Requirement: Telemetry for executions
Every runner execution SHALL emit OpenTelemetry traces and metrics including `correlation_id`, `tenant_id`, `workspace_id`, `asset_id`, `version`, `policy_decisions`, `latency`, `tokens`, `cost`, `eval_scores` and `outcome`. AI-specific telemetry SHALL be sent to **Langfuse** (or equivalent).

#### Scenario: Execution emits traces and AI telemetry
- **WHEN** an asset is invoked from a workflow
- **THEN** OpenTelemetry traces are emitted with required attributes and Langfuse receives prompt/response/eval telemetry
