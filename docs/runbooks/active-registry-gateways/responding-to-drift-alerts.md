# Runbook: Responding to External-Asset Drift Alerts

**Spec:** [active-registry-gateways](../../../openspec/changes/active-registry-gateways/) · **Owner:** Platform Engineering · **Alert source:** [deploy/compose/prometheus/rules/active-registry-gateways.yml](../../../deploy/compose/prometheus/rules/active-registry-gateways.yml)

This runbook covers the operator response to the **ExternalDriftUnresolved24h** Prometheus alert and the in-process `com.forge.asset.external_drift.v1` / `external_drift_deprecated.v1` events emitted by the daily drift cron.

## What drift means

An external MCP or A2A asset's upstream manifest / agent-card has changed since the digest the platform captured at registration / last promotion. The cron compares the live digest with `external_mcp_endpoint.manifest_hash` (or `external_a2a_agent.agent_card_hash`) once per day. When they disagree:

1. `external_drift.v1` fires immediately.
2. If the asset is `approved`, the cron auto-moves it to `deprecated` and emits `external_drift_deprecated.v1`. Runners stop being able to invoke it on the next dispatch (because the gateway's registry lookup will fail the `approved` precondition).
3. If the asset is `in_review`, the cron only emits the drift event; the asset's lifecycle is unchanged because it was not yet trusted in production flows.

The alert fires when 24h pass with `external_drift.v1` count > `external_drift_deprecated.v1` count — i.e. drift was detected but the operator hasn't taken action.

## Triage

1. Open the alert payload. The labelset includes `tenant_id` and the asset id (via the event subject `asset/<id>@<version>`).
2. In Grafana, open the **Forge · Active Registry Gateways** dashboard; the **External-asset drift events** stat shows the 24h count.
3. In the Portal, navigate to **Platform → External integrations**, scope to the Tenant, and find the asset. The row carries a **drift** badge and the **last-verified** timestamp.

## Decide between three actions

### Action A — Roll the asset forward (the vendor changed something legitimately)

The new manifest is intentional. Steps:

1. Open the asset in **Assets** view. Note the recommended replacement field if the vendor left one.
2. Hit the registry promotion endpoint with `acknowledge_drift=true`:
   ```bash
   curl -sS -X POST "${REGISTRY_URL}/v1/assets/${ASSET_ID}/versions/${VERSION}/transition" \
     -H "authorization: Bearer ${TOKEN}" \
     -H "content-type: application/json" \
     -d '{"lifecycle_state": "approved", "trust_level": "T1", "acknowledge_drift": true,
          "eval_scores": {"quality": 0.9, "safety": 0.9, "cost": 0.9, "latency": 0.9}}'
   ```
3. The registry re-fetches the live manifest, accepts the new digest, and resets the cron's baseline.

### Action B — Block until investigation (the vendor changed something suspicious)

The new manifest is unexpected. Steps:

1. Leave the asset in `deprecated`. Runners are already locked out.
2. Open an internal investigation: pull the new manifest from `external_mcp_endpoint`, compare with the registered hash, snapshot for forensics.
3. Once cleared, run Action A. If the change is breaking, register a new asset id (e.g. `vendor-x-v2`) and let the old one fully retire.

### Action C — Retire the integration

The vendor or the integration is no longer wanted. Steps:

1. Transition the asset to `retired` via the standard transition endpoint.
2. Workflows pinned to this asset will fail with `asset_not_pinned`-style errors (the asset is no longer in the registry's approved set). Update the wizard pins to remove it.
3. Delete the credential in Vault.

## Don't do this

- **Don't** disable the drift cron to silence the alert. The cron is the only signal that an external surface has shifted.
- **Don't** ack drift without inspecting the new manifest. The whole point of the cron is to make a vendor's silent change observable.
- **Don't** edit `external_mcp_endpoint.manifest_hash` by hand. Use the promotion endpoint with `acknowledge_drift=true` so the change is audited.

## Verifying the alert is resolved

After Action A or C, the next cron pass refreshes `manifest_fetched_at`. The 24h alert window will close once the unresolved-drift count drops to 0. Operators can hand-trigger the cron via `kubectl exec` on the registry pod and running:

```bash
curl -sS -X POST localhost:8082/internal/drift-cron/run
```

(This endpoint is admin-only and is documented in [services/registry/internal/cron/drift/drift.go](../../../services/registry/internal/cron/drift/drift.go).)

## Related runbooks

- [Enrolling an external MCP](enrolling-an-external-mcp.md)
- [Enrolling an A2A partner](enrolling-an-a2a-partner.md)
- [Configuring an artifact-store binding](configuring-artifact-store-binding.md)
