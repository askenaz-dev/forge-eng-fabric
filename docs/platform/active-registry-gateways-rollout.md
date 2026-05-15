# Active Registry Gateways — Rollout Plan

**Spec:** [active-registry-gateways](../../openspec/changes/active-registry-gateways/) · **Owner:** Platform Engineering

Four-release staged rollout. Each release is gated on the pre-release ratifications under [active-registry-gateways/tasks.md §11](../../openspec/changes/active-registry-gateways/tasks.md). Operators flip a single Tenant cohort at a time.

## Summary

| Release | Code changes | Flag state | Risk | Rollback |
|---|---|---|---|---|
| **N** | Schema + mcp-gateway + a2a-gateway + Nexus driver + shims | `gateway.enforced=false` everywhere | Low — additive | Flip flag off, no DDL revert needed |
| **N+1** | Artifactory + GitHub Packages + CodeArtifact drivers + Pin wizard + Editor palette | `pinning.enabled=true` per Workspace opt-in | Low — opt-in | Flip pin flag off; pinned workflows fall back to open behavior |
| **N+2** | Flip `gateway.enforced=true` for pilot Tenants | Pilots only | Medium — first enforcement | Per-Tenant flip back to `false`; shims still in code |
| **N+3** | Default `gateway.enforced=true` globally + NetworkPolicy enforcement + remove shims | Global | High — hard gate | See §Rollback below |

## Release N — Foundation

### What ships

- DB schema (`db/migrations/registry/0007_active_registry_gateways.sql`): `how_to_json`, `active_surface_json`, `external_provenance` on `asset`; `external_mcp_endpoint`, `external_a2a_agent`, `artifact_store_binding`.
- OpenFGA types `gateway_caller` + `external_partner`.
- `services/mcp-gateway` + `services/a2a-gateway` services.
- `pkg/artifact-store-adapter` with the **Nexus driver only**.
- `pkg/mcp-shim` + `pkg/a2a-shim` for cut-over compatibility.
- Registry external-MCP / external-A2A registration endpoints.
- Daily drift cron.
- All gateways run in cluster but no Tenant is enforcing yet (`gateway.enforced=false`).

### Pre-flight

- **§11.1**: Confirm the [tenant-budget contract](../../contracts/openapi/tenant-budget.yaml) supports per-family categorization without breaking `model-gateway` callers. See [ratifications/11.1-budget-contract.md](../governance/ratifications/active-registry-gateways/11.1-budget-contract.md).

### Bulk-migrate existing assets

Existing rows have `how_to_json IS NULL, active_surface_json IS NULL`. The Portal asset list view surfaces a **Missing active_surface** filter so owners can backfill before §N+1's precondition starts enforcing.

The migration script (run as a one-off during the release window):
```sql
-- Set provenance from metadata.provenance if it was inlined there
UPDATE asset SET external_provenance = COALESCE(metadata->>'provenance', 'internal');

-- For approved MCPs that already have a transport URL in metadata, seed a
-- minimal active_surface so they continue to be discoverable.
UPDATE asset
   SET active_surface_json = jsonb_build_object(
         'family', 'mcp',
         'endpoint', '/v1/gw/mcp/' || id
       )
 WHERE type = 'mcp' AND lifecycle_state = 'approved' AND active_surface_json IS NULL;
```

### Validation

- Promote one synthetic external MCP through the lifecycle. Confirm:
  - Manifest hash captured on registration.
  - `com.forge.asset.external_registered.v1` emitted.
  - Drift cron logs `scanned=1 drifted=0 deprecated=0`.

## Release N+1 — Adapter drivers + design-time pinning

### What ships

- `pkg/artifact-store-adapter` drivers for **Artifactory**, **GitHub Packages (private)** and **AWS CodeArtifact**.
- Portal **Platform → External integrations** view (already shipped in N as part of §7.2 but enabled now).
- Wizard **Pin assets** step gated on `pinning.enabled=true` per Workspace.
- Visual-editor palette sourced from gateway catalogs.
- The `approved` lifecycle precondition starts enforcing `how_to_json` + `active_surface_json` non-null. Assets failing the precondition are blocked from new approvals; existing approved assets are grandfathered.

### Pre-flight

- **§11.2**: Ratify the per-Tenant single-binding decision. See [ratifications/11.2-per-tenant-binding.md](../governance/ratifications/active-registry-gateways/11.2-per-tenant-binding.md).
- Confirm all four adapter drivers pass their contract suite against staging endpoints (not just httptest fakes).

### Validation

