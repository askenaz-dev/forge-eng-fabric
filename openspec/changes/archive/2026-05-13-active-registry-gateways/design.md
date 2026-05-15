## Context

Forge has one mature active-gateway pattern: [model-gateway](openspec/specs/model-gateway/spec.md) (LiteLLM). Every LLM call goes through it, which is why we can enforce cost ceilings, data-classification rules, fallbacks, OPA policy and Langfuse telemetry uniformly. We do *not* have that pattern for the other three asset families:

- **Skills** are referenced by id+version in [workflow-visual-editor](openspec/specs/workflow-visual-editor/spec.md) and invoked directly by runners under [agentic-execution-platform](openspec/specs/agentic-execution-platform/spec.md). There is no internal artifact store for skill bytes — references currently resolve to source paths in this repo and to whatever the in-flight [add-developer-skill-gateway](openspec/changes/add-developer-skill-gateway/proposal.md) outputs for *external* developer clients.
- **MCPs** are registered in [mcp-and-skills](openspec/specs/mcp-and-skills/spec.md) and invoked directly by runners or by Alfred. We have a public MCP proxy in `services/skill-gateway` aimed at external IDEs, but no equivalent internal seam, and no story for **external** MCPs that a Tenant wants to use (e.g., a vendor MCP) being fronted by Forge for policy/credentials/audit.
- **Agents** can talk to each other today only through the workflow runtime; there is no protocol-level A2A gateway, and no inbound A2A so that a registered Forge agent can be invoked as a remote skill by an external agent that the platform is also fronting.

