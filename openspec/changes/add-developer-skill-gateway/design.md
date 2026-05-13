## Context

Forge already has the inputs for this: a typed Asset Registry with lifecycle and trust levels (`services/registry`), policy enforcement (`services/policy-engine`), a model gateway (LiteLLM in compose) and an internal tenant marketplace for workflows (`services/marketplace`). The gap is the surface that lets developers consume those assets **from outside the platform**, in the agentic clients they actually use — Claude Code, GitHub Copilot, OpenAI Codex, Cursor, Gemini CLI, OpenHands, Junie, Goose, etc. None of those clients can reach into the registry today: there is no public API, no canonical packaging in the open Agent Skills format, no public MCP endpoint, no A2A surface, and no CLI to install. The internal `services/marketplace` covers workflow distribution within a Tenant; it is not a developer-facing surface and intentionally does not expose skills, MCPs or agents.

Constraints we are designing inside:

- The platform already declares LiteLLM as the only path to LLMs and the registry's `invoke-check` as the only allow/deny gate for asset invocation. Anything we build must reuse both, not parallel them.
- Multi-tenant isolation is non-negotiable: cross-tenant visibility is a P0 incident.
- Supply-chain attestations (cosign + in-toto) are already required for in-platform deploys; the gateway must keep them in the package.
- The open Agent Skills format is fixed by the broader ecosystem — we adopt it as-is, we do not extend it.

Stakeholders: Platform Engineering owns the gateway; Security signs off on auth + ingress + supply chain; SDLC Team owns publication policy; DX / DevRel owns the CLI and the client integration matrix.

## Goals / Non-Goals

**Goals:**

- Make every approved, T1+ skill / MCP / agent in a Tenant installable into the developer's local agent client with one command.
- Preserve identity, policy, audit, eval and budget guarantees on every external invocation — the gateway is a thin, hardened wrapper around existing services, not a new policy plane.
- Distribute skills in the open Agent Skills format so every compatible client can load them with zero Forge-specific glue.
- Make MCP servers usable remotely over HTTP/SSE without each developer running them locally.
- Let an external coding agent delegate to a registered Forge agent over A2A.
- Ship a single static `forge` CLI that handles auth, install paths per client, MCP wiring and updates.

**Non-Goals:**

- A cross-tenant federation between Forge instances (deferred).
- Distributing `workflow` or `prompt_template` asset types through the gateway in this change — those keep going through the internal marketplace until we have evidence of demand.
- A new auth provider — we extend the existing Keycloak + OpenFGA stack.
- Hosting a public skills marketplace where third parties publish to *our* gateway. The gateway serves what the Tenant's own registry has approved; multi-tenant marketplaces are a separate problem.
- Replacing `services/marketplace`. The internal marketplace stays for in-platform workflow installs into workspaces.

## Decisions

### 1. New service `services/skill-gateway` vs. extending `services/registry`

**Decision**: Create a separate `services/skill-gateway` service. It is stateless (reads `asset`, `asset_package`, `gateway_token`, `gateway_install` tables), runs behind public TLS ingress, and is the only service exposed outside the cluster.

**Why**: The registry today is an internal service behind a private network, with internal-class auth and rate limits. Public exposure changes the hardening profile — request size limits, CORS, abuse detection, much stricter rate-limits, and an attack surface that we want isolated. Putting the public skin in its own binary keeps the registry's blast radius contained and lets us scale them independently.

**Alternative**: Add public routes to the registry. Rejected — couples public hardening to a core internal service and complicates RBAC.

### 2. Asset → Agent Skills packaging owned by the registry, not the gateway

**Decision**: The packager runs as a step in the CI pipeline that owns a skill's source. It produces a deterministic `.tar.zst`, signs it with cosign, attaches an in-toto attestation, and calls `lifecycle-hooks/gateway-publish` on the registry with `{ channel, package_digest, signature_id, attestation_id }`. The registry stores `asset_package` and emits `assets.gateway_published.v1`. The gateway only serves byte-for-byte what the registry approved.

**Why**: Keeps the trust boundary at the registry (which already gates all asset state changes), and makes the gateway a pure read-side that cannot mint new packages. It also reuses the `pipeline-green` discipline we already have for trust-level promotion.

**Alternative**: Let the gateway re-pack on the fly. Rejected — non-deterministic, can't be attested.

### 3. Open Agent Skills format, no Forge extensions

