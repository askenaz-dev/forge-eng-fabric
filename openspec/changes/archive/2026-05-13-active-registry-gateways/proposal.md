## Why

Forge's three asset registries (skills, MCPs, agents) are inert catalogs today — they store metadata, lifecycle and trust levels but they do not *broker* the runtime traffic that uses those assets. The only active gateway pattern we have is [model-gateway](openspec/specs/model-gateway/spec.md) (LiteLLM) for LLMs, where every call goes through one policy/budget/audit/observability seam. Skills, MCPs and agents lack that seam: invocation today is either direct (runtime → MCP container) or, via the in-flight [add-developer-skill-gateway](openspec/changes/add-developer-skill-gateway/proposal.md), only **outbound** to external IDEs. Three concrete consequences:

1. **No private skill artifact backend.** Reference skills live in-tree; there is no internal Nexus/Artifactory-style store for confidential, company-owned, versioned skill packages. Publishing to NPM is not an option for restricted data.
2. **No inbound MCP/A2A aggregation.** External MCPs and external A2A agents cannot be brought into the platform through a single governed seam the way LiteLLM does for external LLM providers. Each integration is bespoke.
3. **Orchestration surfaces are blind to the catalogs at design time.** The [intent-capture-wizard](openspec/specs/intent-capture-wizard/spec.md) and [workflow-visual-editor](openspec/specs/workflow-visual-editor/spec.md) can reference registry assets, but the user cannot pin a curated set of skills/MCPs/agents as part of the end-to-end design — the orchestrator must discover them later.

