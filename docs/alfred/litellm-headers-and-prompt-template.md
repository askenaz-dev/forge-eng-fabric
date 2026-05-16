# Alfred — LiteLLM headers and prompt-template-service

This note documents the contracts Alfred relies on for tenant-attributed LLM
calls (G1) and for `prompt:*` tool dispatch (G2). Shipped by the
`alfred-litellm-header-injection` change.

## Required env vars

| Var | Required | Purpose |
| --- | --- | --- |
| `LITELLM_URL` | yes | LiteLLM gateway base URL. Alfred MUST NOT call providers directly. |
| `LITELLM_KEY` | yes | Bearer key for LiteLLM. |
| `PROMPT_TEMPLATE_SERVICE_URL` | **yes** | Base URL for `prompt-template-service`. **Alfred refuses to start without it.** |

`PROMPT_TEMPLATE_SERVICE_URL` has no default — a missing or empty value
causes `load_settings()` to raise so we never silently route prompt tools
to a stale or unconfigured target. In local dev set
`PROMPT_TEMPLATE_SERVICE_URL=http://localhost:8099` (see
[`portal/.env.example`](../../portal/.env.example)).

## Standard headers on every LLM call

Every outbound request Alfred makes to LiteLLM (chat completions,
embeddings, any future endpoint) carries these four headers:

| Header | Source |
| --- | --- |
| `forgetenantid` | `RequestContext.tenant_id` — derived from JWT claim `forge_tenant_id` / `tenant_id`, or the explicit body field on `POST /v1/intents`. |
| `forgeworkspaceid` | `RequestContext.workspace_id` — the workspace owning the active session. |
| `forgecorrelationid` | `RequestContext.correlation_id` — the correlation id used to thread the request through downstream services. |
| `data_classification` | `RequestContext.data_classification` — defaults to `internal`; callers MAY pass `public`, `confidential`, or `restricted`. |

Missing tenant, workspace, or correlation causes `LiteLLMClient` to raise
`LiteLLMHeaderError` BEFORE issuing any HTTP call. This is intentional:
unattributed requests pollute finops dashboards and break the budget
contract the upcoming F7 billing front depends on.

System-internal callsites (e.g. a future startup health check) use
`RequestContext.system(correlation_id=...)` which populates tenant and
workspace as the sentinel `"system"`.

## Prompt tool dispatch

The `ToolRouter` accepts exactly one shape for prompt tools:

```
prompt:<template_id>:render
```

Calls are dispatched to:

```
POST {PROMPT_TEMPLATE_SERVICE_URL}/v1/render
Content-Type: application/json
{"ref": "<template_id>", "variables": {<tool params>}}
```

Any other shape (notably the legacy `prompt:<id>:invoke`) raises
`InvalidPromptToolId` with a message naming the supported shape. Network
errors and non-2xx upstream responses raise `ToolUnavailable` so the
reasoning loop can decide whether to retry or escalate.

## Why this matters

- **Cost attribution.** Without tenant headers, LiteLLM's per-request cost
  rows cannot be partitioned by tenant/workspace. Finops dashboards lose
  signal and the F7 billing rollup mis-attributes spend.
- **Prompt dispatch.** The legacy `prompt-registry/v1/invoke` endpoint
  does not exist in `services/prompt-registry/`; every `prompt:*` tool
  call was silently 404'ing. The canonical surface is
  `prompt-template-service`'s `/v1/render`, already consumed by
  `workflow-runtime`'s LLM executor (shipped by archived
  `ai-flow-authoring` 2026-05-16). Migrating Alfred to the same target
  avoids maintaining two prompt services.

## References

- OpenSpec change: [`alfred-litellm-header-injection`](../../openspec/changes/alfred-litellm-header-injection/)
- Umbrella roadmap: [`intent-to-infrastructure-gap-closure`](../../openspec/changes/intent-to-infrastructure-gap-closure/)
- Spec delta: [`alfred-control-plane`](../../openspec/changes/alfred-litellm-header-injection/specs/alfred-control-plane/spec.md)
