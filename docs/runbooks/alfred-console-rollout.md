# Alfred Console Redesign — Tenant Rollout Runbook

Feature: Alfred Console v2 (Friendly / Advanced views, spec deduplication, `/forge` rename).
Flag: `forge.alfred_console_v2.enabled` (per-tenant) + `ALFRED_CONSOLE_V2_ENABLED=true` (service).

## Pre-requisites

- `ALFRED_CONSOLE_V2_ENABLED=true` set on the Alfred service and portal deployed.
- `SPEC_MATCH_THRESHOLD_DEFAULT` and `SPEC_MATCH_THRESHOLD_FLOOR` configured (defaults: 0.80 / 0.65).
- Dedup index populated for the target tenant's spec corpus (run `make dedup-index TENANT=<id>`).
- Grafana dashboards imported from `docs/dashboards/alfred-console-v2.json`.

## Rollout sequence

### Stage 1 — Platform team (internal)

1. Enable the flag for the platform tenant:

   ```sh
   curl -X PUT http://<control-plane>/v1/tenants/<platform_tenant_id>/flags/forge.alfred_console_v2.enabled \
     -H 'Authorization: Bearer <token>' \
     -d '{"value": true}'
   ```

2. Validate in the Portal:
   - Sign in as a `workspace.member` → should land on Friendly view.
   - Sign in as a `workspace.developer` → should land on Advanced view.
   - Toggle via the account menu; confirm preference persists across refresh.
   - Submit an intent that matches an existing spec; confirm match dialog fires.
   - Confirm `/forge new` appears in the palette; `/openspec new` shows deprecation toast.

3. Monitor for 48 hours:
   - Grafana: "Alfred Console v2" board — check Friendly/Advanced ratio, match-found rate, error rate.
   - Check `alfred.command.deprecated_alias.v1` event volume (expected near-zero for internal team).

### Stage 2 — Pilot tenants (2 tenants)

**Threshold calibration before enabling globally**: run the calibration script against the pilot tenant corpora:

```sh
uv run python scripts/dedup_threshold_calibration.py \
  --tenant <pilot_tenant_1> \
  --tenant <pilot_tenant_2> \
  --output docs/governance/evidence/alfred-console-v2/threshold-calibration.json
```

The script reports false-positive and false-negative rates at each threshold step (0.60–0.95). Adjust `SPEC_MATCH_THRESHOLD_DEFAULT` per-tenant if the corpus produces unacceptable false-positive rates above 0.80.

Enable the flag for each pilot tenant:

```sh
for TENANT_ID in <pilot_1> <pilot_2>; do
  curl -X PUT http://<control-plane>/v1/tenants/${TENANT_ID}/flags/forge.alfred_console_v2.enabled \
    -H 'Authorization: Bearer <token>' \
    -d '{"value": true}'
done
```

Monitor for 1 week. Exit criteria:
- Error rate < 0.5% on `/api/alfred/intent/start` and `/api/alfred/console`.
- Match-found rate in expected range (5–30% of intents, depending on corpus maturity).
- No `spec_not_ready_for_architect` 409s from unexpected paths.
- Friendly first-paint p95 < 1s (check in Grafana RUM panel).

### Stage 3 — Global rollout

After pilot exit criteria pass, enable the flag for all remaining tenants via the control-plane bulk API:

```sh
curl -X POST http://<control-plane>/v1/flags/forge.alfred_console_v2.enabled/bulk-enable \
  -H 'Authorization: Bearer <token>' \
  -d '{"exclude_tenant_ids": ["<tenants_with_exception>"]}'
```

Tenants with `force_keep_openspec_alias: true` must be in the exclude list. See `docs/runbooks/alfred-openspec-alias-exception.md`.

## Rollback

To disable for a specific tenant:

```sh
curl -X PUT http://<control-plane>/v1/tenants/<tenant_id>/flags/forge.alfred_console_v2.enabled \
  -H 'Authorization: Bearer <token>' \
  -d '{"value": false}'
```

The portal falls back to the original `/alfred` console (pre-v2 path). User preferences already written are preserved and will resume on re-enable.

To roll back globally, set `ALFRED_CONSOLE_V2_ENABLED=false` on the Alfred service and redeploy.

## Dashboards and SLOs

| Signal | SLO | Grafana panel |
|---|---|---|
| Dedup retrieval p95 | < 100 ms | "Intent match latency" |
| Friendly view first-paint p95 | < 1 s | "Portal RUM — Friendly" |
| `/v1/intent/match` error rate | < 1% | "Alfred API errors" |
| Match dialog dismiss rate | track only | "Match dismissed ratio" |
| `/openspec` alias invocations | decreasing | "Deprecated alias volume" |