LiteLLM has shipped both [`/mcp`](https://docs.litellm.ai/docs/mcp) and [`/a2a`](https://docs.litellm.ai/docs/a2a) gateways with this exact shape: a single proxy seam that aggregates internal + external providers, with policy, audit, observability and budget on every hop. We take that pattern, but stay inside our boundaries:

- We do NOT route through LiteLLM for MCP/A2A — LiteLLM stays the LLM gateway. We ship two parallel internal gateways that follow the same operational pattern.
- We do NOT host skill bytes ourselves. The platform stores metadata + digest + signature; the bytes live in a tenant-configurable enterprise artifact store (Nexus, Artifactory, GitHub Packages private, AWS CodeArtifact) accessed through an adapter.
- We do NOT replace [`developer-skill-gateway`](openspec/changes/add-developer-skill-gateway/proposal.md). That stays as the public TLS-fronted, PAT-authenticated seam for external IDEs; it delegates MCP/A2A traffic to the new internal gateways instead of terminating those flows itself.

Stakeholders:
- **Platform Engineering** owns `mcp-gateway`, `a2a-gateway` and the artifact-store adapter.
- **Security** signs off on identity propagation headers, external-endpoint allowlists, OPA bundles and the artifact-store provenance chain.
- **SDLC Team** owns the policy that an asset cannot reach `approved` lifecycle without catalog + how-to + active-surface populated.
- **DX** owns the wizard/editor pinning UX.

## Goals / Non-Goals

**Goals:**

- One governed seam per asset family for every runtime invocation (internal + external).
- Symmetric model across skills, MCPs and agents: catalog (metadata) + how-to (install/usage) + gateway (active runtime surface).
- Private skill artifact store with the same provenance chain (cosign + in-toto) we already use for image deploys.
- Pinning of specific skills/MCPs/agents at the design surfaces (intent capture wizard + workflow visual editor) so the choice is part of the OpenSpec / workflow AST, not deferred to orchestration.
- External MCPs and external A2A agents are first-class — they are registered as registry assets with `external` provenance and reached through the same gateway as internal ones.

**Non-Goals:**

- A new LLM gateway. LiteLLM stays the only LLM seam.
- A native artifact backend. We adapt to enterprise tools the user already runs.
- Cross-tenant federation across Forge gateways.
- Auto-discovery of external MCPs/agents. Registration is explicit per Tenant.
- Workflows and prompt templates moving off the internal marketplace seam — they stay until a follow-up.
- Re-implementing what `services/skill-gateway` already does for external developer clients.

## Decisions

### 1. Two new internal services (`mcp-gateway`, `a2a-gateway`) vs. one combined gateway

**Decision**: Ship `services/mcp-gateway` and `services/a2a-gateway` as separate Go services, both built from the `services/registry` template (chi + JWT middleware + otelhttp + Kafka producer + audit). They are deployed inside the cluster, reachable only from runners, Alfred and `services/skill-gateway`.

**Why**: MCP and A2A are different protocols with different traffic shapes (MCP: tool-call RPC with SSE streaming for long-running tools; A2A: task lifecycle JSON-RPC with `tasks/sendSubscribe` for streaming). Mashing them into one binary obscures the per-protocol policy and rate-limit shape, complicates the OpenTelemetry semantic conventions per protocol, and creates an outsized blast radius if either side has a CVE.

**Alternatives considered**:
- One combined gateway. Rejected — protocol smell-test fails; we'd end up with two adapters inside one binary anyway.
- Embed the gateways inside `services/skill-gateway`. Rejected — that service is the *public* TLS-fronted edge with PAT auth and stricter hardening; internal callers shouldn't need PATs and shouldn't traverse the public edge.

### 2. Internal MCP/A2A traffic stays internal — the public edge stays in `skill-gateway`

**Decision**: `mcp-gateway` and `a2a-gateway` listen only on the internal cluster network. External developer traffic continues to enter through `services/skill-gateway` (public TLS, PAT-authenticated), which then proxies to the internal gateways. Both internal gateways inject signed `X-Forge-Principal|Tenant|Workspace|Correlation-Id` headers; downstream MCPs/agents trust those headers because the gateway is the only ingress point on the trust boundary.

**Why**: Splits the public attack surface (one service, hardened) from the internal traffic shaping (two services, simple). It also lets us drop PAT validation off the hot path for internal callers, which use the existing service-to-service mTLS + workload identity.

**Alternative**: Let `mcp-gateway`/`a2a-gateway` accept both internal and external traffic. Rejected — doubles the auth surface in two more services and forces them to host TLS + abuse detection.

### 3. Artifact-store adapter layer, not a native store

**Decision**: Add `pkg/artifact-store-adapter` with a Go interface (`Put(ctx, digest, reader)`, `Get(ctx, digest) (reader, error)`, `Stat(ctx, digest) (manifest, error)`, `Delete(ctx, digest)`, `Health(ctx)`) and four initial drivers: Nexus, JFrog Artifactory, GitHub Packages (private), AWS CodeArtifact. Per-Tenant binding (`artifact_store_binding`) selects which driver is active. NPM (public or organization-public) is *not* an offered driver and is rejected if configured.

**Why**: Most enterprise customers already run one of these. Building a native store means owning replication, GC, signing, mTLS, quota and DR for a domain that is already a commodity. The adapter gives us a thin, testable seam without the operational tail.

**Alternatives considered**:
- Native object-store-backed registry inside Forge. Rejected — too much surface for the value; postponed to a follow-up if customer feedback demands it.
- Use the registry DB to store small skill payloads (<1 MB). Rejected — turns the registry into a quasi artifact store and complicates RTO.

**Risks**: Adapter feature mismatch (e.g., not every backend supports retention policies). Mitigation: declare a minimum contract in `pkg/artifact-store-adapter` and gate features behind capability flags exposed via `Health(ctx)`.

### 4. Registry is the source of truth for `how_to` and `active_surface`

**Decision**: The asset record gains `how_to_json` (install command per client, usage snippets per language, env requirements) and `active_surface_json` (gateway endpoint per asset family: `{ family: "mcp", endpoint: "/v1/gw/mcp/{asset_id}" }`, `{ family: "a2a", endpoint: "/v1/gw/a2a/{asset_id}" }`, `{ family: "skill", artifact_pointer: "nexus://forge-skills/foo@1.2.3", digest, signature }`). Promotion to `approved` requires both fields to be populated and validated against the schema. Gateways and Portal read these fields rather than computing them.

**Why**: Single writer + many readers. The Portal "How-to" tab, the gateway response headers and the editor palette all dereference the same record. Computing how-to at read time would mean clients diverge.

**Alternative**: Compute `active_surface` in each gateway. Rejected — three writers, hard to invalidate, and prevents the registry from rejecting a publication that lacks a transport.

### 5. External MCP/A2A registration model

**Decision**: External endpoints are registered as registry assets with `provenance=external`. They carry a `transport.endpoint` URL, a per-Tenant `credential_ref` (vault path; never persisted in plaintext), an optional `allowlist` of tool names (MCP) or task types (A2A), and the same lifecycle/trust pipeline as internal assets — including eval scores, which run synthetic probes against the external endpoint. The agent-card (A2A) or tool-manifest (MCP) is fetched once at registration, hashed, and re-verified before each promotion.

**Why**: External-by-default needs the same governance plane as internal, otherwise Tenants would bypass policy by classifying everything as "external." Treating external endpoints as first-class registry assets means policy, observability and audit pick them up without parallel plumbing.

**Alternative**: A separate "integrations" table outside the registry. Rejected — produces a parallel asset lifecycle that policy and observability would each need to learn about.

### 6. Pinning in the orchestration surfaces

**Decision**: The intent capture wizard adds a **Pin assets** step that queries the three gateway catalogs (registry filtered by `active_surface ≠ null`) and lets the user pin skills/MCPs/agents into the draft OpenSpec under a new `selected_assets: { skills: [...], mcps: [...], agents: [...] }` block. The wizard validates that every pin is `approved` and Workspace-visible. The visual editor reads the same `selected_assets` block when seeding its palette and persists explicit pins on the saved workflow AST.

**Why**: Today the orchestrator has to discover the asset set later in the flow; if a Tenant has 200 skills the LLM has to choose blindly. Pinning at design time turns the LLM choice into a bounded selection, which is cheaper, more deterministic and auditable.

**Alternative**: Tag-based filtering only. Rejected — tags are not as precise as explicit pins for governance, and the user explicitly asked for design-time selection.

### 7. Compatibility shim during the cut-over

**Decision**: The existing direct-dial paths (runner → MCP container, agent → agent) get a one-release compatibility shim: a thin client library that emits a deprecation telemetry event and forwards through `mcp-gateway`/`a2a-gateway`. Direct dial is then blocked at network policy in the following release. Feature flag: `gateway.enforced` per Tenant.

**Why**: Lets us migrate per Tenant, observe the deprecation events, and only flip the network policy once usage has converged to zero.

**Alternative**: Hard cutover at release. Rejected — too many internal callers; the migration risk is real.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Gateway becomes a hot single-point-of-failure for tool calls. | Stateless services, horizontal-scale-out, k8s HPA on CPU + p95 latency; circuit-breaker + bounded retries; warm-pool replicas matching expected MCP fan-out. |
| External MCPs/agents drift (endpoint changes shape after registration). | Re-verify the manifest/agent-card hash on each promotion and on a daily cron; emit `external_drift.v1` and mark the asset `deprecated` automatically if drift exceeds policy. |
| Adapter feature mismatch across artifact backends (e.g., retention semantics). | Capability flags returned by `Health(ctx)`; per-driver test suite asserting the minimum contract; documented matrix in the ADR. |
| Identity-propagation header forgery if internal network is breached. | Headers signed by `mcp-gateway`/`a2a-gateway` with a short-lived workload-identity-derived key; downstream MCPs verify the signature; rotation handled by SPIRE/IRSA-style identity. |
| LiteLLM-style budgets duplicated in three gateways → drift. | Budgets live in one Tenant-budget service that LiteLLM, `mcp-gateway` and `a2a-gateway` all consume via gRPC; reuse the existing budget contract from `model-gateway`. |
| Pinning forces orchestration into a tight contract; users may resent it. | Pinning is optional in the wizard (pinning empty = "let Alfred choose"); when set, it is enforced; when empty, current behavior is preserved. |
| Migration breaks active workloads. | Compatibility shim for one release, per-Tenant `gateway.enforced` flag, deprecation telemetry observed before network-policy flip. |

## Migration Plan

1. **Release N**: Ship `services/mcp-gateway`, `services/a2a-gateway`, `pkg/artifact-store-adapter` (Nexus driver only). Extend `services/registry` with `how_to_json`, `active_surface_json`, `external_provenance`. Internal callers continue direct-dial. Compatibility shim added in client libs. Feature flag `gateway.enforced=false` everywhere.
2. **Release N+1**: Add Artifactory, GitHub Packages, CodeArtifact drivers. Wizard and editor gain Pin assets / palette integration behind `pinning.enabled=true` flag (opt-in per Workspace). Existing registry assets bulk-migrated to populate `how_to_json` and `active_surface_json` from current metadata; assets missing either field are flagged in Portal but remain operable.
3. **Release N+2**: Flip `gateway.enforced=true` for pilot Tenants; observe deprecation telemetry for 2 weeks. Drift cron live. External-MCP/A2A registration available in Portal.
4. **Release N+3**: Default `gateway.enforced=true` globally; network policy blocks direct dial; compatibility shim removed; the catalog/how-to/gateway triad becomes a hard precondition for `approved`.

**Rollback**: any release before N+3 can flip `gateway.enforced=false` per Tenant; the compatibility shim keeps direct dial working. The artifact-store change is additive (registry stores adapter pointer alongside existing source paths until N+3), so reverting it does not invalidate published assets.

## Open Questions

1. **Budget service shape** — does the existing LiteLLM Tenant-budget endpoint expose enough for non-LLM (MCP / A2A) cost categorization, or do we introduce per-asset-family cost categories on it? Decision needed before release N.
2. **Drift policy for external assets** — daily cron is the default; do we also re-verify on every Nth invocation to catch faster-moving endpoints? Defaults differ for vendor-controlled vs. partner-controlled endpoints.
3. **Per-Tenant artifact-store override** — can a Tenant configure two backends (e.g., Nexus for skills + GitHub Packages for prompt templates) or one binding per Tenant? Leaning one-per-Tenant for simplicity; revisit if customer feedback demands per-asset-family bindings.
4. **A2A streaming through the public edge** — `services/skill-gateway` already proxies SSE; the new `a2a-gateway` adds `tasks/sendSubscribe`. Need to confirm the proxy chain holds backpressure across two hops (skill-gateway → a2a-gateway → downstream agent) under load; benchmark before release N+2.
5. **Pinning enforcement at orchestration time** — when the LLM tries to invoke an unpinned asset on a pinned-set workflow, do we hard-fail or warn? Default proposal: hard-fail with a clear policy reason; revisit after first pilots.
