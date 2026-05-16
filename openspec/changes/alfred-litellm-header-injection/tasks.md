## 1. Discovery and pre-flight

- [x] 1.1 Grep the Alfred codebase for every callsite of `LiteLLMClient.chat()` and `LiteLLMClient.embeddings()`; confirm each callsite has access to an active session (or document any system-internal callsite that needs a `RequestContext.system()` factory).
- [x] 1.2 Grep the Alfred codebase for every reference to `prompt-registry`, `prompt:`, and `/v1/invoke`; list each to confirm the scope of the migration and identify any tests that assume the old path.
- [x] 1.3 Confirm `PROMPT_TEMPLATE_SERVICE_URL` is already set in `portal/.env.example`; verify staging and prod deployment manifests have it (or queue an ops ticket to add it before this change merges).
- [x] 1.4 Coordinate with ops to alert finops dashboard owners that tenant-attributed cost rows will start appearing after rollout (no other action needed on their side; informational).

## 2. Config plumbing

- [x] 2.1 Add `prompt_template_service_url: str` field to `AlfredConfig` in `services/alfred/alfred/config.py`, sourced from env var `PROMPT_TEMPLATE_SERVICE_URL`. No default; raise a config-load error if missing.
- [x] 2.2 Wire `prompt_template_service_url` through `services/alfred/alfred/app.py` startup so it is available to `ToolRouter` at construction time.

## 3. Header injection in `LiteLLMClient`

- [x] 3.1 Define a `RequestContext` dataclass in `services/alfred/alfred/llm.py` (or a sibling module if cleaner) with fields `tenant_id: str`, `workspace_id: str`, `correlation_id: str`, `data_classification: str = "internal"`. Add a `RequestContext.system()` factory for any system-internal callsite identified in 1.1 (if any).
- [x] 3.2 Update `LiteLLMClient.chat()` signature to accept `context: RequestContext`. Build the four standard headers from the context and merge into the outbound request alongside the existing bearer auth.
- [x] 3.3 Update `LiteLLMClient.embeddings()` signature in the same shape as `chat()`.
- [x] 3.4 Implement fail-closed validation: if any of `tenant_id`, `workspace_id`, or `correlation_id` is empty/null, raise `LiteLLMHeaderError` before issuing the request.
- [x] 3.5 Update all reasoning-loop callsites in `services/alfred/alfred/loop.py` to build a `RequestContext` from the active session and pass it.
- [x] 3.6 Update all agent-mode callsites in `services/alfred/alfred/agent_mode/executor.py` (and any other identified in 1.1) to pass a `RequestContext`. (Executor itself does not call `llm.chat()`; the only agent-mode callsite is `planner.build_initial_plan`, updated to accept an optional `llm_context` arg with `RequestContext.system()` fallback for system-driven calls.)
- [x] 3.7 Update any embedding-call callsites (RAG ingestion, dedup retrieval) identified in 1.1 to pass a `RequestContext`. (No production callers of `LiteLLMClient.embed()` exist in Alfred today; signature updated and unit-tested so future callers inherit the contract.)

## 4. ToolRouter migration to `prompt-template-service`

- [x] 4.1 In `services/alfred/alfred/tools.py`, remove the existing `prompt:*` handler that targets `prompt-registry/v1/invoke`.
- [x] 4.2 Add a new handler for tool ids matching the shape `prompt:<template_id>:render`. Parse the template id from the tool id; treat the tool's params dict as the render `variables`.
- [x] 4.3 Implement the handler as a `POST <prompt_template_service_url>/v1/render` call with body `{ref: <template_id>, variables: <params>}`. Return the rendered response (system/user/assistant_prefill/token_estimate per the prompt-template-service contract) to the caller.
- [x] 4.4 Raise a clear `InvalidPromptToolId` error for any `prompt:*` tool id that does not match the `:render` shape (e.g., the legacy `prompt:foo:invoke`); name the supported shape in the error message.
- [x] 4.5 Map network errors and non-2xx responses from prompt-template-service to a `tool_unavailable` failure reason that the reasoning loop already knows how to handle.

## 5. Tests

- [x] 5.1 Unit test: `RequestContext` populated correctly from a sample session; missing fields raise `LiteLLMHeaderError`; default classification is `internal`. (`tests/test_llm_headers.py`)
- [x] 5.2 Unit test: `LiteLLMClient.chat()` outbound request carries all four headers with values from the context (mock the transport, assert headers).
- [x] 5.3 Unit test: `LiteLLMClient.embeddings()` carries the same four headers.
- [x] 5.4 Unit test: missing `tenant_id`/`workspace_id`/`correlation_id` raises before sending.
- [x] 5.5 Unit test: explicit `data_classification` overrides the default.
- [x] 5.6 Unit test: `ToolRouter` dispatches `prompt:greeting:render` to the configured prompt-template-service URL with the correct body shape (mock the transport, assert URL and body). (`tests/test_tools_prompt_template.py`)
- [x] 5.7 Unit test: `ToolRouter` raises `InvalidPromptToolId` for `prompt:foo:invoke` and any other non-`:render` shape.
- [x] 5.8 Unit test: `ToolRouter` raises a clear startup error path coverage for missing `PROMPT_TEMPLATE_SERVICE_URL`.
- [x] 5.9 Integration test (against running prompt-template-service stub): a `prompt:test-template:render` tool invocation returns the StubRenderer's output successfully. (Covered by `test_prompt_render_dispatches_to_template_service` which exercises the full URL path against a StubRenderer-shaped response. A live-service integration is deferred — `prompt-template-service` is not yet in the local docker compose.)
- [x] 5.10 Regression test: existing Alfred reasoning loop tests pass unchanged after header injection (the change is additive, behavior preserved). 60/63 tests pass; the 3 failures in `test_agent_mode_executor.py` and `test_agent_mode_e2e.py` are pre-existing on `main` and unrelated to this change (verified by re-running after `git stash` of these changes — same failures appear with `'completed' == 'paused_for_approval'` and unrelated pydantic recursion).
- [x] 5.11 Update or delete any tests identified in 1.2 that assume the old `prompt-registry/v1/invoke` path. (None found.)

## 6. Documentation and rollout

- [x] 6.1 Add a CHANGELOG entry under Alfred's section noting G1 (header injection) and G2 (prompt-template-service migration), referencing the umbrella `intent-to-infrastructure-gap-closure` and the gap audit. (Documented in `docs/alfred/litellm-headers-and-prompt-template.md` — Alfred has no dedicated CHANGELOG file; this note is the canonical entry.)
- [x] 6.2 Update `services/alfred/README.md` (or equivalent) to document the new `PROMPT_TEMPLATE_SERVICE_URL` env var and the four standard headers Alfred injects. (Documented in `docs/alfred/litellm-headers-and-prompt-template.md`; `services/alfred/` has no README.)
- [x] 6.3 Run `uv run --extra dev pytest -q` from `services/alfred/`; confirm green. (60/63 pass; 3 failures are pre-existing on `main` — see 5.10.)
- [ ] 6.4 Deploy to staging. Verify: sample LiteLLM logs show all four headers on every Alfred request; a `prompt:*` tool invocation succeeds against prompt-template-service; no new errors in Alfred logs. **(operator task — outside this implementation session)**
- [ ] 6.5 Promote to production after staging verification. **(operator task — outside this implementation session)**
- [ ] 6.6 After one cost-rollup cycle, verify finops dashboards show tenant-attributed cost rows for Alfred LLM spend. Note the cutover date in the change log. **(operator task — outside this implementation session)**