This change evolves each registry from a catalog into an **active registry gateway**: catalog + how-to + governed runtime gateway, the same pattern LiteLLM applies to LLMs ([litellm.ai/docs/mcp](https://docs.litellm.ai/docs/mcp), [litellm.ai/docs/a2a](https://docs.litellm.ai/docs/a2a)).

## What Changes

- **NEW**: A `mcp-gateway` capability — a bidirectional MCP proxy that fronts every approved MCP, whether internal (registry-owned) or external (third-party, configured per Tenant). Internal callers (Alfred, runners, workflows) and external callers (developer IDEs through `developer-skill-gateway`) hit the same governed seam.
- **NEW**: A `agent-to-agent-gateway` capability — a bidirectional A2A protocol gateway that lets the platform's agents invoke external A2A agents, and vice versa, with identity propagation and policy enforcement, mirroring how LiteLLM fronts LLM providers.
- **NEW**: A `skill-artifact-store` capability — a thin adapter layer over a tenant-configurable enterprise artifact store (Nexus, JFrog Artifactory, GitHub Packages, AWS CodeArtifact). The registry never persists skill bytes; it persists the digest, signature and adapter pointer. NPM is *not* a permitted backend.
- **MODIFIED**: [`ai-asset-registry`](openspec/specs/ai-asset-registry/spec.md) — every asset gains a first-class **`how_to`** block (install command, usage snippets, env requirements) and an **`active_surface`** block (gateway endpoint for runtime consumption: `mcp_gateway` URL, `a2a_gateway` URL, or `artifact_store` pointer). The catalog/how-to/gateway triad is a hard requirement for `approved` lifecycle.
- **MODIFIED**: [`mcp-and-skills`](openspec/specs/mcp-and-skills/spec.md) — MCPs reach the runtime through the new `mcp-gateway`; the spec adds an external-MCP onboarding flow (third-party endpoint + Tenant-scoped credential broker + policy).
- **MODIFIED**: [`agentic-execution-platform`](openspec/specs/agentic-execution-platform/spec.md) — runners invoke MCPs and remote agents through the new gateways, not directly. Direct dial is blocked by network policy, the same posture used today for LiteLLM.
- **MODIFIED**: [`intent-capture-wizard`](openspec/specs/intent-capture-wizard/spec.md) — wizard surfaces the three gateway catalogs and lets the user pin skills, MCPs and agents into the draft OpenSpec; the pinned references travel with the spec into orchestration.
- **MODIFIED**: [`workflow-visual-editor`](openspec/specs/workflow-visual-editor/spec.md) — palette pulls skills/MCPs/agents from the gateway catalogs (not the raw registry); selecting a node also pins the gateway endpoint that will serve it at runtime.
- **BREAKING**: None for external developers. **Internally**, runners and Alfred must migrate from direct MCP/agent dial to the gateway; the migration is staged behind a feature flag, with a compatibility shim that proxies the existing direct routes during the cut-over.

## Capabilities

### New Capabilities

- `mcp-gateway`: Bidirectional MCP traffic gateway. Fronts internal MCPs (HTTP/SSE transport over the registry-approved set) and aggregates external MCPs (third-party endpoints registered per Tenant). Enforces identity propagation, OPA policy, rate/cost limits and audit on every tool call; emits per-invocation telemetry into [ai-observability](openspec/specs/ai-observability/spec.md).
- `agent-to-agent-gateway`: A2A protocol gateway following the [A2A spec](https://a2aproject.github.io/A2A/). Lets internal agents invoke external A2A agents and lets external agents invoke registered Forge agents, with the same identity/policy/audit seam. Handles task/send, task/get, task/cancel, task/sendSubscribe.
- `skill-artifact-store`: Private artifact store adapter for skill packages. Pluggable backend (Nexus, Artifactory, GitHub Packages private, CodeArtifact); the registry holds only the canonical package digest, cosign signature and adapter pointer. Publishes through the existing CI pipeline using the same in-toto attestation chain as image deploys.

### Modified Capabilities

- `ai-asset-registry`: Adds `how_to` and `active_surface` blocks to every asset; promotes the catalog/how-to/gateway triad as a precondition for `approved`. Adds `external` provenance flag for assets registered as proxies to third-party MCPs/agents.
- `mcp-and-skills`: All MCP invocation flows through `mcp-gateway`. Adds external-MCP onboarding (endpoint + credential broker + per-Tenant allowlist).
- `agentic-execution-platform`: Runners and Alfred consume MCPs through `mcp-gateway` and remote agents through `agent-to-agent-gateway`; direct dial blocked at network policy.
- `intent-capture-wizard`: Wizard exposes the three gateway catalogs and lets the user pin skills/MCPs/agents on the draft OpenSpec; pinned set is validated and traceable.
- `workflow-visual-editor`: Node palette resolves skills/MCPs/agents through the gateways; saved nodes carry both the asset reference and the gateway endpoint that will serve them.

## Impact

- **New services**:
  - `services/mcp-gateway` (Go, chi + JWT, follows the [services/registry](services/registry) template).
  - `services/a2a-gateway` (Go, same template).
  - `pkg/artifact-store-adapter` (Go interface + Nexus, Artifactory, GitHub Packages, CodeArtifact drivers).
- **Modified services**: `services/registry` (new fields + lifecycle precondition), `services/skill-gateway` (delegates MCP/A2A traffic to the new internal gateways rather than terminating it), `services/policy-engine` (new `gateway.*` rules covering inbound proxy decisions).
- **Database**: new tables — `external_mcp_endpoint` (Tenant + URL + credential ref + allowlist), `external_a2a_agent` (Tenant + endpoint + agent-card hash + credential ref), `artifact_store_binding` (Tenant + backend + adapter config); columns added to `asset` (`how_to_json`, `active_surface_json`, `external_provenance`).
- **Events**: `com.forge.mcp.invocation.v1`, `com.forge.a2a.invocation.v1`, `com.forge.artifact.published.v1` (and `external_registered` variants).
- **Portal**: extends the **Assets** section with **How-to** and **Gateway** tabs per asset; new **External integrations** view under Platform for registering third-party MCPs / A2A agents; intent wizard and visual editor gain a **Pin assets** step.
- **Policy/security**: OPA bundles add `gateway.mcp.*` and `gateway.a2a.*` rules; external endpoints require Tenant-scoped allowlist; identity propagation header set carries the calling principal and is signed by the gateway.
- **Observability**: per-asset metrics in [per-asset-observability](openspec/specs/per-asset-observability/spec.md) extend with `source ∈ {internal, external_proxy}`; existing OpenTelemetry/Langfuse plumbing reused.
- **Compose / runtime**: new docker-compose entries for `mcp-gateway` and `a2a-gateway`; production Helm charts; internal-only network policy by default, public exposure only via [`developer-skill-gateway`](openspec/changes/add-developer-skill-gateway/proposal.md).
- **Docs**: a single "Active Registry Gateways" guide covering the three pillars, plus an ADR for the artifact-store-adapter backend selection.
- **Out of scope (deferred)**:
  - Cross-tenant federation between Forge gateways.
  - Workflow and prompt-template asset types staying on the internal marketplace seam.
  - Auto-discovery of external MCPs/agents (registration remains explicit per Tenant).
