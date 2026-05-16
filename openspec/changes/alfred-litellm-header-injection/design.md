## Context

Alfred makes two kinds of outbound calls today that are broken in different ways:

1. **LLM calls via `LiteLLMClient`** (`services/alfred/alfred/llm.py`) hit the LiteLLM HTTP endpoint with only a bearer token. The four Forge standard headers (`forgetenantid`, `forgeworkspaceid`, `forgecorrelationid`, `data_classification`) are absent. The stub at `services/alfred/alfred_stub/llm.py:25-30` shows the intended contract: every header on every request. The production deviation means LiteLLM's per-request cost rows cannot be partitioned by tenant/workspace, so finops dashboards, budget enforcement, and the upcoming F7 invoice rollup all lose signal.

2. **Prompt tool calls via `ToolRouter`** (`services/alfred/alfred/tools.py`) for any tool ID matching `prompt:*` are routed to `prompt-registry`'s `POST /v1/invoke`. That endpoint is not implemented in `services/prompt-registry/prompt_registry/app.py`. Calls either 404 silently (depending on the HTTP client's error handling) or fall through unobserved. Meanwhile the archived `ai-flow-authoring` change (2026-05-16) shipped a new Go service `services/prompt-template-service/` with a working `POST /v1/render` endpoint, already consumed by workflow-runtime's LLM executor via `HTTPPromptRenderer`. The two prompt services coexist in the tree; Alfred is pointing at the wrong one.

Alfred is the only production consumer of either path today, which makes this a low-risk surgical fix. The umbrella `intent-to-infrastructure-gap-closure` documents both bugs as G1/G2 and assigns them to this Quick-fix front.

## Goals / Non-Goals

**Goals:**
- Every Alfred LLM request carries the four standard headers with values sourced from request context.
- Every `prompt:*` tool invocation reaches `prompt-template-service/v1/render` and returns a real result.
- Tests catch any regression of either behavior.
- Ship in 1-2 days with no coordination with other in-flight changes.

**Non-Goals:**
- Implementing `/v1/invoke` in the legacy `prompt-registry`. Route around it.
- Deprecating or removing `prompt-registry`. Separate follow-up.
- Migrating any other service (workflow-runtime, ai-flow-authoring code, etc.).
- Building the production model-gateway SDK, routing, or cost engine. Those are F0b/F0c.
- Changing Alfred's reasoning loop, dialogue manager, or agent-mode logic.

## Decisions

### D1 — Inject headers in `LiteLLMClient`, not in a wrapper