**Decision**: We adopt the [agentskills.io](https://agentskills.io) layout literally: a folder named after the skill, an `SKILL.md` with YAML front-matter (`name`, `description`, and optional `mcp:` list for MCP wiring), and optional `scripts/`, `references/`, `assets/`. The optional `mcp:` key is the *only* field beyond the standard, and it is purely declarative — clients that ignore it still load the skill correctly.

**Why**: Every compatible client (Claude Code, Copilot, Codex, Cursor, …) already understands the open layout. Adding Forge-specific fields would make our skills second-class everywhere except in our own CLI.

**Alternative**: A Forge manifest in JSON. Rejected — fork of the ecosystem.

### 4. MCP remote transport: Streamable HTTP first, SSE as fallback

**Decision**: The gateway speaks MCP over Streamable HTTP and SSE. Stdio MCPs are not gateway-publishable; the publication hook refuses them with `409 remote_transport_required`. We require every MCP to declare `path_template`, `auth_modes` and `health_path`.

**Why**: Streamable HTTP is the canonical remote transport in the MCP spec and is what every client already supports for remote servers. SSE is the universally-deployed fallback. Stdio is great for local development but cannot survive going public.

**Alternative**: WebSocket. Rejected — not standardized in MCP, more middlebox issues.

### 5. A2A protocol for external-agent → Forge-agent delegation

**Decision**: Use the [Agent2Agent (A2A) protocol](https://google.github.io/A2A/) — JSON-RPC over HTTP with SSE streaming, `tasks/send`, `tasks/get`, `tasks/cancel`, `tasks/sendSubscribe`. Each registered agent gets a stable URL `POST /v1/gateway/a2a/{asset_id}`. The gateway terminates the protocol, looks up the agent's runtime endpoint and re-issues the task inside the platform with the developer's identity.

**Why**: A2A is the closest thing to a standard for agent-to-agent task delegation, supported (or coming) in several of the listed clients. It maps cleanly onto our existing agent runtime contract.

**Alternative**: A bespoke `POST /invoke` envelope. Rejected — re-inventing what A2A solves.

### 6. Auth: PATs for the common case, OIDC device-code for `forge login`

**Decision**:

- `forge login` runs OIDC device-code against Keycloak via the gateway's `/v1/gateway/auth/device` endpoint. The refresh token is stored in the OS keystore.
- Daily CLI traffic, MCP proxy and A2A invocation all use **personal access tokens** that the CLI mints from the refresh token, scoped to `(developer_sub, tenant_id, assume_workspace_id, scope_list)` with a 90-day max lifetime.
- PATs are hashed at rest (argon2id); revocation propagates via Redis pub/sub to all gateway replicas within 5s.

**Why**: PATs are simple, revocable, easy to bind to a single workspace, easy to allowlist by asset, and easy to use from `FORGE_TOKEN=...` in CI. OIDC tokens are great for the bootstrap but awkward for headless and per-tool use.

**Alternative**: OIDC end-to-end (no PATs). Rejected — refresh-token rotation through every MCP/A2A call is operationally hostile.

### 7. CLI client-detection table is data, not code

**Decision**: The list of supported clients and their install paths lives in a JSON table embedded in the CLI binary and reproduced in `docs/clients.md`. Updates to add a new client are a docs + CLI release, not a gateway release.

**Why**: The client ecosystem moves fast. Decoupling the gateway from the client list means we onboard new editors without redeploying the public service. The same data can be served by `GET /v1/gateway/clients` so older CLIs can learn about new clients without an upgrade — but that endpoint is best-effort.

### 8. Reuse existing observability and audit fabrics

**Decision**: Every gateway request emits the standard OpenTelemetry trace and a CloudEvent (`com.forge.gateway.invocation.v1` or `…installed.v1`) onto Kafka. The asset-observability service grows a `source=gateway` ingester. No new telemetry plane.

**Why**: One place to query per-asset metrics, one drift detector, one cost rollup.

### 9. Deployment shape

**Decision**: `services/skill-gateway` ships as a Helm chart deployed behind a public ingress with mTLS optional, request body cap 8 MB, per-PAT rate limits enforced at the gateway (Redis token-bucket), and HPA on CPU + tail latency. Package bytes are served from the registry's object store (S3-compatible) with the gateway as a signed redirector for large bundles (>5 MB).

### 10. Anti-abuse posture

**Decision**: Every PAT request emits a fingerprint (IP /24 + UA + dev_sub) to a Redis sliding-window. Abuse rules (failed-auth spike, scope-escalation attempts, off-allowlist asset access) trip the existing kill-switch and quarantine the PAT for human review.

## Risks / Trade-offs

- **[Public service expands the blast radius]** → Mitigation: separate binary, separate VPC, separate Postgres role (read-only on `asset`, write only on `gateway_*`), mandatory mTLS-optional + WAF in front, weekly automated abuse drills.
- **[PAT leakage is a developer's own laptop hygiene]** → Mitigation: 90-day max lifetime, asset allowlist, immediate revocation, anomaly detection on geographic IP shifts, OS-keystore-only storage in the CLI, no `~/.forge/token`.
- **[Open Agent Skills standard could fork over time]** → Mitigation: keep our packager output strictly within today's standard; if the standard splits, freeze on the most-compatible profile and emit telemetry showing what clients consume.
- **[MCP traffic costs us LLM tokens via brokered keys]** → Mitigation: every LLM call still goes through LiteLLM with the Tenant budget; gateway refuses with `402 budget_exhausted` *before* model dispatch.
- **[Drift between in-platform and external eval samples]** → Mitigation: per-asset observability already correlates by `(asset_id, version)`; drift detector grows a `source=gateway` baseline series; alerts identify the contributing source.
- **[A2A spec is young]** → Mitigation: keep the A2A surface behind a `/v1/gateway/a2a/` versioned prefix; commit only to the methods listed in the spec at v0.x; gate behind a feature flag for early Tenants.
- **[CLI release cadence vs new clients]** → Mitigation: client table is data not code; CLI can also `--client generic` to drop into `~/.agentskills/`.
- **[Stale installed skills on developer laptops]** → Mitigation: CLI `forge skills status` shows drift; gateway sets `Cache-Control: max-age=300`; deprecated assets surface their `deprecation_pointer` in `list` output.

## Migration Plan

Phase 0 — Foundations (one release cycle):

- Migrations: `gateway_token`, `gateway_install`, `asset_package`. Extend `asset` with `distribution_*` columns (nullable, default off).
- Registry: implement `lifecycle-hooks/gateway-publish`, the `distribution` block in payloads, and the new events.
- Packager: ship in the platform-pipelines repo as a reusable GitHub Action / Tekton task; back-publish the three reference skills (`generate-test-cases`, `create-user-stories`, `scaffold-service`) at T2.

Phase 1 — Gateway + CLI (next release cycle):

- Stand up `services/skill-gateway` behind public ingress in staging tenant.
- Ship `forge` CLI 0.1 (login, list, install, status, remove) for macOS / Linux. Windows in 0.2.
- Document install paths for Claude Code + Claude Desktop + Copilot + Codex + Cursor; add the rest behind `--client`.
- Portal **Skill gateway** section: read-only — show publication state, recent installs, recent invocations, revoke buttons.

Phase 2 — A2A + remote MCPs (following cycle):

- Light up `/v1/gateway/mcp/{id}` against `github-mcp` and `jira-mcp` first.
- Light up `/v1/gateway/a2a/{id}` against one approved agent.
- Per-asset observability `source=gateway` series + drift baseline update.

Rollback strategy at each phase:

- Distribution columns nullable → toggling `gateway_published=false` on every asset disables external visibility while keeping schemas intact.
- Public ingress can be removed by deleting the Helm release; nothing depends on the gateway being up except external developers.
- CLI is opt-in; rolling back is uninstalling the binary.
- The internal marketplace and runtime are untouched in every phase, so internal flows are unaffected by gateway rollback.

## Open Questions

- Do we let a developer with `gateway.invoke` consume *team-visibility* assets from another workspace of the same tenant they're a member of, or do we require explicit `assume_workspace_id` per PAT? Current decision: per-PAT, but we may relax for power users.
Answer: We can have assets per workspace, and public assets per tenant. The former are only visible to developers with access to that workspace; the latter are visible to anyone in the tenant with `gateway.invoke` permission. This allows for both private and shared assets within a tenant, while maintaining clear boundaries.
- How do we surface eval drift coming from external use back to the asset owner? Probably a Portal banner + email to `owners`, but the channel needs Security/DX sign-off.
Answer: We can implement a monitoring system that tracks the performance of assets invoked through the gateway. If we detect significant drift in eval metrics compared to in-platform usage, we can trigger alerts that notify asset owners via email and display a banner in the Portal. This way, owners are promptly informed of any issues arising from external usage.
- Pricing/budget allocation for gateway-originated LLM cost: Tenant-wide bucket, or per-PAT cap on top of Tenant? Likely Tenant-wide initially with a per-PAT soft cap to limit single-key blast radius.
Answer: We can start with a Tenant-wide budget for LLM costs incurred through the gateway, which simplifies billing and management. To mitigate the risk of a single PAT causing excessive costs, we can implement a per-PAT soft cap that triggers warnings or temporary throttling when exceeded. This approach balances ease of use with cost control.
- Where does the public gateway URL live per Tenant — single regional `https://gw.forge.acme.io` or per-Tenant subdomain `https://acme.forge.dev`? Per-Tenant subdomain has nicer trust UX; single URL is simpler ops. Decide before Phase 1 cutover.
Answer: Opting for a per-Tenant subdomain (e.g., `https://acme.forge.dev`) provides a better trust experience for developers, as it clearly indicates the association with their specific tenant. While it may introduce some operational complexity, the improved user experience and clearer branding are likely worth the trade-off.
- Cross-client MCP config wars: Claude Desktop expects `claude_desktop_config.json`, Cursor uses its own file, VS Code is moving toward `.vscode/mcp.json`. The CLI handles them per-client, but the standardization effort upstream might converge — we should track and reduce CLI complexity over time.
Answer: for now, the CLI will maintain a mapping of client-specific MCP config file names and locations. As the ecosystem evolves, we can monitor for convergence on a standard config format and location. If a consensus emerges, we can update the CLI to support that standard, reducing complexity and improving interoperability across clients.