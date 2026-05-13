# Skill gateway — overview

The **developer skill gateway** is the public face of the Forge Asset Registry. Where the registry catalogues what your tenant has approved, the gateway is what hands those skills, MCPs and agents to the developer's IDE — Claude Code, GitHub Copilot, OpenAI Codex, Cursor, Gemini CLI, OpenHands, Junie, and the rest of the [Agent Skills](https://agentskills.io) ecosystem.

## Why this exists

The platform already ran every approved asset *inside* its own runtime — for Alfred, for workflows, for the SDLC agents. Developers, though, work inside coding agents that live on their laptops. Without the gateway they could not:

- Discover what governed skills their tenant has approved.
- Install one with a single command and have it run in their local agent.
- Invoke an approved MCP server over the network instead of running it themselves.
- Delegate a task from their external agent into a Forge agent over A2A.

The gateway closes that gap **without weakening any platform guarantee**: identity, policy, audit, eval and budget all keep applying.

## What the gateway is

1. A public HTTPS service (`services/skill-gateway`) per-tenant subdomain (e.g. `https://acme.forge.dev`).
2. A signed Agent Skills package endpoint — every install is content-addressed (sha256) and cosign-signed with an in-toto attestation.
3. An MCP reverse proxy — Streamable HTTP + SSE, with identity injection so the MCP server sees the developer principal, never the gateway.
4. An A2A endpoint — JSON-RPC `tasks/send|get|cancel|sendSubscribe` for external agents calling into Forge agents.
5. PAT issuance + revocation, with OIDC device-code login for the bootstrap.

## What the gateway is NOT

- It is not a public marketplace where third parties publish to *your* gateway. Each tenant's gateway serves what *that tenant's* registry has approved.
- It does not duplicate policy logic. The registry's `invoke-check` and the platform's `policy-engine` remain the only allow/deny gate.
- It is not a federation layer between Forge instances — different tenants on different Forge deployments do not see each other.

## Lifecycle in one diagram

```
publish (CI)                          consume (developer)
─────────────                         ──────────────────────
 source ──► packager ──► signed       forge skills list
                          bundle      forge skills install <name>
                            │                │
                            ▼                ▼
                  registry/asset_package    bundle extracted into the
                            │                client's skills directory
                            ▼                (~/.claude/skills/, etc.)
            distribution.gateway_published   MCP entry inserted into
                            │                the client's MCP config
                            ▼                under `forge:` namespace
                      gateway lists ◄─────── developer invokes from
                      and serves              their agent → MCP proxy
                                                   │
                                                   ▼
                                          policy + audit + telemetry
                                                   │
                                                   ▼
                                            asset-observability
                                            (source=gateway)
```

## How an external invocation stays governed

Every gateway request:

1. Carries a PAT bearing `(developer_sub, tenant_id, assume_workspace_id, scopes)`.
2. Passes per-PAT rate limits and the Tenant LLM budget probe before any model call.
3. Is forwarded with `X-Forge-Principal | Tenant | Workspace | Correlation-Id` headers; the MCP runtime ignores any conflicting claims in the payload.
4. Triggers a CloudEvent (`com.forge.gateway.invocation.v1`) that the asset-observability service ingests as `source=gateway`.

## Where to read next

- Install matrix per client → `docs/skill-gateway/install.md`
- How to publish a skill → `docs/skill-gateway/publishing.md`
- MCP + A2A endpoint reference → `docs/skill-gateway/mcp-and-a2a.md`
- Security model → `docs/skill-gateway/security.md` and `docs/skill-gateway/threat-model.md`
