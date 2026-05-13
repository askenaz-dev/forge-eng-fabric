# Skill gateway — public-ingress runbook

Operational playbook for the developer skill gateway. Owns: Platform Engineering. Reviewed by: Security.

## Inventory

| Component | Where |
|---|---|
| Helm chart | `deploy/kubernetes/skill-gateway/` |
| Image | `ghcr.io/forge-eng-fabric/skill-gateway:<appVersion>` |
| Public hostname pattern | `https://<tenant>.forge.dev` |
| Backing secret | `skill-gateway-secrets` in the gateway namespace |
| Postgres | shared `forge_registry` DB, dedicated role `gateway` (RO on `asset`, RW on `gateway_*`) |
| Redis | dedicated cluster for rate-limit + revocation pub/sub |
| Object store | `s3://forge-packages/<asset_id>/<version>.tar.zst` |
| Kafka topic | `forge.events` (CloudEvents) |

## Deploy

```bash
helm upgrade --install skill-gateway deploy/kubernetes/skill-gateway \
  -n skill-gateway --create-namespace \
  -f deploy/kubernetes/skill-gateway/values.yaml \
  --set image.tag="$VERSION"
```

Secrets are managed externally (sealed-secrets / external-secrets-operator) and exposed as `skill-gateway-secrets`. The expected keys are: `POSTGRES_URL`, `REDIS_URL`, `REGISTRY_SYSTEM_TOKEN`.

## Certs

Per-tenant subdomain (`<tenant>.forge.dev`) requires automated cert issuance. We use cert-manager with a DNS-01 issuer pointing at the platform DNS zone. Renewal lead time is 30 days; alerts fire 14 days before expiry.

## WAF rules (front of the ingress)

Minimum baseline:

- Block requests where `Content-Length > 8 MiB` (ingress also enforces this; WAF is belt-and-suspenders).
- Block requests with `User-Agent` matching the known scraper / scanner deny list.
- Rate-limit `/v1/gateway/auth/*` to 30 req/min per source IP — protects the device-code endpoint from brute force.
- Inspect `Authorization` header presence on non-health routes and short-circuit when missing.

## Incident playbooks

### IR-1: PAT leak (developer reports a leak)

1. Identify the PAT id (`gateway_token.id`) from the developer's account in the portal.
2. `DELETE /v1/gateway/tokens/{id}` from any admin terminal — propagates to all gateway replicas within 5 s via Redis pub/sub.
3. Confirm via `grep token_revoked` in the gateway logs for the affected token id.
4. Notify the developer to regenerate via `forge login`.
5. If the leak is suspected to predate revocation, run the audit query for `gateway_invocation` events keyed by the token id, scope the blast radius, and escalate per the existing security-incident process.

### IR-2: Abuse spike on `/v1/gateway/auth/device`

1. Confirm the spike in Grafana (gateway QPS by route).
2. Tighten the WAF rule on `/v1/gateway/auth/*` (cut to 5 req/min) — change is rule-only, no redeploy.
3. Inspect the source IPs; if concentrated, drop them at the WAF.
4. Escalate to Security if more than one Tenant is affected.

### IR-3: Budget exhaustion on a Tenant

1. Confirm `402 budget_exhausted` is being returned by querying `gateway.invocation.v1` events filtered by tenant + outcome.
2. The Tenant owner sees the budget banner in the portal. Decide: raise budget vs. let the freeze stand.
3. If raising: update the Tenant's LiteLLM budget via the existing Tenant admin flow. The gateway picks up the new limit on its next budget probe (≤30 s).

### IR-4: Drift alert with `drift_source=gateway`

1. Verify in the portal Asset detail page — drift annotation must show `source=gateway`.
2. Pull the last 100 gateway invocations of the affected `(asset_id, version)` from the audit log.
3. If the regression is consistent, file a regression ticket against the asset owner and consider rolling the asset back to a known-good version via `lifecycle-hooks/gateway-publish` with the prior digest.

## Rollback

- The gateway is stateless; rolling back the image is `helm rollback skill-gateway <revision>`.
- Disabling the gateway entirely: scale the deployment to 0 — internal flows (Alfred, workflow-runtime) are not affected.
- Disabling external visibility per asset: flip `distribution.gateway_published=false` via the registry's transition endpoint or by transitioning the asset to `deprecated`.
