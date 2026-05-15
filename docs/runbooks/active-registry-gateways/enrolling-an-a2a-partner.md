# Runbook: Enrolling an A2A Partner (inbound)

**Spec:** [active-registry-gateways](../../../openspec/changes/active-registry-gateways/) · **Owner:** Platform Engineering

This runbook covers the **inbound** path: enrolling an external partner agent so it can invoke specific internal Forge agents through `a2a-gateway`.

For the **outbound** path (internal agent → external A2A) use the [external-MCP runbook](enrolling-an-external-mcp.md) variant for agents (registry endpoint `/v1/registry/agents/external`); the inbound side is what this doc covers.

## Trust model

External partners authenticate to `a2a-gateway` with an HMAC-SHA256 signature over the request body, computed with a shared secret the platform issues at enrollment time. In production the cluster ingress terminates mTLS in front of the gateway and the HMAC layer is the second factor.

A partner record carries:

- `name` — used as the principal in the audit trail.
- `tenant_id` + optional `workspace_id` — the inbound call's tenancy scope.
- `allowed_assets` — list of internal-agent ids the partner may call. **Empty = deny-all.**
- `credential_b64` — base64-encoded shared secret (≥ 16 bytes).

## Operator steps

1. Pick a partner name. Use a stable, slug-shaped string (e.g. `partner-acme-coordinator`).
2. Generate a 32-byte random secret:
   ```bash
   openssl rand -base64 32
   ```
   Store the secret in Vault under a path scoped to the Tenant: `vault://t1/a2a-partners/partner-acme-coordinator`.
3. Enroll the partner via the gateway's admin endpoint:
   ```bash
   curl -sS -X POST "${A2A_GATEWAY_URL}/v1/gw/a2a/partners" \
     -H "authorization: Bearer ${ADMIN_TOKEN}" \
     -H "content-type: application/json" \
     -d '{
           "name": "partner-acme-coordinator",
           "workspace_id": "00000000-0000-0000-0000-000000000000",
           "allowed_assets": ["forge-sdlc-architect"],
           "credential_b64": "<base64-secret-from-step-2>"
         }'
   ```
   The response echoes the partner record with `credential_b64` stripped (the gateway does not echo secrets).
4. Share the secret with the partner out-of-band (signed envelope, PGP, internal CI vault — your call). The Portal **Platform → External integrations** page lists the enrolled partner under **Inbound partners (A2A)** but never displays the secret.

## What the partner sends

The partner POSTs the JSON-RPC envelope to `/v1/gw/a2a/<asset_id>` with two headers:

- `X-Forge-Inbound-Tenant: <tenant_id>` — disambiguates the partner's tenant.
- `X-Forge-Partner-Auth: <partner_name>;<base64(hmac-sha256(secret, body))>` — auth.

Example (bash + jq):
```bash
BODY='{"jsonrpc":"2.0","id":"1","method":"tasks/send","params":{"task":{"text":"design us a workflow"}}}'
SIG=$(printf "%s" "$BODY" | openssl dgst -sha256 -hmac "$SECRET" -binary | base64)
curl -sS -X POST "${A2A_GATEWAY_URL}/v1/gw/a2a/forge-sdlc-architect" \
  -H "content-type: application/json" \
  -H "X-Forge-Inbound-Tenant: ${TENANT_ID}" \
  -H "X-Forge-Partner-Auth: partner-acme-coordinator;${SIG}" \
  --data "${BODY}"
```

The gateway:
- Authenticates the partner (lookup + constant-time HMAC compare).
- Checks the requested asset is in `allowed_assets`.
- Calls policy-engine (`forge.gateway.a2a.allow` with `provenance=external_inbound`).
- Routes to the internal agent's `active_surface.upstream_endpoint`.
- Sets `X-Forge-Principal=partner-acme-coordinator`, `X-Forge-Principal-Kind=external_agent` on the inner call.
- Emits `com.forge.a2a.invocation.v1` with `source=inbound_external`.

## Failure modes

| Symptom | Cause | Action |
|---|---|---|
| `401 unknown_partner` | Partner not enrolled, or HMAC mismatch | Verify the secret + body the partner is signing — the HMAC is over the exact request bytes |
| `403 asset_not_allowlisted` | Partner targeted an agent not in `allowed_assets` | Either expand the allowlist or refuse the call |
| `400 missing_tenant` | Partner didn't send `X-Forge-Inbound-Tenant` | Add the header at the partner's side |
| `404 asset_not_found` | Target agent isn't registered in Forge | Verify the partner is using the correct asset id |
| `409 asset_not_approved` | Target agent is still `in_review` | Promote the internal agent through the lifecycle |

## Revoking a partner

There is no dedicated revoke endpoint yet; re-enroll with an empty `allowed_assets` (deny-all) or rotate the secret out-of-band. A revoke endpoint is tracked under [active-registry-gateways/tasks.md §10.x](../../../openspec/changes/active-registry-gateways/tasks.md).
