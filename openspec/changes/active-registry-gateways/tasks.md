## 1. Data model and migrations (registry)

- [ ] 1.1 Add `how_to_json`, `active_surface_json`, `external_provenance` columns to `asset` (nullable in N, NOT NULL precondition enforced at lifecycle `approved` from release N+1).
- [ ] 1.2 Create `external_mcp_endpoint` table (`asset_id`, `tenant_id`, `endpoint_url`, `credential_ref`, `allowlist[]`, `manifest_hash`, `manifest_fetched_at`, `created_by`, `created_at`).
- [ ] 1.3 Create `external_a2a_agent` table (`asset_id`, `tenant_id`, `endpoint_url`, `credential_ref`, `task_allowlist[]`, `agent_card_hash`, `agent_card_fetched_at`, `created_by`, `created_at`).
- [ ] 1.4 Create `artifact_store_binding` table (`tenant_id`, `backend` ∈ {`nexus`, `artifactory`, `github-packages-private`, `codeartifact`}, `config_json`, `created_at`).
- [ ] 1.5 Add indices on the hot paths: `asset(tenant_id, type, lifecycle_state, active_surface_json IS NULL)`, `external_mcp_endpoint(tenant_id, asset_id)`, `external_a2a_agent(tenant_id, asset_id)`.
- [ ] 1.6 Author OpenFGA tuples for `gateway_caller` (workload identity) and `external_partner` (inbound A2A); update the OpenFGA bootstrap fixture.

## 2. Registry changes (`services/registry`)

- [ ] 2.1 Extend `Asset` Go struct and JSON schema with `how_to` and `active_surface` blocks; surface them on every read response.
- [ ] 2.2 Add JSON schema validators for `how_to` (install per client, usage per language, env) and `active_surface` (`family ∈ {mcp,a2a,skill}` + endpoint or artifact pointer + digest/signature).
- [ ] 2.3 Add lifecycle precondition: promotion to `approved` rejects when `how_to_json`, `active_surface_json` or eval scores are missing.
- [ ] 2.4 Implement external-MCP registration endpoint `POST /v1/registry/mcps/external` (fetch manifest, persist hash, emit `com.forge.asset.external_registered.v1`).
- [ ] 2.5 Implement external-A2A registration endpoint `POST /v1/registry/agents/external` (fetch agent card, persist hash, emit equivalent event).
- [ ] 2.6 Implement re-verification on promotion: fetch live manifest/agent-card, compare hashes, refuse promotion on drift unless `acknowledge_drift=true` is passed.
- [ ] 2.7 Implement daily drift cron (`services/registry/internal/cron/drift`) over all `provenance=external` assets; emit drift events; auto-deprecate on policy breach.
- [ ] 2.8 Update OpenAPI contract in `contracts/openapi/registry.yaml` for the new fields, endpoints and events; regenerate clients.
- [ ] 2.9 Cover with integration tests under `services/registry/tests/` (how_to/active_surface validation, external registration, drift detection, lifecycle precondition).

## 3. Artifact-store adapter (`pkg/artifact-store-adapter`)

- [ ] 3.1 Define the Go interface (`Put`, `Get`, `Stat`, `Delete`, `Health`) and capability-flag schema (`supports_retention`, `supports_signed_urls`, `supports_lifecycle_rules`).
- [ ] 3.2 Implement the Nexus driver (REST API, basic auth or token, per-Tenant repository convention).
- [ ] 3.3 Implement the JFrog Artifactory driver.
- [ ] 3.4 Implement the GitHub Packages (private) driver.
- [ ] 3.5 Implement the AWS CodeArtifact driver.
- [ ] 3.6 Reject `npm-public` and any backend whose Health reports `is_public=true` at binding time.
- [ ] 3.7 Per-driver test suite asserting the minimum contract (immutability of `(asset_id, version)`, digest verification on Get, cross-Tenant denial, audit emission).
- [ ] 3.8 Wire the adapter into the existing CI publish pipeline (cosign sign + in-toto attest + adapter Put + registry lifecycle-publish hook).
- [ ] 3.9 Document the per-backend matrix in `docs/governance/adrs/0002-artifact-store-adapter.md` (ADR), capability flags and operational expectations.

