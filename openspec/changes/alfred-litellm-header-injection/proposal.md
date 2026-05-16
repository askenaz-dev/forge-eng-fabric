## Why

Two production bugs in Alfred surfaced by the May 2026 gap audit are breaking the platform's cost attribution and silently failing every `prompt:*` tool invocation. Both are fixable in 1-2 days without waiting for the larger umbrella reshape (`intent-to-infrastructure-gap-closure`), and shipping them now restores tenant-level cost accuracy that the upcoming billing front (F7) will depend on.

**G1 — Missing tenant headers.** `services/alfred/alfred/llm.py:48` calls LiteLLM with plain auth, without the four standard headers (`forgetenantid`, `forgeworkspaceid`, `forgecorrelationid`, `data_classification`) that the stub at `services/alfred/alfred_stub/llm.py:25-30` injects correctly. Cost telemetry, audit, and classification-based routing cannot work today.

**G2 — Dead route to a nonexistent endpoint.** `services/alfred/alfred/tools.py` `ToolRouter` routes `prompt:*` tool calls to `prompt-registry`'s `/v1/invoke` endpoint, which does not exist in `services/prompt-registry/prompt_registry/app.py`. Every `prompt:*` invocation is dead code or silently failing. The architecturally consistent fix is to migrate Alfred to the new `services/prompt-template-service/` `POST /v1/render` (shipped by archived `ai-flow-authoring` 2026-05-16, already used by workflow-runtime's LLM executor) rather than patching the legacy registry.

## What Changes

- Update `services/alfred/alfred/llm.py` `LiteLLMClient` to inject `forgetenantid`, `forgeworkspaceid`, `forgecorrelationid`, and `data_classification` headers on every `chat()` and `embeddings()` request. Values sourced from request context (session metadata in `services/alfred/alfred/store.py`).
- Add `data_classification` parameter to `chat()` and `embeddings()` with default `internal`. Callers can override per call when classification differs.
- Migrate `services/alfred/alfred/tools.py` `ToolRouter` `prompt:*` path to call `prompt-template-service`'s `POST /v1/render` instead of the nonexistent `prompt-registry/v1/invoke`. Map `prompt:<template_id>:render` tool IDs to the render contract.
- Add `prompt_template_service_url` to `services/alfred/alfred/config.py` (env var `PROMPT_TEMPLATE_SERVICE_URL`, consistent with the convention from `ai-flow-authoring` task 13.2).
- Add unit tests for header injection on every call path. Add unit tests for `prompt:*` routing to the new service. Add an integration test that exercises a `prompt:*` tool end-to-end against a running `prompt-template-service`.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `alfred-control-plane`: ADD two explicit requirements that today are implicit. (1) Every LLM call MUST carry the four standard headers (`forgetenantid`, `forgeworkspaceid`, `forgecorrelationid`, `data_classification`) sourced from request context. (2) Tool invocations of shape `prompt:<template_id>:render` MUST be dispatched to `prompt-template-service` via `POST /v1/render`. The current spec mentions LiteLLM exclusivity and tool invocation in general but does not pin down either contract — this change makes both testable.

## Impact

- **Code**:
  - `services/alfred/alfred/llm.py` — header injection on `LiteLLMClient.chat()` and `.embeddings()`.
  - `services/alfred/alfred/tools.py` — `ToolRouter` `prompt:*` path migrated.
  - `services/alfred/alfred/config.py` — add `prompt_template_service_url` field + env var binding.
  - `services/alfred/tests/test_llm.py` (or equivalent) — header injection tests.
  - `services/alfred/tests/test_tools.py` (or equivalent) — `prompt:*` migration tests.
  - Add an integration test exercising a `prompt:*` tool against a running `prompt-template-service`.
- **APIs**: No public API changes. Internal contracts: Alfred now calls `prompt-template-service/v1/render` instead of `prompt-registry/v1/invoke`.
- **Dependencies**: No new external dependencies. `prompt-template-service` was added to deployment by `ai-flow-authoring`.
- **Migration**: None. The old route was already non-functional; no consumer depends on the legacy behavior.
- **Tests**: New unit + integration tests as above. Existing Alfred reasoning loop tests must remain green (header injection is additive).
- **Governance**: Change log entry documents both fixes and references the gap audit + umbrella `intent-to-infrastructure-gap-closure`.

## Out of scope

- Implementing `/v1/invoke` in `prompt-registry`. Don't patch the legacy service; route around it.
- Deprecating or removing `prompt-registry`. That is a separate follow-up tracked in the umbrella's Out of scope as a V2 candidate.
- Migrating workflow-runtime or any other consumer to the new prompt service. Only Alfred's `ToolRouter` is in scope here.
- Any `model-gateway` work. This change is LiteLLM header injection + prompt-template-service migration only. The full model-gateway SDK + routing + cost telemetry come in umbrella fronts F0b and F0c.
