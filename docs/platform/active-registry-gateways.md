# Active Registry Gateways

**Status:** GA target Release N+3 · **Owner:** Platform Engineering · **Spec:** [openspec/changes/active-registry-gateways](../../openspec/changes/active-registry-gateways/)

Forge runs every LLM call through one governed seam — [model-gateway](../../openspec/specs/model-gateway/spec.md) (LiteLLM). This change extends the same pattern to the other three asset families: skills, MCPs and agents. Each family gets a **three-pillar** treatment.

```
┌─────────────────────────────────────────────────────────────────┐
│  Catalog (metadata + lifecycle)                                 │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────┐  │
│  │   Skills    │ │    MCPs     │ │   Agents    │ │   LLMs   │  │
│  └──────┬──────┘ └──────┬──────┘ └──────┬──────┘ └────┬─────┘  │
│         │               │               │             │        │
│  How-to (install + usage + env)                                │
│         │               │               │             │        │
│  Gateway (runtime invocation seam — signed identity + OPA      │
│   + Tenant budget + audit + per-invocation telemetry)          │
│         │               │               │             │        │
│  ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐ ┌────▼─────┐  │
│  │ skill-      │ │ mcp-gateway │ │ a2a-gateway │ │  model-  │  │
│  │ artifact-   │ │             │ │             │ │ gateway  │  │
│  │ store       │ │             │ │             │ │ (LiteLLM)│  │
│  └─────────────┘ └─────────────┘ └─────────────┘ └──────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

The catalog is the [Asset Registry](../../openspec/specs/ai-asset-registry/spec.md). The how-to and active surface blocks live on the registry row. The gateways are the runtime seams.

## Three pillars in one line

- **Catalog** — the registry asset row, with lifecycle / trust / eval scores / `provenance`.
- **How-to** — `how_to_json` on the asset row: install command per client (claude-code, cursor, cli), usage snippets per language, env-var list. Promotion to `approved` requires this block.
- **Gateway** — `active_surface_json` on the asset row: `family ∈ {mcp, a2a, skill}` plus a runtime endpoint (gateway URL) or artifact pointer (adapter URI + digest + signature).

## Capabilities introduced

| Capability | Service / Package | Spec |
|---|---|---|
| MCP-traffic gateway (HTTP + SSE) | [`services/mcp-gateway`](../../services/mcp-gateway/) | [mcp-gateway/spec.md](../../openspec/changes/active-registry-gateways/specs/mcp-gateway/spec.md) |
| A2A-protocol gateway (JSON-RPC + sendSubscribe) | [`services/a2a-gateway`](../../services/a2a-gateway/) | [agent-to-agent-gateway/spec.md](../../openspec/changes/active-registry-gateways/specs/agent-to-agent-gateway/spec.md) |
| Pluggable artifact-store adapter | [`pkg/artifact-store-adapter`](../../pkg/artifact-store-adapter/) | [skill-artifact-store/spec.md](../../openspec/changes/active-registry-gateways/specs/skill-artifact-store/spec.md) |
| Compatibility shim — runtime → MCP | [`pkg/mcp-shim`](../../pkg/mcp-shim/) | §6.1 |
| Compatibility shim — runtime → A2A | [`pkg/a2a-shim`](../../pkg/a2a-shim/) | §6.2 |
| Pinned-asset enforcement at orchestration | [`services/workflow-runtime`](../../services/workflow-runtime/) | §6.3 |

## Where the seam fits in a request

A runner invoking `github.create_pr`:

1. Runtime executes a step of `type=mcp, ref="github"`. Engine checks `selected_assets` (deny with `asset_not_pinned` + `guardrail.trip.v1` if pinned set excludes it).
2. Runtime calls `pkg/mcp-shim.InvokeTool(ctx, "github", "create_pr", body)` (one-line code change vs. the old direct dial).
3. Shim POSTs `/v1/gw/mcp/github?tool=create_pr` on `mcp-gateway` with `Authorization: Bearer <workload-identity-jwt>`.
4. `mcp-gateway`:
   - Rate-limits (per Tenant/Workspace, Redis-backed).
   - Resolves the active surface from the registry (`GET /v1/assets/github`).
   - Calls policy-engine (`forge.gateway.mcp.allow`).
   - Calls tenant-budget (`POST /v1/budget/check`, `family=mcp`).
   - Signs identity headers (Ed25519, JWKS-published).
   - For external MCPs only: resolves the per-Tenant credential from Vault and attaches it as Bearer.
   - Relays the call (HTTP or SSE with bounded backpressure).
   - Emits `com.forge.mcp.invocation.v1` keyed by `tenant_id` with `source ∈ {internal, external_proxy}`.
5. Asset-observability ingests the event into the per-asset rollup.
6. The runner gets the response back.

A2A traffic follows the same flow with two extras: inbound calls (external agents → internal Forge agents) carry a separate `X-Forge-Partner-Auth` header verified by the gateway's partner-store; outbound calls inject `principal_kind=service`, inbound calls inject `principal_kind=external_agent`.

## What a pinned set looks like

The intent-capture wizard and visual editor let the user pin the asset set at design time:

```yaml
selected_assets:
  skills: [skill-foo, skill-bar]
  mcps:   [github, jira]
  agents: [forge-sdlc-architect]