## 4. MCP Gateway (`services/mcp-gateway`)

- [ ] 4.1 Scaffold the service from the `services/registry` template (chi, JWT middleware, otelhttp, Kafka producer, audit emitter).
- [ ] 4.2 Implement workload-identity-derived signing key (SPIRE/IRSA-equivalent), rotation, and `X-Forge-Identity-Signature` over `(principal, tenant, workspace, correlation_id, ts)`.
- [ ] 4.3 Implement `POST /v1/gw/mcp/{asset_id}` (HTTP + SSE) — route resolution from the registry's `active_surface`, identity header injection, OPA pre-check, Tenant-budget probe, audit + invocation event per tool call.
- [ ] 4.4 Implement credential brokering for external MCPs: fetch from vault at call time, redact from logs/traces/audit, scrub from outbound errors.
- [ ] 4.5 Implement tool-allowlist enforcement for external MCPs (`provenance=external` → check `allowlist`).
- [ ] 4.6 Implement SSE relay with end-to-end backpressure and bounded buffer per connection.
- [ ] 4.7 Implement `GET /v1/gw/mcp/catalog` returning approved MCPs (internal + external) with `provenance`, `active_surface`, `how_to`.
- [ ] 4.8 Implement Redis-backed rate limits per Tenant/Workspace; expose Prometheus metrics for `requests_total`, `latency`, `errors`, `budget_blocks`.
- [ ] 4.9 Implement compatibility-shim client lib (`pkg/mcp-shim`) — emits `com.forge.runtime.gateway_bypass_deprecated.v1` and forwards through the gateway.
- [ ] 4.10 Integration tests under `services/mcp-gateway/tests/` (internal call, external call with credential redaction, policy deny, budget exhaustion, drift propagation, SSE backpressure, shim deprecation event).

## 5. A2A Gateway (`services/a2a-gateway`)

- [ ] 5.1 Scaffold from the `services/registry` template.
- [ ] 5.2 Implement A2A JSON-RPC: `tasks/send`, `tasks/get`, `tasks/cancel`, `tasks/sendSubscribe` over HTTP + SSE.
- [ ] 5.3 Implement outbound flow (internal → external A2A): credential brokering, identity header injection, OPA pre-check, audit + invocation event.
- [ ] 5.4 Implement inbound flow (external partner → internal Forge agent): partner authentication (mTLS or signed JWT), `principal_kind=external_agent`, route to agent runtime, audit + invocation event.
- [ ] 5.5 Implement `GET /v1/gw/a2a/catalog` returning approved A2A agents (internal + external) with metadata.
- [ ] 5.6 Implement enrollment endpoint for external partners (`POST /v1/gw/a2a/partners`) per Tenant; reject unenrolled inbound traffic.
- [ ] 5.7 Implement rate limits per Tenant/Workspace; per-task metrics.
- [ ] 5.8 Implement compatibility-shim client lib (`pkg/a2a-shim`) for the cut-over.
- [ ] 5.9 Integration tests (outbound to external partner with credential redaction, inbound from enrolled partner, inbound from unenrolled partner rejected, streaming via `sendSubscribe` with backpressure, policy deny, drift propagation).

## 6. Runtime integration (`services/workflow-runtime`, Alfred, runners)

- [ ] 6.1 Replace direct MCP dial with `pkg/mcp-shim` everywhere in the runner.
- [ ] 6.2 Replace direct agent-to-agent calls with `pkg/a2a-shim` in the workflow runtime.
- [ ] 6.3 Implement `selected_assets` enforcement in the runtime: reject invocations outside the pinned set with `403 asset_not_pinned`, emit `guardrail.trip.v1`.
- [ ] 6.4 Add per-Tenant `gateway.enforced` flag plumbing into the runtime config; flip network policy templates accordingly.
- [ ] 6.5 Author the NetworkPolicy manifests under `k8s/` blocking direct dial when `gateway.enforced=true`; verify in a kind cluster.
- [ ] 6.6 Integration tests against compose stack (`gateway.enforced=false` shim path, `gateway.enforced=true` block path).

## 7. Portal UI (`portal/`)

