# Skill gateway — MCP and A2A endpoints

Reference for power users wiring MCP servers or A2A agents directly, without the `forge` CLI.

## MCP proxy

```
{HTTP, SSE} /v1/gateway/mcp/{asset_id}
```

Accepts: any MCP Streamable HTTP or SSE request. The gateway adds these headers on the outbound request before forwarding to the MCP runtime:

- `X-Forge-Principal: <developer_sub>`
- `X-Forge-Tenant: <tenant_id>`
- `X-Forge-Workspace: <assume_workspace_id>`
- `X-Forge-Correlation-Id: <uuid>`

Conflicting identity claims inside the inbound MCP payload are ignored; the MCP SDK records `header_override=true` in audit when this happens.

### Manual config (without the CLI)

Claude Desktop's `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "forge:github": {
      "transport": "http",
      "url": "https://acme.forge.dev/v1/gateway/mcp/<github-asset-id>",
      "headers": { "Authorization": "Bearer ${FORGE_TOKEN}" }
    }
  }
}
```

VS Code workspace's `.vscode/mcp.json` uses the same shape under `servers` (Claude Desktop uses `mcpServers`).

### Error responses

| Status | Meaning |
|---|---|
| 401 | PAT missing, expired, or revoked |
| 403 | `missing_scope: gateway.invoke` or `asset_not_in_allowlist` or `cross_workspace_denied` |
| 409 | `remote_transport_unavailable` — the asset is stdio-only |
| 429 | Per-PAT rate limit; honour `Retry-After` |
| 402 | Tenant LLM budget exhausted |

## A2A endpoint

```
POST /v1/gateway/a2a/{asset_id}
content-type: application/json
authorization: Bearer <pat>
```

Body: A2A JSON-RPC envelope. Supported methods: `tasks/send`, `tasks/get`, `tasks/cancel`, `tasks/sendSubscribe` (streamed over SSE).

Example `tasks/send`:

```json
{
  "id": "task-1234",
  "sessionId": "sess-9",
  "message": {
    "role": "user",
    "parts": [{ "type": "text", "text": "Generate test cases for OpenSpec spec-7" }]
  }
}
```

The gateway terminates the protocol, looks up the agent's upstream URL from the registry's `metadata.a2a_upstream_url`, re-issues the task into the platform with the developer's identity, and streams the response back as SSE events when the client subscribes.

### Asset eligibility

- Only assets of type `agent` are A2A-callable through `/v1/gateway/a2a/`.
- The asset must be `lifecycle_state=approved` and `distribution.gateway_published=true`.
- Approved + T0 assets are still refused — T1+ is the minimum for any gateway exposure.

## Authentication

All non-health endpoints require `Authorization: Bearer <pat>`. Issue PATs via:

```bash
curl -X POST https://acme.forge.dev/v1/gateway/tokens \
  -H "Authorization: Bearer $BOOTSTRAP_OIDC_TOKEN" \
  -d '{
    "tenant_id":"…",
    "assume_workspace_id":"…",
    "scopes":["gateway.read","gateway.install","gateway.invoke"],
    "lifetime":"720h"
  }'
```

The plaintext token is returned in the 201 body once. The CLI does this via the OIDC device-code flow under the hood.
