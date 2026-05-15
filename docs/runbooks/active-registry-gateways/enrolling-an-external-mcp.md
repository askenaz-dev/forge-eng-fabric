# Runbook: Enrolling an External MCP

**Spec:** [active-registry-gateways](../../../openspec/changes/active-registry-gateways/) · **Owner:** Platform Engineering

This runbook walks an operator through registering a vendor MCP server for a single Tenant so the platform can broker calls to it.

## Pre-flight

1. Confirm the vendor MCP endpoint is reachable from the platform's egress. Note the URL: `https://vendor-x.example.com/mcp`.
2. Place the API token in Vault. The runbook assumes the path `vault://t1/vendor-x/api-key`; substitute for your Tenant id.
3. Decide the **tool allowlist**. An empty allowlist is treated as deny-all by the gateway — populate it explicitly.
4. Confirm you can hit the registry as a Workspace `editor`. The runbook below shows the Portal path; the API path is documented at the end.

## Portal path

1. Sign in to the Portal as a Workspace owner.
2. Navigate to **Platform → External integrations**.
3. Choose the Workspace from the scope selector and click **Load**.
4. In the **External MCPs** panel, fill in:
   - **Name** — `vendor-x` (used as the registry asset id suffix)
   - **Version** — `0.1.0`
   - **Endpoint URL** — `https://vendor-x.example.com/mcp`
   - **Credential ref** — `vault://t1/vendor-x/api-key`
   - **Tool allowlist** — `read_doc, list_docs`
5. Click **Register**.

The Portal POSTs to `services/registry`'s `/v1/registry/mcps/external` endpoint, which:

- Fetches the live tool manifest and stores its sha256 digest on `external_mcp_endpoint.manifest_hash`.
- Creates the asset row with `provenance=external, lifecycle_state=proposed`.
- Emits `com.forge.asset.external_registered.v1` onto `forge.events`.

A confirmation banner appears: **Registered mcp:vendor-x**.

## Promotion to `approved`

The asset stays in `proposed` until the standard registry lifecycle promotes it. Before promotion, ensure:

- `how_to_json` is populated. For an external MCP this is typically `{"install": {"cli": "use forge-cli mcp-attach vendor-x"}, "usage": {"typescript": "..."}}`.
- The registry's eval-score threshold for the chosen trust level is met (run the eval-harness with synthetic probes against the vendor endpoint).
- The `acknowledge_drift=false` re-verification still matches the registered hash. If the vendor has rotated their manifest, pass `acknowledge_drift=true` on the promotion request (this is logged).

Promotion via Portal: open the asset in **Assets**, click **Promote to in_review** then **Promote to approved**.

## Invoking the MCP at runtime

After approval, runners reach the MCP through:

```
POST /v1/gw/mcp/vendor-x?tool=read_doc HTTP/1.1
Host: mcp-gateway.cluster.local
Authorization: Bearer <workload-identity-jwt>
Content-Type: application/json

{"params": {"doc_id": "..."}}
```

The mcp-gateway resolves the asset, brokers the vault credential, signs identity headers and proxies the call. Observability events surface in the asset rollup with `source=external_proxy`.

## Failure modes

| Symptom | Cause | Action |
|---|---|---|
| Portal returns `400 invalid_credential_ref` | Pasted the raw API token instead of a vault path | Move the token into Vault and re-submit with `vault://...` |
| Portal returns `502 manifest_fetch_failed` | Vendor endpoint unreachable from egress | Test with `curl` from inside the cluster; check egress policy |
| Gateway returns `403 tool_not_allowlisted` at runtime | Caller tried a tool the operator did not enumerate | Edit the allowlist on the asset or refuse the call |
| Daily drift cron emits `external_drift_deprecated.v1` | Vendor rotated their manifest | Open the asset, review the new manifest, re-promote with `acknowledge_drift=true` |
| Gateway returns `403 cross_tenant_denied` | Caller's Tenant id mismatches the asset's Tenant id | The caller is impersonating; investigate via correlation_id |

## API path (no Portal)

```bash
curl -sS -X POST "${REGISTRY_URL}/v1/registry/mcps/external" \
  -H "authorization: Bearer ${TOKEN}" \
  -H "content-type: application/json" \
  -d '{
        "workspace_id": "00000000-0000-0000-0000-000000000000",
        "name": "vendor-x",
        "version": "0.1.0",
        "owner_team": "platform-integrations",
        "endpoint_url": "https://vendor-x.example.com/mcp",
        "credential_ref": "vault://t1/vendor-x/api-key",
        "allowlist": ["read_doc", "list_docs"]
      }'
```