- [ ] 7.1 Extend the asset detail page with **How-to** and **Gateway** tabs reading from `how_to_json` and `active_surface_json`.
- [ ] 7.2 Build the **External integrations** view under Platform: list and register external MCPs and A2A partners, show drift status, manage credentials by ref.
- [ ] 7.3 Build the **Pin assets** step in the intent capture wizard (three filterable lists, one per asset family).
- [ ] 7.4 Update the visual editor palette to source from the gateway catalogs, mark pinned/outside-of-pin, prompt when adding outside-of-pin assets to a pinned workflow.
- [ ] 7.5 Add saved-AST persistence for `node.active_surface.endpoint` (server + client schema).
- [ ] 7.6 Surface drift state and `gateway_published`/`gateway_unpublished` events in the asset list view (badges + filters).
- [ ] 7.7 E2E tests under `portal/e2e/` covering register-external-mcp, pin-skill-in-wizard, palette-pinned-ordering, save-with-active-surface.

## 8. Policy and observability

- [ ] 8.1 Add `gateway.mcp.*` and `gateway.a2a.*` OPA rule bundles; default rules for allow-internal-only, deny-external-without-allowlist, deny-cross-Tenant.
- [ ] 8.2 Extend the Tenant-budget service contract to categorize cost by `family ∈ {llm, mcp, a2a}`; `model-gateway`, `mcp-gateway`, `a2a-gateway` all probe it.
- [ ] 8.3 Wire `per-asset-observability` to ingest `com.forge.mcp.invocation.v1` and `com.forge.a2a.invocation.v1` with `source ∈ {internal, external_proxy, inbound_external}`.
- [ ] 8.4 Add Grafana dashboards: per-gateway latency p50/p95, error rate, budget blocks, external-drift counts.
- [ ] 8.5 Add alerts for `gateway p95 > SLO`, `external_drift > 0 unresolved 24h`, `compatibility shim usage > 0 with gateway.enforced=true` (should be impossible — alert if seen).

## 9. Documentation

- [ ] 9.1 Author `docs/platform/active-registry-gateways.md` covering the three-pillar pattern (catalog + how-to + gateway), with deep links to MCP, A2A, skill artifact-store specs.
- [ ] 9.2 Author `docs/governance/adrs/0002-artifact-store-adapter.md` recording the adapter approach, capability matrix, vendor support and rationale for rejecting public NPM.
- [ ] 9.3 Update `docs/platform-enablement.md` to reference the three new capabilities.
- [ ] 9.4 Update `docs/runbooks/` with operator runbooks: enrolling an external MCP, enrolling an A2A partner, configuring an artifact-store binding, responding to drift alerts.

## 10. Migration and rollout

- [ ] 10.1 Release N: ship gateways + adapter (Nexus only) + registry schema; `gateway.enforced=false` everywhere; bulk-populate `how_to_json` / `active_surface_json` from current asset metadata where derivable; flag the rest in Portal.
- [ ] 10.2 Release N+1: ship Artifactory + GitHub Packages + CodeArtifact drivers; enable `pinning.enabled=true` per Workspace opt-in; wizard + editor surfaces live behind the flag.
- [ ] 10.3 Release N+2: flip `gateway.enforced=true` for pilot Tenants; observe deprecation telemetry for 2 weeks; drift cron live; external-MCP/A2A registration available in Portal.
- [ ] 10.4 Release N+3: default `gateway.enforced=true` globally; network policy blocks direct dial; remove compatibility shims; `how_to` + `active_surface` become hard preconditions for `approved`.
- [ ] 10.5 Rollback plan documented per release with the exact flag flips and database compatibility window.

## 11. Open-question resolution before each release gate

- [ ] 11.1 Before Release N: confirm the budget service contract supports per-family categorization (LLM / MCP / A2A) without breaking `model-gateway` callers.
- [ ] 11.2 Before Release N+1: ratify the per-Tenant single-binding decision (or, if customer feedback demands it, design per-asset-family bindings).
- [ ] 11.3 Before Release N+2: benchmark two-hop SSE backpressure (skill-gateway → a2a-gateway → external) under load; capture the SLO and reject the release if p95 added latency > 50 ms.
- [ ] 11.4 Before Release N+3: ratify hard-fail vs. warn for pinned-asset enforcement, based on pilot feedback; default proposed is hard-fail.
