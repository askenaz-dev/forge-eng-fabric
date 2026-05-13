## Why

Forge already registers, governs and serves skills, MCP servers and agents inside its own runtime (registry + workflow-runtime + LiteLLM), but developers cannot consume any of that from the agentic clients they actually use every day — Claude Code, GitHub Copilot, OpenAI Codex, Cursor, Gemini CLI, OpenHands, and the rest of the [Agent Skills](https://agentskills.io) ecosystem. The asset registry is an internal catalog; there is no public surface that distributes governed assets to developer IDEs, no canonical packaging in the open Agent Skills format, and no remote endpoint for MCPs / agent-to-agent calls authenticated against a developer identity. We need a **developer skill gateway** so that everything the platform certifies (T1+ skills, approved MCPs, approved agents) becomes one `forge skills install <name>` away from any compatible client, with the same policy / audit / eval guarantees as inside the platform.

## What Changes

- **NEW**: A `services/skill-gateway` service that exposes a developer-facing API (token-auth) for discovery, installation, MCP proxy and agent-to-agent invocation of approved Asset Registry items.
- **NEW**: A canonical packager that converts a registry asset of type `skill` into the open Agent Skills format (`SKILL.md` + `scripts/` + `references/` + `assets/`) and produces a signed, content-addressed bundle.
- **NEW**: Remote MCP transport (HTTP + SSE) for any registry-approved MCP server, surfaced through the gateway with per-developer identity propagation, secret brokering and policy checks.
- **NEW**: An Agent2Agent (A2A) endpoint that lets an external coding agent invoke a registered Forge agent as a remote skill, preserving correlation/audit.
- **NEW**: A small `forge` CLI (`forge skills install|list|search|update|remove|login`) that resolves the correct install directory per client (Claude Code, Claude desktop, Copilot, Codex, Cursor, Gemini CLI, Junie, OpenHands, OpenCode, VS Code), writes the package, and registers MCP/A2A endpoints into the client's config.
- **NEW**: Developer identity model — personal access tokens (PAT) scoped to a Tenant + Workspace + optional asset allowlist, OIDC device-code login, revocation and rotation.
- **MODIFIED**: `ai-asset-registry` gains a `distribution` block (gateway publication state, semver channel, package digest, deprecation pointers) and an `assets.published.v1` lifecycle event.
- **MODIFIED**: `mcp-and-skills` requires every approved MCP to declare its remote-transport contract (stdio is still allowed in-platform; HTTP/SSE is required to be gateway-installable).
- **MODIFIED**: `delegated-permissions` adds an `external_developer` principal class with the scopes the gateway accepts.
- **MODIFIED**: `per-asset-observability` ingests gateway invocations as a new source so install/usage telemetry rolls up next to in-platform usage.
- **BREAKING**: None. The gateway is additive; existing internal flows (Alfred, workflows, marketplace installs) keep working unchanged.

## Capabilities

### New Capabilities

- `developer-skill-gateway`: Public, authenticated API surface that lists installable assets, serves Agent Skills packages, proxies MCP traffic and accepts A2A invocations, with per-developer auth, policy enforcement, rate/cost limits and audit.
- `agent-skill-packaging`: Deterministic packaging of Asset Registry skills (and minimal agents) into the open Agent Skills format, producing signed, content-addressed bundles that any compatible client can load.
- `forge-developer-cli`: `forge` CLI that authenticates against the gateway, resolves client-specific install paths, installs/updates/removes skill packages, and writes MCP/A2A endpoints into the local client configuration.

### Modified Capabilities

- `ai-asset-registry`: Adds distribution metadata, gateway-publication lifecycle hook, and the `com.forge.asset.gateway_published.v1` event. Constrains which assets are gateway-eligible (approved + T1 minimum + remote-transport contract).
- `mcp-and-skills`: Every approved MCP must declare a remote-transport contract (HTTP/SSE) so it can be served through the gateway; stdio-only MCPs remain valid for in-platform but are flagged non-installable.
- `delegated-permissions`: Adds `external_developer` principal class, PAT scopes (`gateway.read`, `gateway.install`, `gateway.invoke`), and the rules that map a developer's PAT to an `assume_workspace` context.
- `per-asset-observability`: Adds `source=gateway` ingestion path so per-asset metrics include external IDE usage (installs, invocations, p50/p95, cost) alongside internal usage.

## Impact

- **New services**: `services/skill-gateway` (Go, chi+JWT, follows the same telemetry/audit/kafka patterns as `services/registry`); `cli/forge` (Go, single static binary).
- **New surfaces**:
  - `GET  /v1/gateway/assets` (list installable)
  - `GET  /v1/gateway/assets/{id}@{version}/package` (signed Agent Skills bundle)
  - `*    /v1/gateway/mcp/{asset_id}` (HTTP+SSE MCP proxy)
  - `POST /v1/gateway/a2a/{asset_id}` (A2A invoke)
  - `POST /v1/gateway/tokens` (PAT issue), `DELETE /v1/gateway/tokens/{id}`
  - Device-code OIDC at `/v1/gateway/auth/*`
- **Database**: new tables — `gateway_token`, `gateway_install`, `asset_package` (digest, signature, channel); reuses `asset` + `asset_deployment`.
- **Events**: `com.forge.asset.gateway_published.v1`, `com.forge.gateway.installed.v1`, `com.forge.gateway.invocation.v1`.
- **Portal**: a new **Skill gateway** section under Platform showing publication state per asset, package digest, install instructions per client, recent external invocations and revoke-token UI.
- **Policy/security**: extends OPA bundles with `gateway.*` rules; LiteLLM budget enforcement carries through to gateway-originated LLM calls.
- **Docs**: client-specific install guides (Claude Code, Copilot, Codex, Cursor, Gemini CLI, OpenHands, VS Code), package signature verification, A2A handshake.
- **Compose / runtime**: new docker-compose entry for `skill-gateway`, Helm chart for production, public ingress with mTLS-optional and PAT-required.
- **Out of scope (deferred)**: cross-tenant federation between Forge gateways; non-skills asset types (workflows, prompt templates) staying installable only via the existing internal marketplace until a follow-up.