**Decision.** Add header injection directly to `LiteLLMClient.chat()` and `LiteLLMClient.embeddings()` in `services/alfred/alfred/llm.py`. Build the header dict from a `RequestContext` dataclass passed in (or read from `contextvars` if cleaner with Alfred's async model — investigate during implementation).

**Why.** Alfred's reasoning loop in `services/alfred/alfred/loop.py:153` is the single callsite of `LiteLLMClient.chat()`. Wrapping the client externally would add a layer for one consumer. The stub already demonstrates the pattern inline. Keep parity with the stub.

**Alternatives considered:**
- Subclassing `LiteLLMClient` with a `TenantAwareLiteLLMClient`. Rejected: more code surface for the same effect.
- Middleware via `httpx` event hooks. Rejected: hides the contract from callsite readers.

### D2 — Source header values from session metadata, not function args

**Decision.** `LiteLLMClient.chat()` and `.embeddings()` accept a `context: RequestContext` parameter. `RequestContext` has fields `tenant_id`, `workspace_id`, `correlation_id`, `data_classification` (default `"internal"`). Callers in `loop.py` and `agent_mode/executor.py` already have access to the active session and can build the context from `alfred_session` row fields.

**Why.** Explicit beats implicit. Passing context as a function argument makes the dependency visible at the callsite and tests trivially. `contextvars` is tempting for sync code but adds correctness traps in async paths and async generators.

**Alternatives considered:**
- `contextvars.ContextVar` populated by middleware. Rejected for testability + async edge cases.
- Reading from `request.state` via FastAPI dependency. Rejected: `LiteLLMClient` should not depend on FastAPI internals.

### D3 — Migrate `ToolRouter` to `prompt-template-service`, do not patch `prompt-registry`

**Decision.** Replace the `prompt:` route handler in `services/alfred/alfred/tools.py` with a call to `prompt-template-service`'s `POST /v1/render`. Map tool IDs of shape `prompt:<template_id>:render` to a render request with the template id and incoming params as variables.

**Why.** `prompt-registry` is becoming legacy (zero production consumers). `prompt-template-service` is the canonical prompt surface going forward (used by workflow-runtime, where future SDLC skills via F1b will also live). Migrating Alfred now avoids a second migration later. Patching the legacy service would commit us to maintaining both.

**Alternatives considered:**
- Implement `/v1/invoke` in `prompt-registry` as the smallest possible fix. Rejected: doubles the prompt-service maintenance surface, conflicts with the consolidation direction.
- Add a thin proxy in `prompt-registry` that forwards to `prompt-template-service`. Rejected: adds a hop, hides the real service.

### D4 — Config via env var, consistent with ai-flow-authoring convention

**Decision.** Add `prompt_template_service_url` to `services/alfred/alfred/config.py`, populated from env var `PROMPT_TEMPLATE_SERVICE_URL`. Default to the dev-mode value already present in `portal/.env.example` (set by ai-flow-authoring task 13.2).

**Why.** Stays in lockstep with how other services discover prompt-template-service. No new convention.

### D5 — `data_classification` defaults to `"internal"`, callers can override

**Decision.** `RequestContext.data_classification` defaults to `"internal"`. Reasoning-loop callsites pass `"internal"` implicitly. Callsites that handle restricted user data (none today, but future SDLC skills via F1b will) pass an explicit higher classification.

**Why.** A safe-but-permissive default avoids forcing every existing callsite to change. Header value will always be present (never null). Routing rules in F0c can refine behavior per classification value.

**Alternatives considered:**
- No default, require every callsite to specify. Rejected: too disruptive for a bug fix.
- Default to `"public"`. Rejected: too permissive; Alfred's reasoning context is internal-by-nature.

### D6 — Tool ID format: `prompt:<template_id>:render`

**Decision.** Keep the existing `prompt:*` namespace but enforce a `prompt:<template_id>:render` shape. Any other shape (e.g., `prompt:foo` without `:render`) raises a clear error rather than guessing.

**Why.** Explicit action suffix leaves room for `prompt:<template_id>:validate` or `prompt:<template_id>:describe` later without ambiguity. Current code has no other shape in use (only `:invoke` which we are removing), so this is forward-compatible.

## Risks / Trade-offs

- **Risk: `data_classification` default of `"internal"` might mask future cases where Alfred handles `confidential`/`restricted` data.**
  Mitigation: F0c (routing) will add policy enforcement per classification. For now, document the default loudly in `RequestContext` docstring and add a TODO/note for future callsite audit.

- **Risk: prompt-template-service is itself still a stub (`StubRenderer`).**
  Mitigation: Acceptable. The Quick-fix only requires the contract to work, not the renderer to be production-grade. F1b later loads real production templates. Until then, `prompt:*` calls return stub renders, which is still strictly better than the current silent failure.

- **Risk: Hidden callsites of `LiteLLMClient` that don't have a session context.**
  Mitigation: Grep the codebase before changing the signature. If any caller cannot supply context, either provide a sensible default `RequestContext.system()` for system-internal calls, or refactor that caller to thread context through. Investigate during implementation.

- **Risk: Tests assume the old `prompt-registry` path.**
  Mitigation: Grep for tests referencing `prompt-registry` invocation. Update or delete.

- **Risk: Production rollout of header injection changes how cost rows look in finops.**
  Mitigation: Coordinate with ops before merging — the cost dashboards will start showing tenant-attributed rows where they previously showed unattributed ones. Document the cutover in the change log.

- **Risk: prompt-template-service URL not configured in non-local environments.**
  Mitigation: Verify `PROMPT_TEMPLATE_SERVICE_URL` is set in staging/prod deployment manifests as part of the rollout. Surface a clear startup error if absent (Alfred refuses to start without it).

## Migration Plan

1. Land the code change with header injection and `ToolRouter` migration behind no feature flag (the bugs are bad enough that a flag adds risk without value — the new path is strictly better than the broken old one).
2. Coordinate with ops: confirm `PROMPT_TEMPLATE_SERVICE_URL` set in all environments before merge.
3. Deploy to staging. Verify:
   - Sample LiteLLM logs show the four headers on every Alfred request.
   - A `prompt:test-template:render` tool invocation succeeds (returns stub-rendered content).
   - No new errors in Alfred logs.
4. Promote to production.
5. Verify finops dashboards begin attributing Alfred cost rows to the correct tenant/workspace within one cost-rollup cycle.
6. Update change log noting the cutover.

**Rollback.** Revert the merge. Cost attribution returns to broken state; `prompt:*` tools return to silent failure. No data loss either direction. Rollback is safe.

## Open Questions

- Does Alfred have any non-session-bound LLM calls today (e.g., a startup health check, a background cron)? If yes, those need a `RequestContext.system()` factory. Grep during implementation.
- Should the `correlation_id` header value also include a sub-span identifier for the specific LLM call (e.g., `<correlation_id>.<step_index>`)? Probably yes for downstream debugging, but defer to the F0b SDK rather than over-engineering here.
- What error does Alfred surface to the user if `prompt-template-service` is unreachable? Should match Alfred's existing patterns for upstream failures (probably: fail the tool call with a `tool_unavailable` reason and let the reasoning loop decide whether to retry or escalate).