```

The runtime carries this into the execution row and refuses any asset invocation outside the lists during orchestration with `403 asset_not_pinned`. Empty `selected_assets` preserves the pre-change behavior (no pinning).

## Operator surfaces

- **Portal → Assets → \<asset\>** — How-to and Gateway tabs render `how_to_json` / `active_surface_json`.
- **Portal → Platform → External integrations** — register external MCPs and A2A partners per Workspace; manage credential refs (vault paths only — secrets never appear here).
- **Portal → Alfred Wizard → Pin assets** — three filterable lists, one per asset family.
- **Portal → Workflows → Editor** — palette sources from gateway catalogs; pinned entries sort first; off-pin entries prompt on add.

## How traffic is observed

- Metrics: `mcp_gateway_*` and `a2a_gateway_*` series on each gateway's `/metrics` endpoint; scraped into Prometheus and shown on the **Forge · Active Registry Gateways** Grafana dashboard ([infra/grafana/dashboards/active-registry-gateways.json](../../infra/grafana/dashboards/active-registry-gateways.json)).
- Events: every invocation emits a CloudEvents-shaped envelope onto `forge.events`. `asset-observability` ingests them into the per-asset rollup with the `source` label preserved.
- Alerts: SLO breaches and drift (see [deploy/compose/prometheus/rules/active-registry-gateways.yml](../../deploy/compose/prometheus/rules/active-registry-gateways.yml)).

## What changes for skill artifacts

The platform stores the artifact's digest, cosign signature, in-toto attestation and an adapter pointer on the asset row. The bytes themselves live in a tenant-configured private artifact store (Nexus, Artifactory, GitHub Packages private, AWS CodeArtifact). Public NPM is rejected at binding time. The decision matrix and per-driver capability flags are in [ADR-0002](../governance/adrs/0002-artifact-store-adapter.md).

## Rollout

The migration is staged across four releases (N → N+3) with a per-Tenant `gateway.enforced` flag and a compatibility shim during the cut-over. The complete plan is in [docs/platform/active-registry-gateways-rollout.md](active-registry-gateways-rollout.md) and links to per-release runbooks.

## Deeper dives

- ADR · [0002-artifact-store-adapter](../governance/adrs/0002-artifact-store-adapter.md) — why an adapter beats a native store.
- Runbook · [enrolling-an-external-mcp](../runbooks/active-registry-gateways/enrolling-an-external-mcp.md).
- Runbook · [enrolling-an-a2a-partner](../runbooks/active-registry-gateways/enrolling-an-a2a-partner.md).
- Runbook · [configuring-artifact-store-binding](../runbooks/active-registry-gateways/configuring-artifact-store-binding.md).
- Runbook · [responding-to-drift-alerts](../runbooks/active-registry-gateways/responding-to-drift-alerts.md).