- Toggle `pinning.enabled=true` for one Workspace. Run the intent wizard end-to-end; commit produces an OpenSpec carrying `selected_assets`. Run the workflow; runtime emits `guardrail.trip.v1` when an off-pin asset is invoked.

## Release N+2 — Pilot enforcement

### What ships

- Per-Tenant cohort flip: `gateway.enforced=true` for pilot Tenants.
- NetworkPolicy manifests applied per pilot namespace ([k8s/networkpolicy-gateway-enforced.yaml](../../k8s/networkpolicy-gateway-enforced.yaml)).
- Daily drift cron is in production for all external assets across all Tenants (was Release N for pilot Tenants only).

### Pre-flight

- **§11.3**: Benchmark two-hop SSE backpressure (`skill-gateway → a2a-gateway → external`) under load. Capture the SLO. Reject the release if p95 added latency > 50 ms. See [ratifications/11.3-sse-backpressure-benchmark.md](../governance/ratifications/active-registry-gateways/11.3-sse-backpressure-benchmark.md).

### Validation

- Observe deprecation telemetry for 2 weeks on the pilot cohort. The compat-shim usage metric (`forge_runtime_gateway_bypass_deprecated_total{gateway_enforced="true"}`) must trend to 0. The **CompatibilityShimUsedUnderEnforcement** alert fires immediately if any non-zero reading appears.

### Per-Tenant flip

```bash
# 1. Annotate the Tenant's namespace
kubectl label ns "forge-tenant-${TENANT_ID}" forge.platform/gateway-enforced=true

# 2. Apply the NetworkPolicy bundle
kubectl apply -n "forge-tenant-${TENANT_ID}" -f k8s/networkpolicy-gateway-enforced.yaml

# 3. Set the runtime flag (deployment env)
kubectl set env -n "forge-tenant-${TENANT_ID}" deploy/workflow-runtime GATEWAY_ENFORCED=true
```

## Release N+3 — Global enforcement + cleanup

### What ships

- `gateway.enforced=true` becomes the default for all Tenants.
- NetworkPolicy applied cluster-wide.
- `pkg/mcp-shim` and `pkg/a2a-shim` removed from the runtime go.mod; runtime calls the gateway clients directly.
- The `approved` lifecycle precondition becomes a hard precondition on all asset types (not just new ones). Grandfathered assets must be backfilled or they cannot be re-promoted.

### Pre-flight

- **§11.4**: Ratify hard-fail vs warn for pinned-asset enforcement based on pilot feedback. See [ratifications/11.4-pinned-asset-enforcement.md](../governance/ratifications/active-registry-gateways/11.4-pinned-asset-enforcement.md).

## Rollback playbook

Each release has a documented back-out. Roll forward (a follow-up release that addresses the issue) is always preferred over roll-back; the back-out is the emergency posture only.

| From | To | Action |
|---|---|---|
| N+3 | N+2 | Re-introduce the compatibility shims (revert the runtime deletion), flip `GATEWAY_ENFORCED=false` globally, remove the NetworkPolicy. Lifecycle precondition stays on (assets registered after N+1 already have the blocks). |
| N+2 | N+1 | Per-pilot Tenant: remove the NetworkPolicy, flip `GATEWAY_ENFORCED=false`, keep gateways running. Drift cron stays on. |
| N+1 | N | Flip `pinning.enabled=false` per Workspace. Existing `selected_assets` blocks remain on the workflow rows but the runtime stops enforcing them. Adapter drivers (Artifactory / GH Packages / CodeArtifact) are no-ops if no Tenant has a binding configured for them. |
| N | (pre-N) | Roll back the registry deployment to a tag before 0007 was applied. The migration's `-- +goose Down` block drops the new columns and tables; assets remain operable because the gateways are not yet on the hot path. |

### Database compatibility window

The 0007 migration is **additive** through Release N+2 — old code that doesn't know about the new columns still functions. Release N+3 introduces hard preconditions; rolling back to N+2 needs no DB change, but rolling back to before Release N+1's bulk backfill leaves the columns populated (which is harmless for older code).

## Linked artifacts

- [Tasks](../../openspec/changes/active-registry-gateways/tasks.md)
- [Proposal](../../openspec/changes/active-registry-gateways/proposal.md)
- [Design](../../openspec/changes/active-registry-gateways/design.md)
- [ADR 0002 — artifact-store-adapter](../governance/adrs/0002-artifact-store-adapter.md)
- [Operator runbooks](../runbooks/active-registry-gateways/)
- [Pre-release ratifications](../governance/ratifications/active-registry-gateways/)
