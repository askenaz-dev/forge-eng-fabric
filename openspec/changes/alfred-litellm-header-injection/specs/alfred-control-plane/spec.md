## ADDED Requirements

### Requirement: Standard tenant headers on every LLM call

Every outbound request from Alfred to the LiteLLM gateway (chat completions, embeddings, any future endpoint) SHALL include the four standard Forge headers, populated from the active request context:

- `forgetenantid` — the tenant id of the workspace owning the active session
- `forgeworkspaceid` — the workspace id of the active session
- `forgecorrelationid` — the correlation id used to thread the request through downstream services and audit
- `data_classification` — one of `public`, `internal`, `confidential`, `restricted`; defaults to `internal` when the caller does not specify

Requests missing any of the four headers SHALL NOT be sent. The client SHALL fail closed with a clear error rather than send a request without attribution.

#### Scenario: Reasoning loop call carries all four headers

- **WHEN** Alfred's reasoning loop invokes `LiteLLMClient.chat()` with an active session context
- **THEN** the outbound HTTP request to LiteLLM includes `forgetenantid`, `forgeworkspaceid`, `forgecorrelationid`, and `data_classification` headers, with values matching the session's tenant, workspace, correlation id, and the call's classification (defaulted to `internal` if not provided)

#### Scenario: Embedding call carries all four headers

- **WHEN** Alfred invokes `LiteLLMClient.embeddings()` with an active session context
- **THEN** the outbound HTTP request to LiteLLM includes all four standard headers populated from the session context

#### Scenario: Missing context fails closed

- **WHEN** a code path invokes `LiteLLMClient.chat()` or `.embeddings()` without supplying any of the required header values (no tenant id, no workspace id, or no correlation id)
- **THEN** the client raises a clear error before issuing any network request
- **AND** no LiteLLM call is made

#### Scenario: Classification defaults to internal

- **WHEN** a caller invokes `LiteLLMClient.chat()` without specifying a `data_classification`
- **THEN** the outbound request carries `data_classification: internal`

### Requirement: Prompt tool invocations route to prompt-template-service

Alfred's tool router SHALL dispatch any tool invocation whose tool id matches the shape `prompt:<template_id>:render` to the `prompt-template-service` `POST /v1/render` endpoint. The legacy `prompt-registry` service SHALL NOT receive `prompt:*` invocations from Alfred.

The `prompt-template-service` base URL SHALL be configured via the `PROMPT_TEMPLATE_SERVICE_URL` environment variable, exposed in Alfred's configuration as `prompt_template_service_url`. If the variable is not set, Alfred SHALL refuse to start rather than fall back to a broken default.

#### Scenario: `prompt:<id>:render` reaches prompt-template-service

- **WHEN** Alfred's tool router receives an invocation with tool id `prompt:greeting:render` and params `{name: "Ada"}`
- **THEN** the router issues `POST <PROMPT_TEMPLATE_SERVICE_URL>/v1/render` with body `{ref: "greeting", variables: {name: "Ada"}}`
- **AND** returns the rendered response to the caller

#### Scenario: Legacy `:invoke` shape is rejected

- **WHEN** Alfred's tool router receives an invocation with tool id `prompt:foo:invoke` (or any non-`:render` shape)
- **THEN** the router raises a clear error naming the supported `prompt:<template_id>:render` shape
- **AND** does not contact either prompt service

#### Scenario: Missing config fails startup

- **WHEN** Alfred is started without `PROMPT_TEMPLATE_SERVICE_URL` set in the environment
- **THEN** Alfred refuses to start
- **AND** logs a clear error naming the missing environment variable

#### Scenario: prompt-template-service unreachable surfaces as tool failure

- **WHEN** Alfred's tool router invokes `prompt:greeting:render` and `prompt-template-service` responds with a network error or 5xx status
- **THEN** the tool invocation fails with a `tool_unavailable` reason
- **AND** the reasoning loop receives the failure to decide whether to retry or escalate
