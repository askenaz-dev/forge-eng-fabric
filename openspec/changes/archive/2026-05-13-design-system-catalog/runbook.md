# Runbook — design-system-catalog

## Migration steps

1. **M0 — Schema** (one-shot). Apply migrations in order:
   - `db/migrations/registry/0010_design_system_asset_type.sql`
   - `db/migrations/registry/0011_design_system_catalog_seed.sql`
   - `db/migrations/registry/0012_application_design_system.sql`

   Verify:
   ```bash
   psql $REGISTRY_DB -c "SELECT alias, asset_id FROM design_system_alias;"
   # ds-forge-default -> design_system:00000000-...-001:desing-system-1
   psql $REGISTRY_DB -c "SELECT id, version, name FROM asset WHERE type='design_system';"
   # 4 rows expected
   psql $REGISTRY_DB -c "SELECT DISTINCT design_system_ref FROM application LIMIT 5;"
   # every row carries ds-forge-default (or an explicit ref)
   ```

2. **M1 — Visual-parity verification** (zero-diff gate).
   ```bash
   node scripts/design-system-parity-check.mjs
   ```
   PASS is required before the feature flag flips for any tenant.

3. **M2 — Per-tenant pilot rollout** (gated on `forge.design_system_catalog.enabled`):
   - **Platform tenant first**: turn on the flag for `tenant=00000000-0000-0000-0000-000000000001`. Confirm `GET /v1/design-systems` returns the four built-ins; confirm the wizard's design-system step renders on the "new App" branch.
   - **Pilot tenants**: enable for two pilot tenants (operator's choice). Watch the App-Settings swap PR open/merge rates for 48 hours.
   - **Globally**: flip the flag default to `true` for every tenant.

## Rollback procedures

### Rollback a misbehaving Design System version

The simplest rollback is to **re-target the alias** rather than touching App rows. If `desing-system-3@2.0.0` ships a regression and many Apps swapped to it:

```bash
# Re-target the alias the affected Apps follow (default: ds-forge-default)
curl -X POST $REGISTRY_BASE_URL/v1/design-systems/aliases/ds-forge-default \
  -H "authorization: Bearer $TOKEN" \
  -H "content-type: application/json" \
  -d '{"target": "design_system:00000000-0000-0000-0000-000000000001:desing-system-1@1.0.0"}'
```

For Apps that pin to an explicit `desing-system-3@2.0.0` (not the alias), open a swap PR via the API:

```bash
curl -X POST $APPLICATION_BASE_URL/v1/apps/$APP_ID/design-system:swap \
  -H "authorization: Bearer $TOKEN" \
  -H "content-type: application/json" \
  -d '{"target_ref":"design_system:00000000-0000-0000-0000-000000000001:desing-system-1@1.0.0","reason":"Rollback of v2.0.0 regression"}'
```

### Roll back the catalog feature flag

Disable in the per-tenant flag store. The wizard's design-system step is gated server-side; existing Apps retain their `design_system_ref` value and the build-time merger continues to resolve it. The only user-visible change is that new Apps stop seeing the catalog step (the App is created with `ds-forge-default`).

### Roll back the schema (last-resort)

The asset table has an immutability trigger; deleting the four seeded built-in
template rows requires temporarily lifting the trigger:

```sql
ALTER TABLE asset DISABLE TRIGGER asset_no_delete;
DELETE FROM asset
  WHERE type='design_system'
    AND built_in_template=true
    AND tenant_id='00000000-0000-0000-0000-000000000001';
ALTER TABLE asset ENABLE TRIGGER asset_no_delete;
```

Then `goose down` to revert 0012 → 0011 → 0010. Application rows that
referenced the deleted alias will start failing the build-time merger; before
you do this, run a `UPDATE application SET design_system_ref = NULL` and
plan to re-seed from a known-good backup.

## On-call signals

- `app.design_system.swap_requested.v1` event spikes → triage on the open
  swap PR's CI; if CI is failing across multiple Apps the merger likely has a
  bug.
- `design_system_digest_mismatch` errors in portal build logs → the asset's
  CDN was modified out-of-band; cross-check with the asset's `tokens_sha256`
  in the Registry and re-publish a version with the correct digest.
- `untrusted_url_in_tokens` rejections on tenant publishes → the validator
  rejected an off-tenant URL reference. Confirm with the publishing tenant
  and add the domain to their approved list if legitimate.

## Owner

Platform-design (`forge-platform-design`) owns the four built-in templates
and the alias. The App-team owns the swap/override API surface. The
Portal-team owns the build-time merger and the wizard / Settings UI.
